package notification

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/textproto"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events/contracts"
)

// Handler processes notification-related events.
type Handler struct {
	sender Sender
	tmpl   *template.Template
}

// NewHandler constructs the notification handler.
func NewHandler(sender Sender) *Handler {
	tmpl := template.Must(template.New("welcome").Parse(welcomeTemplate))
	return &Handler{sender: sender, tmpl: tmpl}
}

// HandleUserCreated sends a welcome email when a user is created.
func (h *Handler) HandleUserCreated(msg *message.Message) error {
	var event contracts.UserCreatedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		slog.Error("notification: failed to unmarshal event", "err", err,
			"msg_id", msg.UUID)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}

	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, event); err != nil {
		slog.Error("notification: failed to render template", "err", err)
		return err
	}

	ctx := msg.Context()
	if err := h.sender.Send(ctx, event.Email, "Welcome!", buf.String()); err != nil {
		slog.ErrorContext(ctx, "notification: failed to send email", "err", err, "to", event.Email)
		// Permanent SMTP errors (5xx) won't be fixed by retrying -- ack them.
		if isPermanentSMTPError(err) {
			slog.Warn("notification: permanent SMTP error, acking message", "err", err)
			return nil
		}
		return err
	}

	slog.InfoContext(ctx, "notification: welcome email sent", "to", event.Email)
	return nil
}

// isPermanentSMTPError checks if the error wraps an SMTP 5xx response.
// SMTP 5xx codes indicate permanent failures (bad address, policy reject, etc.)
// that won't be resolved by retrying.
func isPermanentSMTPError(err error) bool {
	var tpErr *textproto.Error
	if errors.As(err, &tpErr) {
		return tpErr.Code >= 500 && tpErr.Code < 600
	}
	return false
}

const welcomeTemplate = `<!DOCTYPE html>
<html>
<body>
  <h1>Welcome, {{.Name}}!</h1>
  <p>Your account has been created with the email <strong>{{.Email}}</strong>.</p>
  <p>Role: {{.Role}}</p>
</body>
</html>`
