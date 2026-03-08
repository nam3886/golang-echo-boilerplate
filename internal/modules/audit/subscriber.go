package audit

import (
	"encoding/json"
	"log/slog"
	"net/netip"

	"github.com/ThreeDotsLabs/watermill/message"
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

// auditEvent is a common interface for all auditable user events.
type auditEvent interface {
	userID() string
	actorID() string
	ipAddress() string
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

// handleAuditEvent is the generic audit handler; unmarshal is done by the caller.
// changes is the raw JSON payload from the event message, preserving all fields.
func (h *Handler) handleAuditEvent(msg *message.Message, ev auditEvent, changes json.RawMessage, action string) error {
	entityID, err := uuid.Parse(ev.userID())
	if err != nil {
		slog.Error("audit: invalid user ID in event", "user_id", ev.userID(), "err", err)
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
		ActorID:    parseActorID(ev.actorID(), entityID),
		Changes:    changes, // raw JSON preserves all event fields (email, name, role, etc.)
		IpAddress:  parseIPAddress(ev.ipAddress()),
	})
}

// HandleUserCreated logs a user creation event to the audit trail.
func (h *Handler) HandleUserCreated(msg *message.Message) error {
	var ev auditPayload
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.Error("audit: failed to unmarshal user.created event", "err", err,
			"payload", string(msg.Payload))
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	return h.handleAuditEvent(msg, ev, json.RawMessage(msg.Payload), "created")
}

// HandleUserUpdated logs a user update event.
func (h *Handler) HandleUserUpdated(msg *message.Message) error {
	var ev auditPayload
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.Error("audit: failed to unmarshal user.updated event", "err", err,
			"payload", string(msg.Payload))
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	return h.handleAuditEvent(msg, ev, json.RawMessage(msg.Payload), "updated")
}

// HandleUserDeleted logs a user deletion event.
func (h *Handler) HandleUserDeleted(msg *message.Message) error {
	var ev auditPayload
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.Error("audit: failed to unmarshal user.deleted event", "err", err,
			"payload", string(msg.Payload))
		return nil // ack — schema mismatch is permanent, retrying won't help
	}
	return h.handleAuditEvent(msg, ev, json.RawMessage(msg.Payload), "deleted")
}
