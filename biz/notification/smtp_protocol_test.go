package notification

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestSMTPNotifierSendsCompleteMessageOverSMTPProtocol(t *testing.T) {
	notifier, err := NewSMTPNotifier(SMTPConfig{
		Host:    "smtp.example.com",
		From:    "GoTenancy <noreply@example.com>",
		TLSMode: SMTPTLSModeNone,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewSMTPNotifier() error = %v", err)
	}

	clientConn, serverConn := net.Pipe()
	notifier.dial = func(context.Context, string, string) (net.Conn, error) {
		return clientConn, nil
	}
	defer func() { _ = clientConn.Close() }()

	received := make(chan string, 1)
	serverDone := make(chan error, 1)
	go func() {
		defer func() { _ = serverConn.Close() }()
		reader := bufio.NewReader(serverConn)
		writer := bufio.NewWriter(serverConn)
		if err := writeSMTPResponse(writer, "220 smtp.example.com ready"); err != nil {
			serverDone <- err
			return
		}
		if _, err := expectSMTPCommand(reader, "EHLO "); err != nil {
			serverDone <- err
			return
		}
		if err := writeSMTPResponse(writer, "250-smtp.example.com", "250 8BITMIME"); err != nil {
			serverDone <- err
			return
		}
		if _, err := expectSMTPCommand(reader, "MAIL FROM:<noreply@example.com>"); err != nil {
			serverDone <- err
			return
		}
		if err := writeSMTPResponse(writer, "250 sender accepted"); err != nil {
			serverDone <- err
			return
		}
		for _, recipient := range []string{"first@example.com", "second@example.com"} {
			if _, err := expectSMTPCommand(reader, "RCPT TO:<"+recipient+">"); err != nil {
				serverDone <- err
				return
			}
			if err := writeSMTPResponse(writer, "250 recipient accepted"); err != nil {
				serverDone <- err
				return
			}
		}
		if _, err := expectSMTPCommand(reader, "DATA"); err != nil {
			serverDone <- err
			return
		}
		if err := writeSMTPResponse(writer, "354 end data with <CR><LF>.<CR><LF>"); err != nil {
			serverDone <- err
			return
		}

		var data strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				serverDone <- err
				return
			}
			if line == ".\r\n" {
				break
			}
			data.WriteString(line)
		}
		received <- data.String()
		if err := writeSMTPResponse(writer, "250 queued"); err != nil {
			serverDone <- err
			return
		}
		if _, err := expectSMTPCommand(reader, "QUIT"); err != nil {
			serverDone <- err
			return
		}
		serverDone <- writeSMTPResponse(writer, "221 bye")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = notifier.Send(ctx, Message{
		TenantID: "tenant-a",
		Channel:  ChannelEmail,
		To:       "first@example.com, second@example.com",
		Subject:  "Tenant update",
		Body:     "first line\nsecond line",
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("scripted SMTP server error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("scripted SMTP server did not complete")
	}

	data := <-received
	for _, want := range []string{
		"From: \"GoTenancy\" <noreply@example.com>\r\n",
		"To: <first@example.com>, <second@example.com>\r\n",
		"Subject: Tenant update\r\n",
		"\r\nfirst line\r\nsecond line",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("SMTP DATA %q does not contain %q", data, want)
		}
	}
}

func TestSMTPNotifierRequiresAdvertisedStartTLS(t *testing.T) {
	notifier, err := NewSMTPNotifier(SMTPConfig{
		Host:    "smtp.example.com",
		From:    "noreply@example.com",
		TLSMode: SMTPTLSModeStartTLS,
	})
	if err != nil {
		t.Fatalf("NewSMTPNotifier() error = %v", err)
	}

	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	serverDone := make(chan error, 1)
	go func() {
		defer func() { _ = serverConn.Close() }()
		reader := bufio.NewReader(serverConn)
		writer := bufio.NewWriter(serverConn)
		if err := writeSMTPResponse(writer, "220 smtp.example.com ready"); err != nil {
			serverDone <- err
			return
		}
		if _, err := expectSMTPCommand(reader, "EHLO "); err != nil {
			serverDone <- err
			return
		}
		serverDone <- writeSMTPResponse(writer, "250 smtp.example.com")
	}()

	client, err := smtp.NewClient(clientConn, "smtp.example.com")
	if err != nil {
		t.Fatalf("smtp.NewClient() error = %v", err)
	}
	if err := notifier.secure(client); !errors.Is(err, ErrTLSRequired) {
		t.Fatalf("secure() error = %v, want ErrTLSRequired", err)
	}
	if err := <-serverDone; err != nil {
		t.Fatalf("scripted SMTP server error = %v", err)
	}
}

func TestSMTPNotifierAuthenticatesForLocalRelayAndBuildsTLSConfig(t *testing.T) {
	notifier, err := NewSMTPNotifier(SMTPConfig{
		Host:       "localhost",
		ServerName: "localhost",
		Username:   "mailer",
		Password:   "secret",
		From:       "noreply@example.com",
		TLSMode:    SMTPTLSModeNone,
	})
	if err != nil {
		t.Fatalf("NewSMTPNotifier() error = %v", err)
	}

	config := notifier.tlsConfig()
	if config.ServerName != "localhost" || config.MinVersion != tls.VersionTLS12 {
		t.Fatalf("tlsConfig() = %+v, want localhost and TLS 1.2 minimum", config)
	}
	if SMTPTLSModeImplicitTLS.String() != "tls" {
		t.Fatalf("SMTPTLSModeImplicitTLS.String() = %q, want tls", SMTPTLSModeImplicitTLS.String())
	}

	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	serverDone := make(chan error, 1)
	go func() {
		defer func() { _ = serverConn.Close() }()
		reader := bufio.NewReader(serverConn)
		writer := bufio.NewWriter(serverConn)
		if err := writeSMTPResponse(writer, "220 localhost ready"); err != nil {
			serverDone <- err
			return
		}
		if _, err := expectSMTPCommand(reader, "EHLO "); err != nil {
			serverDone <- err
			return
		}
		if err := writeSMTPResponse(writer, "250-localhost", "250 AUTH PLAIN"); err != nil {
			serverDone <- err
			return
		}
		if _, err := expectSMTPCommand(reader, "AUTH PLAIN "); err != nil {
			serverDone <- err
			return
		}
		serverDone <- writeSMTPResponse(writer, "235 authenticated")
	}()

	client, err := smtp.NewClient(clientConn, "localhost")
	if err != nil {
		t.Fatalf("smtp.NewClient() error = %v", err)
	}
	if err := notifier.authenticate(client); err != nil {
		t.Fatalf("authenticate() error = %v", err)
	}
	if err := <-serverDone; err != nil {
		t.Fatalf("scripted SMTP server error = %v", err)
	}
}

func TestSMTPNotifierRejectsInvalidImplicitTLSHandshake(t *testing.T) {
	notifier, err := NewSMTPNotifier(SMTPConfig{
		Host:    "smtp.example.com",
		From:    "noreply@example.com",
		TLSMode: SMTPTLSModeImplicitTLS,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewSMTPNotifier() error = %v", err)
	}

	clientConn, serverConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	defer func() { _ = serverConn.Close() }()
	notifier.dial = func(context.Context, string, string) (net.Conn, error) {
		return clientConn, nil
	}
	go func() {
		buffer := make([]byte, 1024)
		_, _ = serverConn.Read(buffer)
		_, _ = serverConn.Write([]byte("not a TLS handshake"))
	}()

	if _, stop, err := notifier.smtpClient(context.Background(), "smtp.example.com:465"); err == nil || stop != nil {
		t.Fatalf("smtpClient(implicit TLS invalid handshake) returned stop=%t err=%v; want nil stop and TLS error", stop != nil, err)
	}
}

func expectSMTPCommand(reader *bufio.Reader, prefix string) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("SMTP command = %q, want prefix %q", line, prefix)
	}
	return line, nil
}

func writeSMTPResponse(writer *bufio.Writer, lines ...string) error {
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\r\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}
