package audit

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"

	"github.com/ThreeDotsLabs/watermill/message"
	sqlcgen "github.com/gnha/gnha-services/gen/sqlc"
	"github.com/gnha/gnha-services/internal/shared/events"
	"github.com/google/uuid"
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

type createdWrapper struct{ e events.UserCreatedEvent }

func (w createdWrapper) userID() string    { return w.e.UserID }
func (w createdWrapper) actorID() string   { return w.e.ActorID }
func (w createdWrapper) ipAddress() string { return w.e.IPAddress }

type updatedWrapper struct{ e events.UserUpdatedEvent }

func (w updatedWrapper) userID() string    { return w.e.UserID }
func (w updatedWrapper) actorID() string   { return w.e.ActorID }
func (w updatedWrapper) ipAddress() string { return w.e.IPAddress }

type deletedWrapper struct{ e events.UserDeletedEvent }

func (w deletedWrapper) userID() string    { return w.e.UserID }
func (w deletedWrapper) actorID() string   { return w.e.ActorID }
func (w deletedWrapper) ipAddress() string { return w.e.IPAddress }

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
func (h *Handler) handleAuditEvent(msg *message.Message, ev auditEvent, raw any, action string) error {
	entityID, err := uuid.Parse(ev.userID())
	if err != nil {
		slog.Error("audit: invalid user ID in event", "user_id", ev.userID(), "err", err)
		return nil // ack — retrying won't fix bad data
	}

	changes, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("audit: marshaling %s event changes: %w", action, err)
	}

	return h.queries.CreateAuditLog(msg.Context(), sqlcgen.CreateAuditLogParams{
		EntityType: "user",
		EntityID:   entityID,
		Action:     action,
		ActorID:    parseActorID(ev.actorID(), entityID),
		Changes:    changes,
		IpAddress:  parseIPAddress(ev.ipAddress()),
	})
}

// HandleUserCreated logs a user creation event to the audit trail.
func (h *Handler) HandleUserCreated(msg *message.Message) error {
	var ev events.UserCreatedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.Error("audit: failed to unmarshal user.created event", "err", err)
		return err
	}
	return h.handleAuditEvent(msg, createdWrapper{ev}, ev, "created")
}

// HandleUserUpdated logs a user update event.
func (h *Handler) HandleUserUpdated(msg *message.Message) error {
	var ev events.UserUpdatedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.Error("audit: failed to unmarshal user.updated event", "err", err)
		return err
	}
	return h.handleAuditEvent(msg, updatedWrapper{ev}, ev, "updated")
}

// HandleUserDeleted logs a user deletion event.
func (h *Handler) HandleUserDeleted(msg *message.Message) error {
	var ev events.UserDeletedEvent
	if err := json.Unmarshal(msg.Payload, &ev); err != nil {
		slog.Error("audit: failed to unmarshal user.deleted event", "err", err)
		return err
	}
	return h.handleAuditEvent(msg, deletedWrapper{ev}, ev, "deleted")
}
