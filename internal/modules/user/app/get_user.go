package app

import (
	"context"
	"fmt"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
)

// GetUserHandler handles fetching a single user.
type GetUserHandler struct {
	repo domain.UserRepository
}

// NewGetUserHandler constructs the handler.
func NewGetUserHandler(repo domain.UserRepository) *GetUserHandler {
	return &GetUserHandler{repo: repo}
}

// Handle returns a user by ID.
func (h *GetUserHandler) Handle(ctx context.Context, id string) (*domain.User, error) {
	if id == "" {
		return nil, domain.ErrUserIDRequired()
	}
	user, err := h.repo.GetByID(ctx, domain.UserID(id))
	if err != nil {
		return nil, fmt.Errorf("getting user %s: %w", id, err)
	}
	return user, nil
}
