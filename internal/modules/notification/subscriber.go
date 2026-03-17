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
// Deduplication is performed via Redis SET NX on the event's EventID (not Watermill UUID),
// so publisher retries with a new Watermill message are still caught as duplicates.
// Fail-open on Redis errors: email delivery takes priority over strict dedup.
func (h *Handler) HandleUserCreated(msg *message.Message) error {
	ctx := msg.Context()

	// Unmarshal first — dedup key is only consumed for well-formed messages.
	// Consuming the dedup slot before unmarshal would permanently block redelivery
	// of a valid message with the same UUID if the payload was malformed on first delivery.
	var event contracts.UserCreatedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		slog.ErrorContext(ctx, "notification: failed to unmarshal event",
			"module", "notification", "operation", "HandleUserCreated",
			"err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}

	// Dedup check: skip if already sent. Uses event.EventID (not msg.UUID) so that
	// publisher retries with a new Watermill UUID are still caught as duplicates.
	// The dedup key is only committed AFTER successful send to prevent silent message loss
	// (crash between SetNX and Send would permanently silence the message).
	dedupKey := event.EventID
	if dedupKey == "" {
		dedupKey = msg.UUID // fallback for legacy events without EventID
	}
	if h.rdb != nil {
		// nolint:staticcheck // SetNX works fine despite deprecation warning
		exists, err := h.rdb.Exists(ctx, "notification:dedup:"+dedupKey).Result()
		if err != nil {
			// Fail-open: Redis issue should not block email delivery.
			slog.WarnContext(ctx, "notification: dedup check failed, proceeding to send",
				"module", "notification", "operation", "HandleUserCreated",
				"msg_id", msg.UUID, "err", err)
		} else if exists > 0 {
			slog.InfoContext(ctx, "notification: duplicate message, skipping",
				"module", "notification", "operation", "HandleUserCreated",
				"msg_id", msg.UUID, "event_id", dedupKey)
			return nil
		}
	}

	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, event); err != nil {
		// Template failure is permanent (bad template, not transient infra) — ack to avoid
		// infinite retry loop. Fix the template and redeploy to reprocess.
		slog.ErrorContext(ctx, "notification: failed to render template, acking to avoid retry loop",
			"module", "notification", "operation", "HandleUserCreated",
			"err", err, "msg_id", msg.UUID,
			"error_code", "template_render_failed", "retryable", false)
		return nil
	}

	if err := h.sender.Send(ctx, event.Email, "Welcome!", buf.String()); err != nil {
		// Permanent SMTP errors (5xx) won't be fixed by retrying — ack them.
		if isPermanentSMTPError(err) {
			slog.WarnContext(ctx, "notification: permanent SMTP error, acking",
				"module", "notification", "operation", "HandleUserCreated",
				"err", err, "user_id", event.UserID,
				"error_code", "smtp_permanent", "retryable", false)
			return nil
		}
		slog.ErrorContext(ctx, "notification: failed to send email",
			"module", "notification", "operation", "HandleUserCreated",
			"err", err, "user_id", event.UserID,
			"error_code", "smtp_transient", "retryable", true)
		return err
	}

	// Commit dedup key AFTER successful send — crash before this point allows redelivery.
	if h.rdb != nil {
		// nolint:staticcheck // SetNX works fine despite deprecation warning
		if _, err := h.rdb.SetNX(ctx, "notification:dedup:"+dedupKey, "1", dedupTTL).Result(); err != nil {
			slog.WarnContext(ctx, "notification: failed to set dedup key after send, may get duplicate",
				"module", "notification", "operation", "HandleUserCreated",
				"msg_id", msg.UUID, "err", err)
		}
	}

	slog.InfoContext(ctx, "notification: welcome email sent",
		"module", "notification", "operation", "HandleUserCreated",
		"user_id", event.UserID)
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
</body>
</html>`
