package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/netip"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events/contracts"
	"github.com/google/uuid"
	sqlcgen "github.com/gnha/golang-echo-boilerplate/gen/sqlc"
)

// auditWriter persists audit log entries.
type auditWriter interface {
	CreateAuditLog(ctx context.Context, params sqlcgen.CreateAuditLogParams) error
}

// Handler processes audit-related events.
// Required: writer (audit chain breaks without a working writer)
type Handler struct {
	writer auditWriter
}

// NewHandler constructs the audit handler.
// Panics if writer is nil — a nil writer panics at the first audit event instead of at startup.
func NewHandler(writer auditWriter) *Handler {
	if writer == nil {
		panic("audit.NewHandler: writer must not be nil")
	}
	return &Handler{writer: writer}
}

// parseIPAddress parses a string IP into *netip.Addr for the audit log.
func parseIPAddress(ip string) *netip.Addr {
	if ip == "" {
		return nil
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return nil
	}
	return &addr
}

// resolveMessageID parses the Watermill message UUID as the audit log primary key.
// Falls back to a deterministic UUID derived from the event's EventID (or raw payload)
// so that publisher-side retries with different Watermill UUIDs still collide on
// ON CONFLICT (id) DO NOTHING.
func resolveMessageID(msg *message.Message) uuid.UUID {
	msgID, err := uuid.Parse(msg.UUID)
	if err != nil {
		var baseEvent struct {
			EventID string `json:"event_id"`
		}
		if jsonErr := json.Unmarshal(msg.Payload, &baseEvent); jsonErr == nil && baseEvent.EventID != "" {
			msgID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(baseEvent.EventID))
		} else {
			msgID = uuid.NewSHA1(uuid.NameSpaceOID, msg.Payload)
		}
		slog.WarnContext(msg.Context(), "audit: invalid msg UUID, using event-derived deterministic ID",
			"module", "audit", "operation", "resolveMessageID",
			"msg_uuid", msg.UUID, "derived_id", msgID, "err", err)
	}
	return msgID
}

// parseActorID extracts the actor from the event, falling back to entityID.
func parseActorID(ctx context.Context, actorIDStr string, entityID uuid.UUID) uuid.UUID {
	if actorIDStr == "" {
		return entityID
	}
	parsed, err := uuid.Parse(actorIDStr)
	if err != nil {
		slog.WarnContext(ctx, "audit: invalid actor_id in event, falling back to entity_id",
			"actor_id", actorIDStr, "entity_id", entityID, "err", err)
		return entityID
	}
	return parsed
}

// handleAuditEvent writes a row to the audit log.
// changes is the raw JSON payload from the event message, preserving all fields.
func (h *Handler) handleAuditEvent(msg *message.Message, userID, actorID, ipAddress, action string) error {
	entityID, err := uuid.Parse(userID)
	if err != nil {
		slog.ErrorContext(msg.Context(), "audit: invalid user ID in event",
			"module", "audit", "operation", "handleAuditEvent",
			"user_id", userID, "err", err,
			"error_code", "invalid_user_id", "retryable", false)
		return nil // ack — retrying won't fix bad data
	}

	return h.writer.CreateAuditLog(msg.Context(), sqlcgen.CreateAuditLogParams{
		ID:         resolveMessageID(msg),
		EntityType: "user",
		EntityID:   entityID,
		Action:     action,
		ActorID:    parseActorID(msg.Context(), actorID, entityID),
		Changes:    json.RawMessage(msg.Payload), // raw JSON preserves all event fields
		IpAddress:  parseIPAddress(ipAddress),
		Status:     "success",
	})
}

// HandleUserCreated logs a user creation event to the audit trail.
func (h *Handler) HandleUserCreated(msg *message.Message) error {
	var ev contracts.UserCreatedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "audit: failed to unmarshal user.created event",
			"module", "audit", "operation", "HandleUserCreated",
			"err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	if ev.Version != contracts.UserEventSchemaVersion {
		slog.WarnContext(msg.Context(), "audit: unknown event version, acking to prevent retry loop",
			"module", "audit", "operation", "HandleUserCreated",
			"got_version", ev.Version, "expected_version", contracts.UserEventSchemaVersion)
		return nil // ack — don't retry unknown versions
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.ActorID, ev.IPAddress, "created")
}

