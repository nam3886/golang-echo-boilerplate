package app

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
)

// GetUserHandler handles fetching a single user.
// Required: repo
type GetUserHandler struct {
	repo domain.UserRepository
}

// NewGetUserHandler constructs the handler.
func NewGetUserHandler(repo domain.UserRepository) *GetUserHandler {
	return &GetUserHandler{repo: repo}
}

// Handle returns a user by ID.
func (h *GetUserHandler) Handle(ctx context.Context, id string) (_ *domain.User, err error) {
	ctx, span := otel.Tracer("user").Start(ctx, "GetUserHandler.Handle")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	if id == "" {
		return nil, domain.ErrUserIDRequired()
	}
	user, err := h.repo.GetByID(ctx, domain.UserID(id))
	if err != nil {
		return nil, fmt.Errorf("getting user %s: %w", id, err)
	}
	return user, nil
}
