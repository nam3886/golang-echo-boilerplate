package app

import (
	"context"

	"github.com/gnha/gnha-services/internal/modules/user/domain"
)

// ListUsersResult holds the paginated list result.
type ListUsersResult struct {
	Users      []*domain.User
	NextCursor string
	HasMore    bool
}

// ListUsersHandler handles listing users with cursor pagination.
type ListUsersHandler struct {
	repo domain.UserRepository
}

// NewListUsersHandler constructs the handler.
func NewListUsersHandler(repo domain.UserRepository) *ListUsersHandler {
	return &ListUsersHandler{repo: repo}
}

// Handle returns a paginated list of users.
func (h *ListUsersHandler) Handle(ctx context.Context, limit int, cursor string) (*ListUsersResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	result, err := h.repo.List(ctx, limit, cursor)
	if err != nil {
		return nil, err
	}

	return &ListUsersResult{
		Users:      result.Users,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}, nil
}
