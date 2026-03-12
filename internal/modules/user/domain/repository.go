package domain

import "context"

//go:generate mockgen -source=repository.go -destination=../../../shared/mocks/mock_user_repository.go -package=mocks

// ListResult holds paginated query results.
// Users is always non-nil; empty slice for no results.
type ListResult struct {
	Users []*User
	Total int
}

// TotalPages computes the total number of pages for the given page size.
func (r ListResult) TotalPages(pageSize int) int64 {
	if pageSize <= 0 {
		return 0
	}
	return (int64(r.Total) + int64(pageSize) - 1) / int64(pageSize)
}

// UserRepository is the port for user persistence.
type UserRepository interface {
	GetByID(ctx context.Context, id UserID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context, page, pageSize int) (ListResult, error)
	Create(ctx context.Context, user *User) error
	// Update MUST return nil error when fn returns ErrNoChange (no fields modified).
	Update(ctx context.Context, id UserID, fn func(*User) error) error
	SoftDelete(ctx context.Context, id UserID) (*User, error)
}
