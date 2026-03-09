// Package postgres implements the user repository using pgx and sqlc.
package postgres

import (
	"context"
	"errors"
	"fmt"

	sqlcgen "github.com/gnha/gnha-services/gen/sqlc"
	"github.com/gnha/gnha-services/internal/modules/user/domain"
	sharederr "github.com/gnha/gnha-services/internal/shared/errors"
	"github.com/google/uuid"
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
func NewPgUserRepository(pool *pgxpool.Pool) *PgUserRepository {
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
			return nil, sharederr.ErrNotFound()
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
			return nil, sharederr.ErrNotFound()
		}
		return nil, fmt.Errorf("getting user by email: %w", err)
	}
	return toDomain(row), nil
}

// List returns a paginated list of users.
func (r *PgUserRepository) List(ctx context.Context, limit int, cursor string) (domain.ListResult, error) {
	q := sqlcgen.New(r.pool)

	// Fetch limit+1 to detect whether more pages exist.
	params := sqlcgen.ListUsersParams{Limit: int32(limit + 1)}
	if cursor != "" {
		decoded, err := decodeCursor(cursor)
		if err != nil {
			return domain.ListResult{}, sharederr.New(sharederr.CodeInvalidArgument, "invalid pagination cursor")
		}
		params.CursorCreatedAt = pgtype.Timestamptz{Time: decoded.T, Valid: true}
		params.CursorID = pgtype.UUID{Bytes: decoded.U, Valid: true}
	}

	rows, err := q.ListUsers(ctx, params)
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("listing users: %w", err)
	}

	users := make([]*domain.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, toDomainFromListRow(row))
	}

	hasMore := len(users) > limit
	if hasMore {
		users = users[:limit]
	}

	var nextCursor string
	if hasMore && len(users) > 0 {
		last := users[len(users)-1]
		uid, err := uuid.Parse(string(last.ID()))
		if err != nil {
			return domain.ListResult{}, fmt.Errorf("parsing user ID for cursor: %w", err)
		}
		cursor, err := encodeCursor(last.CreatedAt(), uid)
		if err != nil {
			return domain.ListResult{}, fmt.Errorf("encoding pagination cursor: %w", err)
		}
		nextCursor = cursor
	}

	return domain.ListResult{
		Users:      users,
		NextCursor: nextCursor,
		HasMore:    hasMore,
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
	*user = *toDomainFromCreateRow(row)
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
			return sharederr.ErrNotFound()
		}
		return fmt.Errorf("fetching user for update: %w", err)
	}

	user := toDomain(row)
	if err := fn(user); err != nil {
		// No mutations — skip SQL UPDATE and commit the read-lock transaction.
		if errors.Is(err, sharederr.ErrNoChange()) {
			return tx.Commit(ctx)
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
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrEmailTaken()
		}
		return fmt.Errorf("updating user: %w", err)
	}

	// Overwrite the entity with DB-generated timestamps (e.g. updated_at = NOW()).
	*user = *toDomainFromUpdateRow(updatedRow)

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
			return nil, sharederr.ErrNotFound()
		}
		return nil, fmt.Errorf("soft deleting user: %w", err)
	}
	return toDomainFromSoftDeleteRow(row), nil
}
