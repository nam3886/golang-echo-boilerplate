// Package postgres implements the user repository using pgx and sqlc.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"

	sqlcgen "github.com/gnha/golang-echo-boilerplate/gen/sqlc"
	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgUserRepository implements domain.UserRepository using pgx + sqlc.
type PgUserRepository struct {
	pool *pgxpool.Pool
}

// NewPgUserRepository constructs the repository.
// Panics if pool is nil — a nil pool produces a valid-looking struct that
// crashes at the first DB call instead of at startup.
func NewPgUserRepository(pool *pgxpool.Pool) *PgUserRepository {
	if pool == nil {
		panic("NewPgUserRepository: pool must not be nil")
	}
	return &PgUserRepository{pool: pool}
}

func (r *PgUserRepository) GetByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	uid, err := parseUserID(id)
	if err != nil {
		return nil, err
	}
	q := sqlcgen.New(r.pool)
	row, err := q.GetUserByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound()
		}
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return toDomainFromGetRow(row), nil
}

func (r *PgUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	q := sqlcgen.New(r.pool)
	row, err := q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound()
		}
		return nil, fmt.Errorf("getting user by email: %w", err)
	}
	return toDomain(row), nil
}

// List returns a paginated list of users using a window function for total count.
func (r *PgUserRepository) List(ctx context.Context, page, pageSize int) (domain.ListResult, error) {
	q := sqlcgen.New(r.pool)

	offset := (page - 1) * pageSize
	if offset > math.MaxInt32 || pageSize > math.MaxInt32 {
		return domain.ListResult{}, domain.ErrInvalidPagination()
	}
	rows, err := q.ListUsersWithTotal(ctx, sqlcgen.ListUsersWithTotalParams{
		Limit:  int32(pageSize),
		Offset: int32(offset),
	})
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("listing users: %w", err)
	}

	var total int64
	users := make([]*domain.User, 0, len(rows))
	for _, row := range rows {
		total = row.TotalCount
		users = append(users, toDomainFromListWithTotalRow(row))
	}

	return domain.ListResult{
		Users: users,
		Total: int(total),
	}, nil
}

func (r *PgUserRepository) Create(ctx context.Context, user *domain.User) error {
	uid, err := parseUserID(user.ID())
	if err != nil {
		return err
	}
	q := sqlcgen.New(r.pool)
	row, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
		ID:       uid,
		Email:    user.Email(),
		Name:     user.Name(),
		Password: user.Password(),
		Role:     string(user.Role()),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_users_email_active" {
			return domain.ErrEmailTaken()
		}
		return fmt.Errorf("inserting user: %w", err)
	}
	// Overwrite entity with DB-authoritative timestamps (created_at, updated_at).
	// Password is passed through since RETURNING excludes it for security.
	*user = *toDomainFromCreateRow(row, user.Password())
	return nil
}

func (r *PgUserRepository) Update(ctx context.Context, id domain.UserID, fn func(*domain.User) error) error {
	uid, err := parseUserID(id)
	if err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := sqlcgen.New(tx)
	row, err := q.GetUserByIDForUpdate(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrUserNotFound()
		}
		return fmt.Errorf("fetching user for update: %w", err)
	}

	user := toDomain(row)
	if err := fn(user); err != nil {
		// No mutations — let deferred Rollback release the FOR UPDATE lock.
		// Do NOT Commit: committing a no-op update holds the row lock through an
		// unnecessary round-trip and obscures intent.
		if errors.Is(err, sharederr.ErrNoChange()) {
			return nil
		}
		return err
	}

	name := user.Name()
	role := string(user.Role())
	email := user.Email()
	updatedRow, err := q.UpdateUser(ctx, sqlcgen.UpdateUserParams{
		ID:    uid,
		Name:  pgtype.Text{String: name, Valid: true},
		Role:  pgtype.Text{String: role, Valid: true},
		Email: pgtype.Text{String: email, Valid: true},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_users_email_active" {
			return domain.ErrEmailTaken()
		}
		return fmt.Errorf("updating user: %w", err)
	}

	// Overwrite the entity with DB-generated timestamps (e.g. updated_at = NOW()).
	// Password is passed through since RETURNING excludes it for security.
	*user = *toDomainFromUpdateRow(updatedRow, user.Password())

	return tx.Commit(ctx)
}

// SoftDelete atomically soft-deletes a user in a single UPDATE … RETURNING query,
// eliminating the previous GET-then-DELETE TOCTOU race and using DB-authoritative timestamps.
func (r *PgUserRepository) SoftDelete(ctx context.Context, id domain.UserID) (*domain.User, error) {
	uid, err := parseUserID(id)
	if err != nil {
		return nil, err
	}
	q := sqlcgen.New(r.pool)
	row, err := q.SoftDeleteUser(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound()
		}
		return nil, fmt.Errorf("soft deleting user: %w", err)
	}
	return toDomainFromSoftDeleteRow(row), nil
}
