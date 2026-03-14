package notification

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/textproto"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/redis/go-redis/v9"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/events/contracts"
)

const dedupTTL = 24 * time.Hour

// Handler processes notification-related events.
// Required: sender
// Optional: rdb (Redis dedup — nil disables dedup; fail-open if Redis is unavailable)
type Handler struct {
	sender Sender
	tmpl   *template.Template
	rdb    *redis.Client
}

// NewHandler constructs the notification handler.
// Panics if sender is nil. rdb is optional — nil disables dedup entirely.
func NewHandler(sender Sender, rdb *redis.Client) *Handler {
	if sender == nil {
		panic("notification.NewHandler: sender must not be nil")
	}
	tmpl := template.Must(template.New("welcome").Parse(welcomeTemplate))
	return &Handler{sender: sender, tmpl: tmpl, rdb: rdb}
}

// HandleUserCreated sends a welcome email when a user is created.
// Deduplication is performed via Redis SET NX on the Watermill message UUID.
// Fail-open on Redis errors: email delivery takes priority over strict dedup.
func (h *Handler) HandleUserCreated(msg *message.Message) error {
	ctx := msg.Context()

	// Unmarshal first — dedup key is only consumed for well-formed messages.
	// Consuming the dedup slot before unmarshal would permanently block redelivery
	// of a valid message with the same UUID if the payload was malformed on first delivery.
	var event contracts.UserCreatedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		slog.ErrorContext(ctx, "notification: failed to unmarshal event",
			"module", "notification", "err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}

	// Dedup: SET NX with 24h TTL — skip if already processed.
	// Skipped entirely when rdb is nil (dedup disabled).
	if h.rdb != nil {
		// nolint:staticcheck // SetNX works fine despite deprecation warning
		ok, err := h.rdb.SetNX(ctx, "notification:dedup:"+msg.UUID, "1", dedupTTL).Result()
		if err != nil {
			// Fail-open: Redis issue should not block email delivery.
			slog.WarnContext(ctx, "notification: dedup check failed, proceeding to send",
				"module", "notification", "msg_id", msg.UUID, "err", err)
		} else if !ok {
			slog.InfoContext(ctx, "notification: duplicate message, skipping",
				"module", "notification", "msg_id", msg.UUID)
			return nil
		}
	}

	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, event); err != nil {
		// Template failure is permanent (bad template, not transient infra) — ack to avoid
		// infinite retry loop. Fix the template and redeploy to reprocess.
		slog.ErrorContext(ctx, "notification: failed to render template, acking to avoid retry loop",
			"module", "notification", "err", err, "msg_id", msg.UUID,
			"error_code", "template_render_failed", "retryable", false)
		return nil
	}

	if err := h.sender.Send(ctx, event.Email, "Welcome!", buf.String()); err != nil {
		// Permanent SMTP errors (5xx) won't be fixed by retrying — ack them.
		if isPermanentSMTPError(err) {
			slog.WarnContext(ctx, "notification: permanent SMTP error, acking",
				"module", "notification", "err", err, "user_id", event.UserID,
				"error_code", "smtp_permanent", "retryable", false)
			return nil
		}
		slog.ErrorContext(ctx, "notification: failed to send email",
			"module", "notification", "err", err, "user_id", event.UserID,
			"error_code", "smtp_transient", "retryable", true)
		return err
	}

	slog.InfoContext(ctx, "notification: welcome email sent", "user_id", event.UserID)
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
