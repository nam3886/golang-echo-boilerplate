package app

import (
	"context"

	"github.com/gnha/gnha-services/internal/modules/user/domain"
)

// ListUsersHandler handles listing users with cursor pagination.
type ListUsersHandler struct {
	repo domain.UserRepository
}

// NewListUsersHandler constructs the handler.
func NewListUsersHandler(repo domain.UserRepository) *ListUsersHandler {
	return &ListUsersHandler{repo: repo}
}

// Handle returns a paginated list of users.
func (h *ListUsersHandler) Handle(ctx context.Context, limit int, cursor string) (domain.ListResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	return h.repo.List(ctx, limit, cursor)
}
