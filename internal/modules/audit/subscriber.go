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
type Handler struct {
	writer auditWriter
}

// NewHandler constructs the audit handler.
func NewHandler(writer auditWriter) *Handler {
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
			"module", "audit", "user_id", userID, "err", err,
			"error_code", "invalid_user_id", "retryable", false)
		return nil // ack — retrying won't fix bad data
	}

	// Use the Watermill message UUID as the audit log primary key for idempotency.
	// ON CONFLICT (id) DO NOTHING in the query silently deduplicates retries.
	msgID, err := uuid.Parse(msg.UUID)
	if err != nil {
		// Fallback: derive a deterministic UUID from the raw payload so retries of the
		// same message still collide with ON CONFLICT (id) DO NOTHING.
		// uuid.New() (random) would defeat dedup — never use it here.
		msgID = uuid.NewSHA1(uuid.NameSpaceOID, msg.Payload)
		slog.WarnContext(msg.Context(), "audit: invalid msg UUID, using payload-derived deterministic ID",
			"module", "audit", "operation", "InsertAuditLog",
			"msg_uuid", msg.UUID, "derived_id", msgID, "err", err)
	}

	return h.writer.CreateAuditLog(msg.Context(), sqlcgen.CreateAuditLogParams{
		ID:         msgID,
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
			"module", "audit", "err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.ActorID, ev.IPAddress, "created")
}

// HandleUserUpdated logs a user update event.
func (h *Handler) HandleUserUpdated(msg *message.Message) error {
	var ev contracts.UserUpdatedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "audit: failed to unmarshal user.updated event",
			"module", "audit", "err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.ActorID, ev.IPAddress, "updated")
}

// HandleUserDeleted logs a user deletion event.
func (h *Handler) HandleUserDeleted(msg *message.Message) error {
	var ev contracts.UserDeletedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "audit: failed to unmarshal user.deleted event",
			"module", "audit", "err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.ActorID, ev.IPAddress, "deleted")
}

// HandleUserLoggedIn logs a login event to the audit trail.
func (h *Handler) HandleUserLoggedIn(msg *message.Message) error {
	var ev contracts.UserLoggedInEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "audit: failed to unmarshal user.logged_in event",
			"module", "audit", "err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.UserID, ev.IPAddress, "logged_in")
}

// HandleUserLoggedOut logs a logout event to the audit trail.
func (h *Handler) HandleUserLoggedOut(msg *message.Message) error {
	var ev contracts.UserLoggedOutEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.ErrorContext(msg.Context(), "audit: failed to unmarshal user.logged_out event",
			"module", "audit", "err", err, "msg_id", msg.UUID,
			"error_code", "unmarshal_failed", "retryable", false)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.UserID, ev.IPAddress, "logged_out")
}
