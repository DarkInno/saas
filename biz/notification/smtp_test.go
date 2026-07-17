package notification

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestNewSMTPNotifierValidation(t *testing.T) {
	notifier, err := NewSMTPNotifier(SMTPConfig{
		Host: "smtp.example.com",
		From: "SaaS <noreply@example.com>",
	})
	if err != nil {
		t.Fatalf("NewSMTPNotifier() error = %v", err)
	}
	if notifier.config.Port != defaultSMTPPort || notifier.config.Channel != ChannelEmail || notifier.config.TLSMode != SMTPTLSModeStartTLS {
		t.Fatalf("default SMTP config = %+v, want default port/channel/starttls", notifier.config)
	}

	if _, err := NewSMTPNotifier(SMTPConfig{Host: "smtp.example.com", From: "bad\r\nBcc: x@example.com"}); !errors.Is(err, ErrInvalidSMTPConfig) {
		t.Fatalf("NewSMTPNotifier(header injection) error = %v, want ErrInvalidSMTPConfig", err)
	}
	if _, err := NewSMTPNotifier(SMTPConfig{Host: "smtp.example.com", From: "noreply@example.com", TLSMode: "ssl3"}); !errors.Is(err, ErrInvalidSMTPConfig) {
		t.Fatalf("NewSMTPNotifier(bad tls mode) error = %v, want ErrInvalidSMTPConfig", err)
	}
}

func TestSMTPNotifierMessageValidation(t *testing.T) {
	notifier, err := NewSMTPNotifier(SMTPConfig{
		Host: "smtp.example.com",
		From: "noreply@example.com",
	})
	if err != nil {
		t.Fatalf("NewSMTPNotifier() error = %v", err)
	}

	err = validateSMTPMessage(notifier.config, Message{TenantID: "tenant-a", Channel: "sms", To: "user@example.com"})
	if !errors.Is(err, ErrUnsupportedChannel) {
		t.Fatalf("validateSMTPMessage(channel) error = %v, want ErrUnsupportedChannel", err)
	}
	err = validateSMTPMessage(notifier.config, Message{TenantID: "tenant-a", Channel: ChannelEmail, To: "user@example.com", Subject: "Hi\r\nBcc: x@example.com"})
	if !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("validateSMTPMessage(injection) error = %v, want ErrInvalidMessage", err)
	}
	if err := (*SMTPNotifier)(nil).Send(context.Background(), Message{TenantID: "tenant-a", Channel: ChannelEmail, To: "user@example.com"}); !errors.Is(err, ErrInvalidSMTPConfig) {
		t.Fatalf("nil SMTPNotifier Send() error = %v, want ErrInvalidSMTPConfig", err)
	}
}

func TestBuildSMTPMessage(t *testing.T) {
	recipients, err := parseAddressList("User <user@example.com>, other@example.com")
	if err != nil {
		t.Fatalf("parseAddressList() error = %v", err)
	}

	raw, err := buildSMTPMessage("SaaS <noreply@example.com>", recipients, Message{
		TenantID: "tenant-a",
		Channel:  ChannelEmail,
		To:       "User <user@example.com>, other@example.com",
		Subject:  "Welcome",
		Body:     "line1\nline2",
	})
	if err != nil {
		t.Fatalf("buildSMTPMessage() error = %v", err)
	}
	message := string(raw)
	for _, want := range []string{
		"From: SaaS <noreply@example.com>\r\n",
		"To: \"User\" <user@example.com>, <other@example.com>\r\n",
		"Subject: Welcome\r\n",
		"Content-Type: text/plain; charset=\"utf-8\"\r\n",
		"\r\nline1\r\nline2",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("SMTP message %q does not contain %q", message, want)
		}
	}
}

func TestSMTPNotifierHonorsContextAfterDial(t *testing.T) {
	notifier, err := NewSMTPNotifier(SMTPConfig{
		Host:    "smtp.example.com",
		From:    "noreply@example.com",
		TLSMode: SMTPTLSModeNone,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewSMTPNotifier() error = %v", err)
	}

	clientConn, serverConn := net.Pipe()
	defer func() { _ = serverConn.Close() }()
	notifier.dial = func(context.Context, string, string) (net.Conn, error) {
		return clientConn, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(20*time.Millisecond, cancel)
	defer timer.Stop()
	started := time.Now()
	err = notifier.Send(ctx, smtpTestMessage())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Send() error = %v, want context canceled", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Send() elapsed = %s, want prompt context cancellation", elapsed)
	}
}

func TestSMTPNotifierAppliesTimeoutAfterDial(t *testing.T) {
	notifier, err := NewSMTPNotifier(SMTPConfig{
		Host:    "smtp.example.com",
		From:    "noreply@example.com",
		TLSMode: SMTPTLSModeNone,
		Timeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewSMTPNotifier() error = %v", err)
	}

	clientConn, serverConn := net.Pipe()
	defer func() { _ = serverConn.Close() }()
	notifier.dial = func(context.Context, string, string) (net.Conn, error) {
		return clientConn, nil
	}

	started := time.Now()
	if err := notifier.Send(context.Background(), smtpTestMessage()); err == nil {
		t.Fatal("Send() error = nil, want SMTP timeout")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Send() elapsed = %s, want bounded by SMTP timeout", elapsed)
	}
}

func smtpTestMessage() Message {
	return Message{
		TenantID: "tenant-a",
		Channel:  ChannelEmail,
		To:       "user@example.com",
		Subject:  "Subject",
		Body:     "Body",
	}
}
