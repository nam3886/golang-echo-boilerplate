package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/events"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/netutil"

	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
)

// CreateUserCmd holds input for creating a user.
type CreateUserCmd struct {
	Email    string
	Name     string
	Password string
	Role     string
}

// CreateUserHandler handles user creation.
type CreateUserHandler struct {
	repo   domain.UserRepository
	hasher auth.PasswordHasher
	bus    events.EventPublisher
}

// NewCreateUserHandler constructs the handler.
func NewCreateUserHandler(repo domain.UserRepository, hasher auth.PasswordHasher, bus events.EventPublisher) *CreateUserHandler {
	return &CreateUserHandler{repo: repo, hasher: hasher, bus: bus}
}

// Handle creates a new user after checking email uniqueness.
func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCmd) (_ *domain.User, err error) {
	ctx, span := otel.Tracer("user").Start(ctx, "CreateUserHandler.Handle")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	// Only admins can create admin accounts.
	if domain.Role(cmd.Role) == domain.RoleAdmin {
		caller := auth.UserFromContext(ctx)
		if caller == nil || !caller.HasPermission("admin:*") {
			return nil, sharederr.ErrForbidden()
		}
	}

	// Fast-path: check email availability before expensive password hashing.
	// The DB unique constraint (idx_users_email_active) is the authoritative guard against races.
	existing, err := h.repo.GetByEmail(ctx, cmd.Email)
	if err != nil && !errors.Is(err, sharederr.ErrNotFound()) {
		return nil, fmt.Errorf("checking email: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrEmailTaken()
	}

	hashedPwd, err := h.hasher.Hash(cmd.Password)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	user, err := domain.NewUser(cmd.Email, cmd.Name, hashedPwd, domain.Role(cmd.Role))
	if err != nil {
		return nil, err
	}

	if err := h.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	// Publish event after successful DB write
	if err := h.bus.Publish(ctx, domain.TopicUserCreated, domain.UserCreatedEvent{
		Version:   1,
		UserID:    string(user.ID()),
		ActorID:   auth.ActorIDFromContext(ctx),
		Email:     user.Email(),
		Name:      user.Name(),
		Role:      string(user.Role()),
		IPAddress: netutil.GetClientIP(ctx),
		At:        user.CreatedAt(),
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish user.created event",
			"module", "user", "operation", "create",
			"user_id", string(user.ID()), "err", err)
	}

	return user, nil
}
