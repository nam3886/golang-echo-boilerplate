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

// UpdateUserCmd holds input for updating a user.
type UpdateUserCmd struct {
	ID   string
	Name *string
	Role *string
}

// UpdateUserHandler handles user updates via closure-based UoW.
type UpdateUserHandler struct {
	repo domain.UserRepository
	bus  events.EventPublisher
}

// NewUpdateUserHandler constructs the handler.
func NewUpdateUserHandler(repo domain.UserRepository, bus events.EventPublisher) *UpdateUserHandler {
	return &UpdateUserHandler{repo: repo, bus: bus}
}

// Handle applies partial updates to a user within a transaction.
func (h *UpdateUserHandler) Handle(ctx context.Context, cmd UpdateUserCmd) (*domain.User, error) {
	var updated *domain.User
	err := h.repo.Update(ctx, domain.UserID(cmd.ID), func(user *domain.User) error {
		if cmd.Name != nil {
			if err := user.ChangeName(*cmd.Name); err != nil {
				return err
			}
		}
		if cmd.Role != nil {
			if err := user.ChangeRole(domain.Role(*cmd.Role)); err != nil {
				return err
			}
		}
		updated = user
		return nil
	})
	if err != nil {
		return nil, err
	}

	var actorID string
	if actor := auth.UserFromContext(ctx); actor != nil {
		actorID = actor.UserID
	}
	if err := h.bus.Publish(ctx, events.TopicUserUpdated, events.UserUpdatedEvent{
		UserID:    cmd.ID,
		ActorID:   actorID,
		IPAddress: netutil.GetClientIP(ctx),
		At:        time.Now(),
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish user.updated event",
			"user_id", cmd.ID, "err", err)
	}

	return updated, nil
}
