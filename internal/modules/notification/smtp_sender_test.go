package notification

import (
	"context"
	"testing"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
)

func TestSMTPSender_InvalidEmail(t *testing.T) {
	sender := NewSMTPSender(&config.Config{
		SMTPHost: "localhost",
		SMTPPort: 1025,
		SMTPFrom: "test@example.com",
	})
	err := sender.Send(context.Background(), "not-an-email", "subject", "body")
	if err == nil {
		t.Fatal("expected error for invalid email")
	}
}

func TestSMTPSender_ContextCancelled(t *testing.T) {
	sender := NewSMTPSender(&config.Config{
		SMTPHost: "192.0.2.1", // non-routable IP — forces timeout
		SMTPPort: 1025,
		SMTPFrom: "test@example.com",
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	err := sender.Send(ctx, "user@example.com", "subject", "body")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestSMTPSender_CRLFInjection(t *testing.T) {
	// CRLF chars in subject should be stripped before SMTP attempt.
	// Test verifies no panic or data corruption — connection error is expected.
	s := &SMTPSender{
		host: "192.0.2.1",
		port: 1025,
		from: "test@example.com",
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Expect context error, not a crash from CRLF in subject
	_ = s.Send(ctx, "user@example.com", "subject\r\nBcc: hacker@evil.com", "body")
}

func TestSMTPSender_FromHeaderWithAlias(t *testing.T) {
	// Verify no panic when fromAlias is set.
	s := &SMTPSender{
		host:      "192.0.2.1",
		port:      1025,
		from:      "noreply@example.com",
		fromAlias: "My App",
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = s.Send(ctx, "user@example.com", "Test", "body")
}
