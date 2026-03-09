package app

import (
	"context"
	"log/slog"

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
	if id == "" {
		return domain.ErrInvalidArgument()
	}
	user, err := h.repo.SoftDelete(ctx, domain.UserID(id))
	if err != nil {
		return err
	}

	if err := h.bus.Publish(ctx, domain.TopicUserDeleted, domain.UserDeletedEvent{
		UserID:    id,
		ActorID:   auth.ActorIDFromContext(ctx),
		IPAddress: netutil.GetClientIP(ctx),
		At:        *user.DeletedAt(), // DB-authoritative deletion timestamp
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish user.deleted event",
			"user_id", id, "err", err)
	}

	return nil
}
