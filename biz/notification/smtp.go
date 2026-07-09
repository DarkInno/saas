package notification

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

const (
	// ChannelEmail is the conventional notification channel for email delivery.
	ChannelEmail = "email"

	defaultSMTPPort    = 587
	defaultSMTPTimeout = 10 * time.Second
)

// SMTPTLSMode controls how SMTPNotifier secures the SMTP connection.
type SMTPTLSMode string

const (
	// SMTPTLSModeStartTLS starts plaintext, then requires STARTTLS before auth or delivery.
	SMTPTLSModeStartTLS SMTPTLSMode = "starttls"

	// SMTPTLSModeImplicitTLS uses TLS from the first byte, commonly on port 465.
	SMTPTLSModeImplicitTLS SMTPTLSMode = "tls"

	// SMTPTLSModeNone disables TLS. Use only for trusted local relays.
	SMTPTLSModeNone SMTPTLSMode = "none"
)

// SMTPConfig configures SMTPNotifier.
type SMTPConfig struct {
	Host       string
	Port       int
	ServerName string
	Username   string
	Password   string
	From       string
	Channel    string
	TLSMode    SMTPTLSMode
	Timeout    time.Duration
}

// SMTPNotifier sends email notifications through SMTP.
type SMTPNotifier struct {
	config SMTPConfig
	dial   func(context.Context, string, string) (net.Conn, error)
}

var _ Notifier = (*SMTPNotifier)(nil)

// NewSMTPNotifier creates an SMTP-backed notifier.
func NewSMTPNotifier(config SMTPConfig) (*SMTPNotifier, error) {
	config = normalizeSMTPConfig(config)
	if err := validateSMTPConfig(config); err != nil {
		return nil, err
	}

	dialer := &net.Dialer{Timeout: config.Timeout}
	return &SMTPNotifier{
		config: config,
		dial:   dialer.DialContext,
	}, nil
}

// Send delivers a notification message through SMTP.
func (notifier *SMTPNotifier) Send(ctx context.Context, message Message) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if notifier == nil {
		return ErrInvalidSMTPConfig
	}
	if err := validateSMTPMessage(notifier.config, message); err != nil {
		return err
	}

	from, err := parseSingleAddress(notifier.config.From)
	if err != nil {
		return err
	}
	recipients, err := parseAddressList(message.To)
	if err != nil {
		return err
	}
	body, err := buildSMTPMessage(from.String(), recipients, message)
	if err != nil {
		return err
	}

	address := net.JoinHostPort(notifier.config.Host, strconv.Itoa(notifier.config.Port))
	client, err := notifier.smtpClient(ctx, address)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			err = errors.Join(err, client.Close())
		}
	}()

	if err := notifier.secure(client); err != nil {
		return err
	}
	if err := notifier.authenticate(client); err != nil {
		return err
	}
	if err := client.Mail(from.Address); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient.Address); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := client.Quit(); err != nil {
		return err
	}
	closed = true
	return nil
}

func (notifier *SMTPNotifier) smtpClient(ctx context.Context, address string) (*smtp.Client, error) {
	switch notifier.config.TLSMode {
	case SMTPTLSModeImplicitTLS:
		conn, err := notifier.dial(ctx, "tcp", address)
		if err != nil {
			return nil, err
		}
		tlsConn := tls.Client(conn, notifier.tlsConfig())
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return smtp.NewClient(tlsConn, notifier.config.ServerName)
	default:
		conn, err := notifier.dial(ctx, "tcp", address)
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, notifier.config.ServerName)
	}
}

func (notifier *SMTPNotifier) secure(client *smtp.Client) error {
	switch notifier.config.TLSMode {
	case SMTPTLSModeImplicitTLS:
		return nil
	case SMTPTLSModeStartTLS:
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return ErrTLSRequired
		}
		return client.StartTLS(notifier.tlsConfig())
	case SMTPTLSModeNone:
		return nil
	default:
		return ErrInvalidSMTPConfig
	}
}

func (notifier *SMTPNotifier) authenticate(client *smtp.Client) error {
	if notifier.config.Username == "" && notifier.config.Password == "" {
		return nil
	}
	auth := smtp.PlainAuth("", notifier.config.Username, notifier.config.Password, notifier.config.ServerName)
	return client.Auth(auth)
}

