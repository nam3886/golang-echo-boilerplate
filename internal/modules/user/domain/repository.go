package domain

import "context"

//go:generate mockgen -source=repository.go -destination=../../../shared/mocks/mock_user_repository.go -package=mocks

// ListResult holds the paginated result from a List query.
type ListResult struct {
	Users      []*User
	Total      int64
	NextCursor string
	HasMore    bool
}

// UserRepository is the port for user persistence.
type UserRepository interface {
	GetByID(ctx context.Context, id UserID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context, limit int, cursor string) (ListResult, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, id UserID, fn func(*User) error) error
	SoftDelete(ctx context.Context, id UserID) error
}
