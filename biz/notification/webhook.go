package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// ChannelWebhook is the conventional notification channel for webhook delivery.
	ChannelWebhook = "webhook"

	// WebhookSignatureHeader is the default HMAC signature header.
	WebhookSignatureHeader = "X-SaaS-Signature"

	// WebhookTimestampHeader is the default HMAC timestamp header.
	WebhookTimestampHeader = "X-SaaS-Timestamp"

	webhookTenantHeader     = "X-SaaS-Tenant-ID"
	webhookChannelHeader    = "X-SaaS-Channel"
	idempotencyKeyHeader    = "Idempotency-Key"
	defaultWebhookTimeout   = 10 * time.Second
	defaultHTTPUserAgent    = "saas"
	defaultWebhookBodyLimit = 4096
)

// WebhookConfig configures WebhookNotifier.
type WebhookConfig struct {
	Endpoint          string
	Channel           string
	Headers           map[string]string
	Secret            []byte
	SignatureHeader   string
	TimestampHeader   string
	Timeout           time.Duration
	MaxResponseBytes  int64
	Client            *http.Client
	Now               func() time.Time
	AllowInsecureHTTP bool
}

// WebhookPayload is the JSON request body sent by WebhookNotifier.
type WebhookPayload struct {
	ID       string            `json:"id,omitempty"`
	TenantID string            `json:"tenant_id"`
	Channel  string            `json:"channel"`
	To       string            `json:"to"`
	Subject  string            `json:"subject,omitempty"`
	Body     string            `json:"body,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// WebhookStatusError describes a non-2xx webhook response.
type WebhookStatusError struct {
	StatusCode int
	Body       string
}

// Error returns a safe delivery error without embedding provider response body.
func (err *WebhookStatusError) Error() string {
	if err == nil {
		return ErrWebhookDelivery.Error()
	}
	return fmt.Sprintf("%s: status %d", ErrWebhookDelivery, err.StatusCode)
}

// Unwrap returns the sentinel webhook delivery error.
func (err *WebhookStatusError) Unwrap() error {
	return ErrWebhookDelivery
}

// Retryable reports whether the status is normally safe to retry.
func (err *WebhookStatusError) Retryable() bool {
	if err == nil {
		return false
	}
	return err.StatusCode == http.StatusTooManyRequests || err.StatusCode >= http.StatusInternalServerError
}

// WebhookNotifier sends notifications to an HTTP endpoint as JSON.
type WebhookNotifier struct {
	endpoint        url.URL
	channel         string
	headers         http.Header
	secret          []byte
	signatureHeader string
	timestampHeader string
	client          *http.Client
	now             func() time.Time
	maxResponseBody int64
}

var _ Notifier = (*WebhookNotifier)(nil)

// NewWebhookNotifier creates an HTTP webhook notifier.
func NewWebhookNotifier(config WebhookConfig) (*WebhookNotifier, error) {
	config = normalizeWebhookConfig(config)
	endpoint, err := validateWebhookConfig(config)
	if err != nil {
		return nil, err
	}

	client := config.Client
	if client == nil {
		client = &http.Client{
			Timeout:       config.Timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}

	headers := make(http.Header, len(config.Headers))
	for key, value := range config.Headers {
		headers.Set(key, value)
	}
	secret := append([]byte(nil), config.Secret...)

	return &WebhookNotifier{
		endpoint:        *endpoint,
		channel:         config.Channel,
		headers:         headers,
		secret:          secret,
		signatureHeader: config.SignatureHeader,
		timestampHeader: config.TimestampHeader,
		client:          client,
		now:             config.Now,
		maxResponseBody: config.MaxResponseBytes,
	}, nil
}

// Send delivers message to the configured webhook endpoint.
func (notifier *WebhookNotifier) Send(ctx context.Context, message Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if notifier == nil {
		return ErrNilNotifier
	}
	if err := message.Validate(); err != nil {
		return err
	}
	if message.Channel != notifier.channel {
		return ErrUnsupportedChannel
	}

	request, err := notifier.request(ctx, message.Clone())
	if err != nil {
		return err
	}
	response, err := notifier.client.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, notifier.maxResponseBody))
		return nil
	}

	body, readErr := io.ReadAll(io.LimitReader(response.Body, notifier.maxResponseBody))
	if readErr != nil {
		return errors.Join(&WebhookStatusError{StatusCode: response.StatusCode}, readErr)
	}
	return &WebhookStatusError{StatusCode: response.StatusCode, Body: string(body)}
}

func (notifier *WebhookNotifier) request(ctx context.Context, message Message) (*http.Request, error) {
	payload := WebhookPayload{
		ID:       message.ID,
		TenantID: message.TenantID.String(),
		Channel:  message.Channel,
		To:       message.To,
		Subject:  message.Subject,
		Body:     message.Body,
		Metadata: message.Metadata,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, notifier.endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, values := range notifier.headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", defaultHTTPUserAgent)
	request.Header.Set(webhookTenantHeader, message.TenantID.String())
	request.Header.Set(webhookChannelHeader, message.Channel)
	if message.ID != "" {
		request.Header.Set(idempotencyKeyHeader, message.ID)
	}
	if len(notifier.secret) > 0 {
		timestamp := strconv.FormatInt(notifier.now().UTC().Unix(), 10)
		request.Header.Set(notifier.timestampHeader, timestamp)
		request.Header.Set(notifier.signatureHeader, signWebhook(timestamp, body, notifier.secret))
	}
	return request, nil
}

func normalizeWebhookConfig(config WebhookConfig) WebhookConfig {
	config.Endpoint = strings.TrimSpace(config.Endpoint)
	config.Channel = strings.TrimSpace(config.Channel)
	config.SignatureHeader = strings.TrimSpace(config.SignatureHeader)
	config.TimestampHeader = strings.TrimSpace(config.TimestampHeader)
	if config.Channel == "" {
		config.Channel = ChannelWebhook
	}
	if config.SignatureHeader == "" {
		config.SignatureHeader = WebhookSignatureHeader
	}
	if config.TimestampHeader == "" {
		config.TimestampHeader = WebhookTimestampHeader
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultWebhookTimeout
	}
	if config.MaxResponseBytes <= 0 {
		config.MaxResponseBytes = defaultWebhookBodyLimit
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return config
}

func validateWebhookConfig(config WebhookConfig) (*url.URL, error) {
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, ErrInvalidWebhookConfig
	}
	if endpoint.Scheme == "" || endpoint.Host == "" || endpoint.User != nil || endpoint.Fragment != "" {
		return nil, ErrInvalidWebhookConfig
	}
	if endpoint.Scheme != "https" {
		if endpoint.Scheme != "http" || (!config.AllowInsecureHTTP && !isLoopbackHost(endpoint.Hostname())) {
			return nil, ErrInvalidWebhookConfig
		}
	}
	if config.Channel == "" || config.Timeout <= 0 || config.MaxResponseBytes <= 0 {
		return nil, ErrInvalidWebhookConfig
	}
	if invalidHTTPHeaderName(config.SignatureHeader) || invalidHTTPHeaderName(config.TimestampHeader) {
		return nil, ErrInvalidWebhookConfig
	}
	for key, value := range config.Headers {
		if invalidHTTPHeaderName(key) || hasHeaderInjection(value) {
			return nil, ErrInvalidWebhookConfig
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "host", "content-length", "transfer-encoding":
			return nil, ErrInvalidWebhookConfig
		}
	}
	return endpoint, nil
}

func invalidHTTPHeaderName(name string) bool {
	name = strings.TrimSpace(name)
	return name == "" || strings.ContainsAny(name, ":\r\n")
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func signWebhook(timestamp string, body []byte, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