func (notifier *SMTPNotifier) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName: notifier.config.ServerName,
		MinVersion: tls.VersionTLS12,
	}
}

func normalizeSMTPConfig(config SMTPConfig) SMTPConfig {
	config.Host = strings.TrimSpace(config.Host)
	config.ServerName = strings.TrimSpace(config.ServerName)
	config.Username = strings.TrimSpace(config.Username)
	config.From = strings.TrimSpace(config.From)
	config.Channel = strings.TrimSpace(config.Channel)
	if config.Port == 0 {
		config.Port = defaultSMTPPort
	}
	if config.ServerName == "" {
		config.ServerName = config.Host
	}
	if config.Channel == "" {
		config.Channel = ChannelEmail
	}
	if config.TLSMode == "" {
		config.TLSMode = SMTPTLSModeStartTLS
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultSMTPTimeout
	}
	return config
}

func validateSMTPConfig(config SMTPConfig) error {
	if config.Host == "" || config.Port <= 0 || config.ServerName == "" || config.From == "" {
		return ErrInvalidSMTPConfig
	}
	switch config.TLSMode {
	case SMTPTLSModeStartTLS, SMTPTLSModeImplicitTLS, SMTPTLSModeNone:
	default:
		return ErrInvalidSMTPConfig
	}
	if hasHeaderInjection(config.From) || hasHeaderInjection(config.ServerName) || hasHeaderInjection(config.Channel) {
		return ErrInvalidSMTPConfig
	}
	if _, err := parseSingleAddress(config.From); err != nil {
		return ErrInvalidSMTPConfig
	}
	return nil
}

func validateSMTPMessage(config SMTPConfig, message Message) error {
	if message.TenantID == "" || message.Channel == "" || message.To == "" {
		return ErrInvalidMessage
	}
	if message.Channel != config.Channel {
		return ErrUnsupportedChannel
	}
	if hasHeaderInjection(message.To) || hasHeaderInjection(message.Subject) {
		return ErrInvalidMessage
	}
	if _, err := parseAddressList(message.To); err != nil {
		return ErrInvalidMessage
	}
	return nil
}

func buildSMTPMessage(from string, recipients []*mail.Address, message Message) ([]byte, error) {
	if hasHeaderInjection(from) || hasHeaderInjection(message.Subject) {
		return nil, ErrInvalidMessage
	}

	var buffer bytes.Buffer
	writeHeader(&buffer, "From", from)
	writeHeader(&buffer, "To", addressListString(recipients))
	writeHeader(&buffer, "Subject", mime.QEncoding.Encode("utf-8", message.Subject))
	writeHeader(&buffer, "MIME-Version", "1.0")
	writeHeader(&buffer, "Content-Type", `text/plain; charset="utf-8"`)
	writeHeader(&buffer, "Content-Transfer-Encoding", "8bit")
	buffer.WriteString("\r\n")
	buffer.WriteString(normalizeEmailBody(message.Body))
	return buffer.Bytes(), nil
}

func writeHeader(buffer *bytes.Buffer, key string, value string) {
	buffer.WriteString(key)
	buffer.WriteString(": ")
	buffer.WriteString(value)
	buffer.WriteString("\r\n")
}

func parseSingleAddress(value string) (*mail.Address, error) {
	address, err := mail.ParseAddress(value)
	if err != nil {
		return nil, err
	}
	if address.Address == "" {
		return nil, ErrInvalidMessage
	}
	return address, nil
}

func parseAddressList(value string) ([]*mail.Address, error) {
	addresses, err := mail.ParseAddressList(value)
	if err != nil {
		return nil, err
	}
	if len(addresses) == 0 {
		return nil, ErrInvalidMessage
	}
	return addresses, nil
}

func addressListString(addresses []*mail.Address) string {
	parts := make([]string, len(addresses))
	for i, address := range addresses {
		parts[i] = address.String()
	}
	return strings.Join(parts, ", ")
}

func hasHeaderInjection(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}

func (mode SMTPTLSMode) String() string {
	return string(mode)
}

func normalizeEmailBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	return strings.ReplaceAll(body, "\n", "\r\n")
}
