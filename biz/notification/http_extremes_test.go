package notification

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPNotifiersDefaultClientsDoNotFollowCredentialBearingRedirects(t *testing.T) {
	for _, tt := range notificationHTTPTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			redirectTarget := make(chan http.Header, 1)
			target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				redirectTarget <- request.Header.Clone()
				writer.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(target.Close)

			origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				for _, header := range tt.credentialHeaders {
					if request.Header.Get(header) == "" {
						t.Errorf("origin request missing %s", header)
					}
				}
				http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
			}))
			t.Cleanup(origin.Close)

			err := tt.newSender(t, origin.URL, nil, time.Second, 128)(context.Background())
			status, _, ok := tt.statusError(err)
			if !ok || status != http.StatusTemporaryRedirect {
				t.Fatalf("Send() error = %v, want status error %d", err, http.StatusTemporaryRedirect)
			}

			select {
			case headers := <-redirectTarget:
				t.Fatalf("default client followed redirect and exposed credentials: authorization=%q signature=%q timestamp=%q", headers.Get("Authorization"), headers.Get(WebhookSignatureHeader), headers.Get(WebhookTimestampHeader))
			default:
			}
		})
	}
}

func TestHTTPNotifiersTruncateAndRedactProviderErrorBodies(t *testing.T) {
	const responseLimit = 32
	const sensitiveMarker = "provider-private-diagnostic"
	providerBody := strings.Repeat(sensitiveMarker+"-", 8)

	for _, tt := range notificationHTTPTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusBadGateway)
				_, _ = writer.Write([]byte(providerBody))
			}))
			t.Cleanup(provider.Close)

			err := tt.newSender(t, provider.URL, nil, time.Second, responseLimit)(context.Background())
			status, body, ok := tt.statusError(err)
			if !ok || status != http.StatusBadGateway {
				t.Fatalf("Send() error = %v, want status error %d", err, http.StatusBadGateway)
			}
			if len(body) != responseLimit || body != providerBody[:responseLimit] {
				t.Fatalf("captured provider body = %q (len %d), want first %d bytes", body, len(body), responseLimit)
			}
			if strings.Contains(err.Error(), sensitiveMarker) || strings.Contains(err.Error(), body) {
				t.Fatalf("delivery error leaks provider body: %q", err)
			}
		})
	}
}

func TestHTTPNotifiersHonorDefaultClientTimeout(t *testing.T) {
	for _, tt := range notificationHTTPTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			requestStarted := make(chan struct{})
			releaseProvider := make(chan struct{})
			defer close(releaseProvider)
			provider := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				close(requestStarted)
				<-releaseProvider
			}))
			t.Cleanup(provider.Close)

			err := tt.newSender(t, provider.URL, nil, 35*time.Millisecond, 128)(context.Background())
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Send() timeout error = %v, want context deadline exceeded", err)
			}
			select {
			case <-requestStarted:
			default:
				t.Fatal("Send() returned before making a provider request")
			}
		})
	}
}

func TestHTTPNotifiersHonorCallerCancellation(t *testing.T) {
	for _, tt := range notificationHTTPTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			requestStarted := make(chan struct{})
			releaseProvider := make(chan struct{})
			defer close(releaseProvider)
			provider := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				close(requestStarted)
				<-releaseProvider
			}))
			t.Cleanup(provider.Close)

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			go func() {
				select {
				case <-requestStarted:
					cancel()
				case <-time.After(time.Second):
				}
			}()

			err := tt.newSender(t, provider.URL, nil, time.Second, 128)(ctx)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Send() cancellation error = %v, want context canceled", err)
			}
		})
	}
}

func TestHTTPNotifiersPropagateClientTransportFailures(t *testing.T) {
	transportFailure := errors.New("provider transport unavailable")

	for _, tt := range notificationHTTPTestCases() {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: notificationRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return nil, transportFailure
			})}

			err := tt.newSender(t, "http://127.0.0.1:1/notify", client, time.Second, 128)(context.Background())
			if !errors.Is(err, transportFailure) {
				t.Fatalf("Send() error = %v, want transport failure", err)
			}
			if calls != 1 {
				t.Fatalf("transport calls = %d, want 1", calls)
			}
		})
	}
}

type notificationHTTPTestCase struct {
	name              string
	credentialHeaders []string
	newSender         func(t *testing.T, endpoint string, client *http.Client, timeout time.Duration, maxResponseBytes int64) func(context.Context) error
	statusError       func(error) (status int, body string, ok bool)
}

func notificationHTTPTestCases() []notificationHTTPTestCase {
	return []notificationHTTPTestCase{
		{
			name:              "resend",
			credentialHeaders: []string{"Authorization"},
			newSender:         newResendHTTPSender,
			statusError: func(err error) (int, string, bool) {
				var statusError *ResendStatusError
				if !errors.As(err, &statusError) {
					return 0, "", false
				}
				return statusError.StatusCode, statusError.Body, true
			},
		},
		{
			name:              "webhook",
			credentialHeaders: []string{"Authorization", WebhookSignatureHeader, WebhookTimestampHeader},
			newSender:         newWebhookHTTPSender,
			statusError: func(err error) (int, string, bool) {
				var statusError *WebhookStatusError
				if !errors.As(err, &statusError) {
					return 0, "", false
				}
				return statusError.StatusCode, statusError.Body, true
			},
		},
	}
}

func newResendHTTPSender(t *testing.T, endpoint string, client *http.Client, timeout time.Duration, maxResponseBytes int64) func(context.Context) error {
	t.Helper()
	notifier, err := NewResendNotifier(ResendConfig{
		APIKey:           "re_test",
		From:             "noreply@example.com",
		Endpoint:         endpoint,
		Timeout:          timeout,
		MaxResponseBytes: maxResponseBytes,
		Client:           client,
	})
	if err != nil {
		t.Fatalf("NewResendNotifier() error = %v", err)
	}
	message := testMessage(ChannelEmail)
	message.Body = "body"
	return func(ctx context.Context) error {
		return notifier.Send(ctx, message)
	}
}

func newWebhookHTTPSender(t *testing.T, endpoint string, client *http.Client, timeout time.Duration, maxResponseBytes int64) func(context.Context) error {
	t.Helper()
	notifier, err := NewWebhookNotifier(WebhookConfig{
		Endpoint:         endpoint,
		Headers:          map[string]string{"Authorization": "Bearer webhook-token"},
		Secret:           []byte("webhook-secret"),
		Timeout:          timeout,
		MaxResponseBytes: maxResponseBytes,
		Client:           client,
	})
	if err != nil {
		t.Fatalf("NewWebhookNotifier() error = %v", err)
	}
	message := testMessage(ChannelWebhook)
	message.Body = "body"
	return func(ctx context.Context) error {
		return notifier.Send(ctx, message)
	}
}

type notificationRoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn notificationRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
