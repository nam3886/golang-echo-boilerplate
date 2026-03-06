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

func (s *SMTPSender) Send(_ context.Context, to, subject, body string) error {
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

	if err := smtp.SendMail(addr, auth, sanitize(s.from), []string{sanitize(to)}, []byte(msg)); err != nil {
		return fmt.Errorf("sending email to %s: %w", to, err)
	}
	return nil
}
