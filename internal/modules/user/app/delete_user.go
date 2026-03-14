package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events/contracts"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/netutil"
)

// DeleteUserHandler handles soft-deleting a user.
// Required: repo, bus
type DeleteUserHandler struct {
	repo domain.UserRepository
	bus  events.EventPublisher
}

// NewDeleteUserHandler constructs the handler.
// Panics if any required dependency is nil.
func NewDeleteUserHandler(repo domain.UserRepository, bus events.EventPublisher) *DeleteUserHandler {
	if repo == nil {
		panic("NewDeleteUserHandler: repo must not be nil")
	}
	if bus == nil {
		panic("NewDeleteUserHandler: bus must not be nil")
	}
	return &DeleteUserHandler{repo: repo, bus: bus}
}

// Handle soft-deletes a user by ID.
func (h *DeleteUserHandler) Handle(ctx context.Context, id string) (err error) {
	ctx, span := otel.Tracer("user").Start(ctx, "DeleteUserHandler.Handle")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	if id == "" {
		return domain.ErrUserIDRequired()
	}

	caller := auth.UserFromContext(ctx)
	if caller == nil {
		return sharederr.ErrForbidden()
	}
	if caller.UserID != id && !caller.HasPermission("user:delete") {
		return sharederr.ErrForbidden()
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
		EventID:   uuid.NewString(),
		Version:   contracts.EventSchemaVersion,
		UserID:    id,
		ActorID:   auth.ActorIDFromContext(ctx),
		IPAddress: netutil.GetClientIP(ctx),
		At:        deletedAt,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish user.deleted event",
			"module", "user", "operation", "DeleteUserHandler",
			"user_id", id, "error_code", "event_publish_failed",
			"retryable", true, "err", err)
	}

	return nil
}
