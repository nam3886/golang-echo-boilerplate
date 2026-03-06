package audit

import (
	"encoding/json"
	"log/slog"
	"net/netip"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	sqlcgen "github.com/gnha/gnha-services/gen/sqlc"
	"github.com/gnha/gnha-services/internal/shared/events"
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

// HandleUserCreated logs a user creation event to the audit trail.
func (h *Handler) HandleUserCreated(msg *message.Message) error {
	var event events.UserCreatedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		slog.Error("audit: failed to unmarshal user.created event", "err", err)
		return err
	}

	entityID, err := uuid.Parse(event.UserID)
	if err != nil {
		slog.Error("audit: invalid user ID in event", "user_id", event.UserID, "err", err)
		return nil // ack — retrying won't fix bad data
	}

	ctx := msg.Context()
	changes, _ := json.Marshal(event)

	return h.queries.CreateAuditLog(ctx, sqlcgen.CreateAuditLogParams{
		EntityType: "user",
		EntityID:   entityID,
		Action:     "created",
		ActorID:    parseActorID(event.ActorID, entityID),
		Changes:    changes,
		IpAddress:  parseIPAddress(event.IPAddress),
	})
}

// HandleUserUpdated logs a user update event.
func (h *Handler) HandleUserUpdated(msg *message.Message) error {
	var event events.UserUpdatedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		slog.Error("audit: failed to unmarshal user.updated event", "err", err)
		return err
	}

	entityID, err := uuid.Parse(event.UserID)
	if err != nil {
		slog.Error("audit: invalid user ID in event", "user_id", event.UserID, "err", err)
		return nil
	}

	ctx := msg.Context()
	changes, _ := json.Marshal(event)

	return h.queries.CreateAuditLog(ctx, sqlcgen.CreateAuditLogParams{
		EntityType: "user",
		EntityID:   entityID,
		Action:     "updated",
		ActorID:    parseActorID(event.ActorID, entityID),
		Changes:    changes,
		IpAddress:  parseIPAddress(event.IPAddress),
	})
}

// HandleUserDeleted logs a user deletion event.
func (h *Handler) HandleUserDeleted(msg *message.Message) error {
	var event events.UserDeletedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		slog.Error("audit: failed to unmarshal user.deleted event", "err", err)
		return err
	}

	entityID, err := uuid.Parse(event.UserID)
	if err != nil {
		slog.Error("audit: invalid user ID in event", "user_id", event.UserID, "err", err)
		return nil
	}

	ctx := msg.Context()
	changes, _ := json.Marshal(event)

	return h.queries.CreateAuditLog(ctx, sqlcgen.CreateAuditLogParams{
		EntityType: "user",
		EntityID:   entityID,
		Action:     "deleted",
		ActorID:    parseActorID(event.ActorID, entityID),
		Changes:    changes,
		IpAddress:  parseIPAddress(event.IPAddress),
	})
}
