package app

import (
	"context"
	"fmt"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
)

// ListUsersHandler handles listing users with offset pagination.
type ListUsersHandler struct {
	repo domain.UserRepository
}

// NewListUsersHandler constructs the handler.
func NewListUsersHandler(repo domain.UserRepository) *ListUsersHandler {
	return &ListUsersHandler{repo: repo}
}

// Handle returns a paginated list of users.
func (h *ListUsersHandler) Handle(ctx context.Context, page, pageSize int) (domain.ListResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	result, err := h.repo.List(ctx, page, pageSize)
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("listing users: %w", err)
	}
	return result, nil
}
