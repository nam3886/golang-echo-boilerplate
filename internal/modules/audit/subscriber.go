package audit

import (
	"encoding/json"
	"log/slog"
	"net/netip"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/gnha/gnha-services/internal/shared/events/contracts"
	"github.com/google/uuid"
	sqlcgen "github.com/gnha/gnha-services/gen/sqlc"
)

// Handler processes audit-related events.
type Handler struct {
	queries *sqlcgen.Queries
}

// NewHandler constructs the audit handler.
func NewHandler(queries *sqlcgen.Queries) *Handler {
	return &Handler{queries: queries}
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
func parseActorID(actorIDStr string, entityID uuid.UUID) uuid.UUID {
	if actorIDStr == "" {
		return entityID
	}
	parsed, err := uuid.Parse(actorIDStr)
	if err != nil {
		slog.Warn("audit: invalid actor_id in event, falling back to entity_id",
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
		slog.Error("audit: invalid user ID in event", "user_id", userID, "err", err)
		return nil // ack — retrying won't fix bad data
	}

	// Use the Watermill message UUID as the audit log primary key for idempotency.
	// ON CONFLICT (id) DO NOTHING in the query silently deduplicates retries.
	msgID, err := uuid.Parse(msg.UUID)
	if err != nil {
		msgID = uuid.New()
	}

	return h.queries.CreateAuditLog(msg.Context(), sqlcgen.CreateAuditLogParams{
		ID:         msgID,
		EntityType: "user",
		EntityID:   entityID,
		Action:     action,
		ActorID:    parseActorID(actorID, entityID),
		Changes:    json.RawMessage(msg.Payload), // raw JSON preserves all event fields
		IpAddress:  parseIPAddress(ipAddress),
	})
}

// HandleUserCreated logs a user creation event to the audit trail.
func (h *Handler) HandleUserCreated(msg *message.Message) error {
	var ev contracts.UserCreatedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.Error("audit: failed to unmarshal user.created event", "err", err, "msg_id", msg.UUID)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.ActorID, ev.IPAddress, "created")
}

// HandleUserUpdated logs a user update event.
func (h *Handler) HandleUserUpdated(msg *message.Message) error {
	var ev contracts.UserUpdatedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.Error("audit: failed to unmarshal user.updated event", "err", err, "msg_id", msg.UUID)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.ActorID, ev.IPAddress, "updated")
}

// HandleUserDeleted logs a user deletion event.
func (h *Handler) HandleUserDeleted(msg *message.Message) error {
	var ev contracts.UserDeletedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.Error("audit: failed to unmarshal user.deleted event", "err", err, "msg_id", msg.UUID)
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	return h.handleAuditEvent(msg, ev.UserID, ev.ActorID, ev.IPAddress, "deleted")
}
