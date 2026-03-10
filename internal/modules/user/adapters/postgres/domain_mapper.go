package postgres

import (
	"time"

	sqlcgen "github.com/gnha/gnha-services/gen/sqlc"
	"github.com/gnha/gnha-services/internal/modules/user/domain"
	sharederr "github.com/gnha/gnha-services/internal/shared/errors"
	"github.com/google/uuid"
)

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
// Password is set to "" because this query intentionally excludes it.
// Callers must not use Password() on entities returned by read-only queries.
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

// toDomainFromUpdateRow converts an UpdateUserRow to a domain entity.
// Password is preserved via the pwd parameter since RETURNING excludes it.
func toDomainFromUpdateRow(row sqlcgen.UpdateUserRow, pwd string) *domain.User {
	var deletedAt *time.Time
	if row.DeletedAt.Valid {
		deletedAt = &row.DeletedAt.Time
	}
	return domain.Reconstitute(
		domain.UserID(row.ID.String()),
		row.Email, row.Name, pwd,
		domain.Role(row.Role),
		row.CreatedAt, row.UpdatedAt, deletedAt,
	)
}

// toDomainFromCreateRow converts a CreateUserRow to a domain entity.
// Password is preserved via the pwd parameter since RETURNING excludes it.
func toDomainFromCreateRow(row sqlcgen.CreateUserRow, pwd string) *domain.User {
	var deletedAt *time.Time
	if row.DeletedAt.Valid {
		deletedAt = &row.DeletedAt.Time
	}
	return domain.Reconstitute(
		domain.UserID(row.ID.String()),
		row.Email, row.Name, pwd,
		domain.Role(row.Role),
		row.CreatedAt, row.UpdatedAt, deletedAt,
	)
}

// toDomainFromSoftDeleteRow converts a SoftDeleteUserRow (no password) to a domain entity.
// Password is "" since the RETURNING clause intentionally excludes it.
func toDomainFromSoftDeleteRow(row sqlcgen.SoftDeleteUserRow) *domain.User {
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
