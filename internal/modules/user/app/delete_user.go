package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/gnha/gnha-services/internal/modules/user/domain"
	"github.com/gnha/gnha-services/internal/shared/auth"
	"github.com/gnha/gnha-services/internal/shared/events"
	"github.com/gnha/gnha-services/internal/shared/netutil"
)

// DeleteUserHandler handles soft-deleting a user.
type DeleteUserHandler struct {
	repo domain.UserRepository
	bus  events.EventPublisher
}

// NewDeleteUserHandler constructs the handler.
func NewDeleteUserHandler(repo domain.UserRepository, bus events.EventPublisher) *DeleteUserHandler {
	return &DeleteUserHandler{repo: repo, bus: bus}
}

// Handle soft-deletes a user by ID.
func (h *DeleteUserHandler) Handle(ctx context.Context, id string) error {
	if err := h.repo.SoftDelete(ctx, domain.UserID(id)); err != nil {
		return err
	}

	var actorID string
	if actor := auth.UserFromContext(ctx); actor != nil {
		actorID = actor.UserID
	}
	if err := h.bus.Publish(ctx, events.TopicUserDeleted, events.UserDeletedEvent{
		UserID:    id,
		ActorID:   actorID,
		IPAddress: netutil.GetClientIP(ctx),
		At:        time.Now(),
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish user.deleted event",
			"user_id", id, "err", err)
	}

	return nil
}
