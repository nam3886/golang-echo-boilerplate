package postgres

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	sqlcgen "github.com/gnha/golang-echo-boilerplate/gen/sqlc"
	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	"github.com/google/uuid"
)

// nullTimeToPtr converts a pgtype.Timestamptz to *time.Time.
func nullTimeToPtr(nt pgtype.Timestamptz) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}

// parseUserID safely parses a domain.UserID into a uuid.UUID.
func parseUserID(id domain.UserID) (uuid.UUID, error) {
	uid, err := uuid.Parse(string(id))
	if err != nil {
		return uuid.UUID{}, domain.ErrInvalidUserID()
	}
	return uid, nil
}

// toDomain converts a sqlc User row (with password) to a domain entity.
func toDomain(row sqlcgen.User) *domain.User {
	return domain.Reconstitute(
		domain.UserID(row.ID.String()),
		row.Email, row.Name, row.Password,
		domain.Role(row.Role),
		row.CreatedAt, row.UpdatedAt, nullTimeToPtr(row.DeletedAt),
	)
}

// toDomainFromGetRow converts a GetUserByIDRow (no password) to a domain entity.
// Password is set to "" because read-only queries intentionally exclude it for security.
//
// ⚠️ WARNING: Do NOT call Password() on entities returned by this mapper — it will
// return an empty string. credential_adapter.go has an explicit guard for this.
// Any new caller that needs the password MUST use GetByEmail (toDomain, which includes password).
func toDomainFromGetRow(row sqlcgen.GetUserByIDRow) *domain.User {
	return domain.Reconstitute(
		domain.UserID(row.ID.String()),
		row.Email, row.Name, "",
		domain.Role(row.Role),
		row.CreatedAt, row.UpdatedAt, nullTimeToPtr(row.DeletedAt),
	)
}

// toDomainFromListWithTotalRow converts a ListUsersWithTotalRow (no password) to a domain entity.
func toDomainFromListWithTotalRow(row sqlcgen.ListUsersWithTotalRow) *domain.User {
	return domain.Reconstitute(
		domain.UserID(row.ID.String()),
		row.Email, row.Name, "",
		domain.Role(row.Role),
		row.CreatedAt, row.UpdatedAt, nullTimeToPtr(row.DeletedAt),
	)
}

// toDomainFromUpdateRow converts an UpdateUserRow to a domain entity.
// Password is preserved via the pwd parameter since RETURNING excludes it.
func toDomainFromUpdateRow(row sqlcgen.UpdateUserRow, pwd string) *domain.User {
	return domain.Reconstitute(
		domain.UserID(row.ID.String()),
		row.Email, row.Name, pwd,
		domain.Role(row.Role),
		row.CreatedAt, row.UpdatedAt, nullTimeToPtr(row.DeletedAt),
	)
}

// toDomainFromCreateRow converts a CreateUserRow to a domain entity.
// Password is preserved via the pwd parameter since RETURNING excludes it.
func toDomainFromCreateRow(row sqlcgen.CreateUserRow, pwd string) *domain.User {
	return domain.Reconstitute(
		domain.UserID(row.ID.String()),
		row.Email, row.Name, pwd,
		domain.Role(row.Role),
		row.CreatedAt, row.UpdatedAt, nullTimeToPtr(row.DeletedAt),
	)
}

// toDomainFromSoftDeleteRow converts a SoftDeleteUserRow (no password) to a domain entity.
// Password is "" since the RETURNING clause intentionally excludes it.
func toDomainFromSoftDeleteRow(row sqlcgen.SoftDeleteUserRow) *domain.User {
	return domain.Reconstitute(
		domain.UserID(row.ID.String()),
		row.Email, row.Name, "",
		domain.Role(row.Role),
		row.CreatedAt, row.UpdatedAt, nullTimeToPtr(row.DeletedAt),
	)
}
