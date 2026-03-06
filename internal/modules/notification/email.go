package notification

import (
	"context"
	"fmt"
	"mime"
	"net/smtp"
	"strings"

	"github.com/gnha/gnha-services/internal/shared/config"
)

// SMTPSender sends emails via SMTP.
type SMTPSender struct {
	host string
	port int
	from string
}

// NewSMTPSender creates a new SMTP-based notification sender.
func NewSMTPSender(cfg *config.Config) Sender {
	return &SMTPSender{
		host: cfg.SMTPHost,
		port: cfg.SMTPPort,
		from: cfg.SMTPFrom,
	}
}

func (s *SMTPSender) Send(_ context.Context, to, subject, body string) error {
	// Sanitize header values to prevent CRLF injection
	sanitize := func(v string) string {
		return strings.NewReplacer("\r", "", "\n", "").Replace(v)
	}
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		sanitize(s.from), sanitize(to), mime.QEncoding.Encode("utf-8", sanitize(subject)), body)

	if err := smtp.SendMail(addr, nil, sanitize(s.from), []string{sanitize(to)}, []byte(msg)); err != nil {
		return fmt.Errorf("sending email to %s: %w", to, err)
	}
	return nil
}
