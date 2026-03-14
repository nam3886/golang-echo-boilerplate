package app

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
)

const (
	maxPage         = 10000
	defaultPageSize = 20
	maxPageSize     = 100
)

// ListUsersHandler handles listing users with offset pagination.
// Required: repo
type ListUsersHandler struct {
	repo domain.UserRepository
}

// NewListUsersHandler constructs the handler.
// Panics if repo is nil.
func NewListUsersHandler(repo domain.UserRepository) *ListUsersHandler {
	if repo == nil {
		panic("NewListUsersHandler: repo must not be nil")
	}
	return &ListUsersHandler{repo: repo}
}

// Handle returns a paginated list of users.
// ⚠️ pageSize=0 is silently clamped to 20; pageSize>100 is silently clamped to 100.
// ⚠️ page<=0 is silently clamped to 1; page>10000 is silently clamped to 10000.
// The effective pageSize used is returned in ListResult.PageSize.
func (h *ListUsersHandler) Handle(ctx context.Context, page, pageSize int) (_ domain.ListResult, err error) {
	ctx, span := otel.Tracer("user").Start(ctx, "ListUsersHandler.Handle")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	if page <= 0 {
		page = 1
	}
	if page > maxPage {
		page = maxPage
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	result, err := h.repo.List(ctx, page, pageSize)
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("listing users: %w", err)
	}
	result.PageSize = pageSize
	return result, nil
}