// HandleUserUpdated logs a user update event.
func (h *Handler) HandleUserUpdated(msg *message.Message) error {
	var ev contracts.UserUpdatedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "audit: failed to unmarshal user.updated event",
			"module", "audit", "operation", "HandleUserUpdated",
			"err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	if ev.Version != contracts.UserEventSchemaVersion {
		slog.WarnContext(msg.Context(), "audit: unknown event version, acking to prevent retry loop",
			"module", "audit", "operation", "HandleUserUpdated",
			"got_version", ev.Version, "expected_version", contracts.UserEventSchemaVersion)
		return nil // ack — don't retry unknown versions
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.ActorID, ev.IPAddress, "updated")
}

// HandleUserDeleted logs a user deletion event.
func (h *Handler) HandleUserDeleted(msg *message.Message) error {
	var ev contracts.UserDeletedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "audit: failed to unmarshal user.deleted event",
			"module", "audit", "operation", "HandleUserDeleted",
			"err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	if ev.Version != contracts.UserEventSchemaVersion {
		slog.WarnContext(msg.Context(), "audit: unknown event version, acking to prevent retry loop",
			"module", "audit", "operation", "HandleUserDeleted",
			"got_version", ev.Version, "expected_version", contracts.UserEventSchemaVersion)
		return nil // ack — don't retry unknown versions
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.ActorID, ev.IPAddress, "deleted")
}

// HandleUserLoggedIn logs a login event to the audit trail.
func (h *Handler) HandleUserLoggedIn(msg *message.Message) error {
	var ev contracts.UserLoggedInEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "audit: failed to unmarshal user.logged_in event",
			"module", "audit", "operation", "HandleUserLoggedIn",
			"err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	if ev.Version != contracts.UserEventSchemaVersion {
		slog.WarnContext(msg.Context(), "audit: unknown event version, acking to prevent retry loop",
			"module", "audit", "operation", "HandleUserLoggedIn",
			"got_version", ev.Version, "expected_version", contracts.UserEventSchemaVersion)
		return nil // ack — don't retry unknown versions
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.UserID, ev.IPAddress, "logged_in")
}

// HandleUserLoginFailed logs a failed login attempt to the audit trail.
// Uses uuid.Nil as entity_id since the user may not exist; email is captured in changes JSON.
func (h *Handler) HandleUserLoginFailed(msg *message.Message) error {
	var ev contracts.UserLoginFailedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "audit: failed to unmarshal user.login_failed event",
			"module", "audit", "operation", "HandleUserLoginFailed",
			"err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	if ev.Version != contracts.UserEventSchemaVersion {
		slog.WarnContext(msg.Context(), "audit: unknown event version, acking to prevent retry loop",
			"module", "audit", "operation", "HandleUserLoginFailed",
			"got_version", ev.Version, "expected_version", contracts.UserEventSchemaVersion)
		return nil // ack — don't retry unknown versions
	}

	return h.writer.CreateAuditLog(msg.Context(), sqlcgen.CreateAuditLogParams{
		ID:         resolveMessageID(msg),
		EntityType: "auth",
		EntityID:   uuid.Nil,
		Action:     "login_failed",
		ActorID:    uuid.Nil,
		Changes:    json.RawMessage(msg.Payload),
		IpAddress:  parseIPAddress(ev.IPAddress),
		Status:     "failure",
	})
}

// HandleUserLoggedOut logs a logout event to the audit trail.
func (h *Handler) HandleUserLoggedOut(msg *message.Message) error {
	var ev contracts.UserLoggedOutEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "audit: failed to unmarshal user.logged_out event",
			"module", "audit", "operation", "HandleUserLoggedOut",
			"err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	if ev.Version != contracts.UserEventSchemaVersion {
		slog.WarnContext(msg.Context(), "audit: unknown event version, acking to prevent retry loop",
			"module", "audit", "operation", "HandleUserLoggedOut",
			"got_version", ev.Version, "expected_version", contracts.UserEventSchemaVersion)
		return nil // ack — don't retry unknown versions
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.UserID, ev.IPAddress, "logged_out")
}
