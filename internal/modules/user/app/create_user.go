package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gnha/gnha-services/internal/modules/user/domain"
	"github.com/gnha/gnha-services/internal/shared/auth"
	sharederr "github.com/gnha/gnha-services/internal/shared/errors"
	"github.com/gnha/gnha-services/internal/shared/events"
	appmw "github.com/gnha/gnha-services/internal/shared/middleware"
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
	bus    *events.EventBus
}

// NewCreateUserHandler constructs the handler.
func NewCreateUserHandler(repo domain.UserRepository, hasher auth.PasswordHasher, bus *events.EventBus) *CreateUserHandler {
	return &CreateUserHandler{repo: repo, hasher: hasher, bus: bus}
}

// Handle creates a new user after checking email uniqueness.
func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCmd) (*domain.User, error) {
	// Check email uniqueness
	existing, err := h.repo.GetByEmail(ctx, cmd.Email)
	if err != nil && !errors.Is(err, sharederr.ErrNotFound) {
		return nil, fmt.Errorf("checking email: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrEmailTaken
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
	var actorID string
	if actor := auth.UserFromContext(ctx); actor != nil {
		actorID = actor.UserID
	}
	if err := h.bus.Publish(ctx, events.TopicUserCreated, events.UserCreatedEvent{
		UserID:    string(user.ID()),
		ActorID:   actorID,
		Email:     user.Email(),
		Name:      user.Name(),
		Role:      string(user.Role()),
		IPAddress: appmw.GetClientIP(ctx),
		At:        time.Now(),
	}); err != nil {
		slog.ErrorContext(ctx, "failed to publish user.created event",
			"user_id", string(user.ID()), "err", err)
	}

	return user, nil
}
