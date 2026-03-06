package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
			return nil, sharederr.ErrNotFound
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
			return nil, sharederr.ErrNotFound
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
		if err == nil {
			params.CursorCreatedAt = pgtype.Timestamptz{Time: decoded.T, Valid: true}
			params.CursorID = pgtype.UUID{Bytes: decoded.U, Valid: true}
		}
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
		if uid, err := uuid.Parse(string(last.ID())); err == nil {
			nextCursor = encodeCursor(last.CreatedAt(), uid)
		}
	}

	return domain.ListResult{
		Users:      users,
		Total:      int64(len(users)),
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
	_, err = q.CreateUser(ctx, sqlcgen.CreateUserParams{
		ID:       uid,
		Email:    user.Email(),
		Name:     user.Name(),
		Password: user.Password(),
		Role:     string(user.Role()),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_email_key" {
			return domain.ErrEmailTaken
		}
		return fmt.Errorf("inserting user: %w", err)
	}
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
	defer tx.Rollback(ctx)

	q := sqlcgen.New(tx)
	row, err := q.GetUserByIDForUpdate(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sharederr.ErrNotFound
		}
		return fmt.Errorf("fetching user for update: %w", err)
	}

	user := toDomain(row)
	if err := fn(user); err != nil {
		return err
	}

	name := user.Name()
	role := string(user.Role())
	_, err = q.UpdateUser(ctx, sqlcgen.UpdateUserParams{
		ID:   uid,
		Name: pgtype.Text{String: name, Valid: true},
		Role: pgtype.Text{String: role, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *PgUserRepository) SoftDelete(ctx context.Context, id domain.UserID) error {
	uid, err := parseUserID(id)
	if err != nil {
		return err
	}
	q := sqlcgen.New(r.pool)
	rows, err := q.SoftDeleteUser(ctx, uid)
	if err != nil {
		return fmt.Errorf("soft deleting user: %w", err)
	}
	if rows == 0 {
		return sharederr.ErrNotFound
	}
	return nil
}

// parseUserID safely parses a domain.UserID into a uuid.UUID.
func parseUserID(id domain.UserID) (uuid.UUID, error) {
	uid, err := uuid.Parse(string(id))
	if err != nil {
		return uuid.UUID{}, sharederr.New(sharederr.CodeInvalidArgument, "invalid user ID format")
	}
	return uid, nil
}

// toDomain converts a sqlc User row (with password) to a domain entity.
func toDomain(row sqlcgen.User) *domain.User {
	var deletedAt *time.Time
	if row.DeletedAt.Valid {
		deletedAt = &row.DeletedAt.Time
	}
	return domain.Reconstitute(
		domain.UserID(row.ID.String()),
		row.Email, row.Name, row.Password,
		domain.Role(row.Role),
		row.CreatedAt, row.UpdatedAt, deletedAt,
	)
}

// toDomainFromGetRow converts a GetUserByIDRow (no password) to a domain entity.
func toDomainFromGetRow(row sqlcgen.GetUserByIDRow) *domain.User {
	var deletedAt *time.Time
	if row.DeletedAt.Valid {
		deletedAt = &row.DeletedAt.Time
	}
	return domain.Reconstitute(
		domain.UserID(row.ID.String()),
		row.Email, row.Name, "",
		domain.Role(row.Role),
		row.CreatedAt, row.UpdatedAt, deletedAt,
	)
}

// toDomainFromListRow converts a ListUsersRow (no password) to a domain entity.
func toDomainFromListRow(row sqlcgen.ListUsersRow) *domain.User {
	var deletedAt *time.Time
	if row.DeletedAt.Valid {
		deletedAt = &row.DeletedAt.Time
	}
	return domain.Reconstitute(
		domain.UserID(row.ID.String()),
		row.Email, row.Name, "",
		domain.Role(row.Role),
		row.CreatedAt, row.UpdatedAt, deletedAt,
	)
}

// Cursor helpers for keyset pagination.
type cursorPayload struct {
	T time.Time `json:"t"`
	U uuid.UUID `json:"u"`
}

func encodeCursor(t time.Time, id uuid.UUID) string {
	data, _ := json.Marshal(cursorPayload{T: t, U: id})
	return base64.URLEncoding.EncodeToString(data)
}

func decodeCursor(cursor string) (*cursorPayload, error) {
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, err
	}
	var c cursorPayload
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
