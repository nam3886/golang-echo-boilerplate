package app

import (
	"context"

	"github.com/gnha/gnha-services/internal/modules/user/domain"
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
		return nil, domain.ErrInvalidArgument
	}
	return h.repo.GetByID(ctx, domain.UserID(id))
}
