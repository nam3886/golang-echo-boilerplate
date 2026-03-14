package domain

import "context"

//go:generate mockgen -source=repository.go -destination=../../../shared/mocks/mock_user_repository.go -package=mocks

// ListResult holds paginated query results.
// Users is always non-nil; empty slice for no results.
// PageSize carries the effective (clamped) page size used for the query.
type ListResult struct {
	Users    []*User
	Total    int
	PageSize int
}

// TotalPages computes the total number of pages for the given page size.
func (r ListResult) TotalPages(pageSize int) int {
	if pageSize <= 0 {
		return 0
	}
	return (r.Total + pageSize - 1) / pageSize
}

// UserRepository is the port for user persistence.
//
// Contracts:
//   - GetByID, GetByEmail: returns ErrUserNotFound if missing; retryable
//   - Create: NOT idempotent; returns ErrEmailTaken on duplicate (DB idx_users_email_active)
//   - Update: TOCTOU handled via FOR UPDATE lock; concurrent updates serialized at DB level
//   - SoftDelete: atomic UPDATE RETURNING; returns deleted snapshot
//   - List: always retryable; returns empty slice (never nil) for no results
type UserRepository interface {
	// GetByID returns the active (non-deleted) user with the given ID.
	// Returns ErrUserNotFound if no matching active user exists.
	GetByID(ctx context.Context, id UserID) (*User, error)

	// GetByEmail returns the active user matching the given email address.
	// Returns ErrUserNotFound if no matching active user exists.
	GetByEmail(ctx context.Context, email string) (*User, error)

	// List returns a paginated slice of active users ordered by created_at DESC.
	// page is 1-based; pageSize must be > 0.
	List(ctx context.Context, page, pageSize int) (ListResult, error)

	// Create persists a new user. Returns ErrEmailTaken if the email already exists
	// among active users (enforced by the DB unique index idx_users_email_active).
	Create(ctx context.Context, user *User) error

	// Update fetches the user by id under a FOR UPDATE lock, calls fn with the
	// current state, then persists the result. The fn callback may mutate the user
	// or return sharederr.ErrNoChange() to skip the SQL UPDATE.
	// MUST return nil when fn returns ErrNoChange (no fields modified).
	// Returns ErrUserNotFound if no active user exists with the given ID.
	//
	// ⚠️ WARNING: fn may be retried on serialization failure. The closure MUST be
	// idempotent and reset any accumulated state (e.g., changedFields slice) at
	// the start of each invocation. See update_user.go for the reference pattern.
	Update(ctx context.Context, id UserID, fn func(*User) error) error

	// SoftDelete marks the user as deleted by setting deleted_at.
	// Returns the deleted user snapshot. Returns ErrUserNotFound if no active user
	// exists with the given ID (already deleted users are not found).
	SoftDelete(ctx context.Context, id UserID) (*User, error)
}
