package app

import (
	"context"
	"log/slog"

	"github.com/gnha/gnha-services/internal/modules/user/domain"
	"github.com/gnha/gnha-services/internal/shared/auth"
	"github.com/gnha/gnha-services/internal/shared/events"
	"github.com/gnha/gnha-services/internal/shared/netutil"
)

// UpdateUserCmd holds input for updating a user.
type UpdateUserCmd struct {
	ID    string
	Name  *string
	Role  *string
	Email *string
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
	if cmd.ID == "" {
		return nil, domain.ErrInvalidArgument()
	}
	// Skip DB lock entirely when no fields are provided.
	if cmd.Name == nil && cmd.Role == nil && cmd.Email == nil {
		user, err := h.repo.GetByID(ctx, domain.UserID(cmd.ID))
		if err != nil {
			return nil, err
		}
		return user, nil
	}
	// Email uniqueness is enforced by the DB unique index (idx_users_email_active).
	// Unlike CreateUser, no pre-check is done here because the FOR UPDATE lock
	// serializes concurrent updates to the same user row.
	var updated *domain.User
	var mutated bool
	err := h.repo.Update(ctx, domain.UserID(cmd.ID), func(user *domain.User) error {
		if cmd.Email != nil && *cmd.Email != user.Email() {
			if err := user.ChangeEmail(*cmd.Email); err != nil {
				return err
			}
			mutated = true
		}
		if cmd.Name != nil && *cmd.Name != user.Name() {
			if err := user.ChangeName(*cmd.Name); err != nil {
				return err
			}
			mutated = true
		}
		if cmd.Role != nil && *cmd.Role != string(user.Role()) {
			if err := user.ChangeRole(domain.Role(*cmd.Role)); err != nil {
				return err
			}
			mutated = true
		}
		updated = user
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Skip event if nothing was actually changed.
	if !mutated {
		return updated, nil
	}

	var actorID string
	if actor := auth.UserFromContext(ctx); actor != nil {
		actorID = actor.UserID
	}
	if err := h.bus.Publish(ctx, domain.TopicUserUpdated, domain.UserUpdatedEvent{
		UserID:    cmd.ID,
		ActorID:   actorID,
		Name:      updated.Name(),
		Email:     updated.Email(),
		Role:      string(updated.Role()),
		IPAddress: netutil.GetClientIP(ctx),
		At:        updated.UpdatedAt(),
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish user.updated event",
			"user_id", cmd.ID, "err", err)
	}

	return updated, nil
}
