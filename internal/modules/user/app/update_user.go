package app

import (
	"context"
	"log/slog"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/netutil"
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
// If no fields are provided (all nil), it returns the current user state
// without acquiring a FOR UPDATE lock. This is a deliberate optimization:
// the caller (gRPC handler) may send updates with no actual changes.
func (h *UpdateUserHandler) Handle(ctx context.Context, cmd UpdateUserCmd) (*domain.User, error) {
	if cmd.ID == "" {
		return nil, domain.ErrUserIDRequired()
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
	var changedFields []string
	err := h.repo.Update(ctx, domain.UserID(cmd.ID), func(user *domain.User) error {
		changedFields = nil // reset on retry
		// Pre-check avoids false-positive mutation tracking.
		// Entity methods also validate, but we need to know IF a change happened
		// to skip unnecessary DB writes and event publishing.
		if cmd.Email != nil && *cmd.Email != user.Email() {
			if err := user.ChangeEmail(*cmd.Email); err != nil {
				return err
			}
			changedFields = append(changedFields, "email")
		}
		if cmd.Name != nil && *cmd.Name != user.Name() {
			if err := user.ChangeName(*cmd.Name); err != nil {
				return err
			}
			changedFields = append(changedFields, "name")
		}
		if cmd.Role != nil && *cmd.Role != string(user.Role()) {
			if err := user.ChangeRole(domain.Role(*cmd.Role)); err != nil {
				return err
			}
			changedFields = append(changedFields, "role")
		}
		updated = user
		if len(changedFields) == 0 {
			return sharederr.ErrNoChange()
		}
		return nil
	})
	// err==nil && len(changedFields)==0: repo committed read-only tx (ErrNoChange), no SQL UPDATE issued.
	if err != nil {
		return nil, err
	}

	// Skip event if nothing was actually changed.
	if len(changedFields) == 0 {
		return updated, nil
	}

	if err := h.bus.Publish(ctx, domain.TopicUserUpdated, domain.UserUpdatedEvent{
		Version:       1,
		UserID:        string(updated.ID()),
		ActorID:       auth.ActorIDFromContext(ctx),
		Name:          updated.Name(),
		Email:         updated.Email(),
		Role:          string(updated.Role()),
		ChangedFields: changedFields,
		IPAddress:     netutil.GetClientIP(ctx),
		At:            updated.UpdatedAt(),
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish user.updated event",
			"user_id", cmd.ID, "err", err)
	}

	return updated, nil
}
