package notification

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNewSMTPNotifierValidation(t *testing.T) {
	notifier, err := NewSMTPNotifier(SMTPConfig{
		Host: "smtp.example.com",
		From: "GoTenancy <noreply@example.com>",
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

	raw, err := buildSMTPMessage("GoTenancy <noreply@example.com>", recipients, Message{
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
		"From: GoTenancy <noreply@example.com>\r\n",
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
