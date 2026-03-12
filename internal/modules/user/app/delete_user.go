package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/netutil"
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
		return domain.ErrUserIDRequired()
	}
	user, err := h.repo.SoftDelete(ctx, domain.UserID(id))
	if err != nil {
		return fmt.Errorf("deleting user %s: %w", id, err)
	}

	if !user.IsDeleted() {
		return fmt.Errorf("soft delete returned non-deleted user %s: adapter bug", id)
	}
	deletedAt := *user.DeletedAt()

	if err := h.bus.Publish(ctx, domain.TopicUserDeleted, domain.UserDeletedEvent{
		Version:   1,
		UserID:    id,
		ActorID:   auth.ActorIDFromContext(ctx),
		IPAddress: netutil.GetClientIP(ctx),
		At:        deletedAt,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish user.deleted event",
			"user_id", id, "err", err)
	}

	return nil
}
