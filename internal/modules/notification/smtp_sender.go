package notification

import (
	"context"
	"fmt"
	"mime"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
)

// SMTPSender sends emails via SMTP.
type SMTPSender struct {
	host      string
	port      int
	from      string
	fromAlias string
	user      string
	password  string
}

// NewSMTPSender creates a new SMTP-based notification sender.
func NewSMTPSender(cfg *config.Config) Sender {
	return &SMTPSender{
		host:      cfg.SMTPHost,
		port:      cfg.SMTPPort,
		from:      cfg.SMTPFrom,
		fromAlias: cfg.SMTPFromAlias,
		user:      cfg.SMTPUser,
		password:  cfg.SMTPPassword,
	}
}

// Send delivers an email via SMTP with CRLF-injection protection.
func (s *SMTPSender) Send(ctx context.Context, to, subject, body string) error {
	if _, err := mail.ParseAddress(to); err != nil {
		return fmt.Errorf("invalid email address %q: %w", to, err)
	}

	// Sanitize header values to prevent CRLF injection
	sanitize := func(v string) string {
		return strings.NewReplacer("\r", "", "\n", "").Replace(v)
	}

	fromHeader := sanitize(s.from)
	if s.fromAlias != "" {
		fromHeader = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", sanitize(s.fromAlias)), sanitize(s.from))
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		fromHeader, sanitize(to), mime.QEncoding.Encode("utf-8", sanitize(subject)), body)

	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.password, s.host)
	}

	// Hard 30s deadline guards against SMTP servers that accept the connection
	// but never respond. The goroutine may leak until TCP timeout after the
	// deadline fires, but the leak is bounded and acceptable without a custom
	// SMTP client that supports context cancellation natively.
	const smtpHardTimeout = 30 * time.Second
	smtpCtx, smtpCancel := context.WithTimeout(ctx, smtpHardTimeout)
	defer smtpCancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- smtp.SendMail(addr, auth, sanitize(s.from), []string{sanitize(to)}, []byte(msg))
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("sending email to %s: %w", to, err)
		}
		return nil
	case <-smtpCtx.Done():
		return fmt.Errorf("sending email to %s: %w", to, smtpCtx.Err())
	}
}
