package notification

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSMTPNotifierStopsBeforeDATAWhenRecipientIsRejected(t *testing.T) {
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
	notifier.dial = func(context.Context, string, string) (net.Conn, error) { return clientConn, nil }
	t.Cleanup(func() { _ = clientConn.Close() })
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
		if err := writeSMTPResponse(writer, "250 smtp.example.com"); err != nil {
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
		if _, err := expectSMTPCommand(reader, "RCPT TO:<user@example.com>"); err != nil {
			serverDone <- err
			return
		}
		if err := writeSMTPResponse(writer, "550 recipient unavailable"); err != nil {
			serverDone <- err
			return
		}

		// SMTP is request/response ordered: the client cannot issue DATA until it
		// has received this RCPT response. A 550 therefore proves no message body
		// can have been accepted for the rejected recipient.
		serverDone <- nil
	}()

	if err := notifier.Send(context.Background(), smtpTestMessage()); err == nil {
		t.Fatal("Send() error = nil, want recipient rejection")
	}
	awaitSMTPServer(t, serverDone)
}

func TestSMTPNotifierReturnsRelayDATARejection(t *testing.T) {
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
	notifier.dial = func(context.Context, string, string) (net.Conn, error) { return clientConn, nil }
	t.Cleanup(func() { _ = clientConn.Close() })
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
		if err := writeSMTPResponse(writer, "250 smtp.example.com"); err != nil {
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
		if _, err := expectSMTPCommand(reader, "RCPT TO:<user@example.com>"); err != nil {
			serverDone <- err
			return
		}
		if err := writeSMTPResponse(writer, "250 recipient accepted"); err != nil {
			serverDone <- err
			return
		}
		if _, err := expectSMTPCommand(reader, "DATA"); err != nil {
			serverDone <- err
			return
		}
		if err := writeSMTPResponse(writer, "354 send body"); err != nil {
			serverDone <- err
			return
		}
		var body strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				serverDone <- err
				return
			}
			if line == ".\r\n" {
				break
			}
			body.WriteString(line)
		}
		if !strings.Contains(body.String(), "Subject: Subject") {
			serverDone <- fmt.Errorf("SMTP DATA = %q, want message headers", body.String())
			return
		}
		serverDone <- writeSMTPResponse(writer, "554 message rejected after DATA")
	}()

	if err := notifier.Send(context.Background(), smtpTestMessage()); err == nil {
		t.Fatal("Send() error = nil, want DATA rejection")
	}
	awaitSMTPServer(t, serverDone)
}

func awaitSMTPServer(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("scripted SMTP server error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("scripted SMTP server did not complete")
	}
}
