package domain

import (
	"net/mail"
	"time"

	"github.com/google/uuid"
)

// UserID is a typed identifier for users.
type UserID string

// Role represents user authorization level.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
)

// IsValid checks if the role is a known value.
func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleMember, RoleViewer:
		return true
	}
	return false
}

// User is the domain entity with encapsulated fields.
type User struct {
	id        UserID
	email     string
	name      string
	password  string // hashed
	role      Role
	createdAt time.Time
	updatedAt time.Time
	deletedAt *time.Time
}

// NewUser creates a validated User entity.
func NewUser(email, name, hashedPassword string, role Role) (*User, error) {
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return nil, ErrInvalidEmail()
	}
	email = addr.Address
	if name == "" {
		return nil, ErrNameRequired()
	}
	if !role.IsValid() {
		return nil, ErrInvalidRole()
	}
	if hashedPassword == "" {
		return nil, ErrPasswordRequired()
	}
	now := time.Now()
	return &User{
		id:        UserID(uuid.NewString()),
		email:     email,
		name:      name,
		password:  hashedPassword,
		role:      role,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// Reconstitute rebuilds a User from persistence data (no validation).
// For persistence adapters ONLY. Do not call from application code.
func Reconstitute(id UserID, email, name, password string, role Role, createdAt, updatedAt time.Time, deletedAt *time.Time) *User {
	return &User{
		id: id, email: email, name: name, password: password, role: role,
		createdAt: createdAt, updatedAt: updatedAt, deletedAt: deletedAt,
	}
}

// ID returns the user identifier.
func (u *User) ID() UserID           { return u.id }

// Email returns the user email address.
func (u *User) Email() string        { return u.email }

// Name returns the user display name.
func (u *User) Name() string         { return u.name }

// Password returns the hashed password.
func (u *User) Password() string     { return u.password }

// Role returns the user role.
func (u *User) Role() Role           { return u.role }

// CreatedAt returns when the user was created.
func (u *User) CreatedAt() time.Time { return u.createdAt }

// UpdatedAt returns when the user was last updated.
func (u *User) UpdatedAt() time.Time { return u.updatedAt }

// DeletedAt returns the soft-delete timestamp, or nil if not deleted.
func (u *User) DeletedAt() *time.Time { return u.deletedAt }

// ChangeName updates the user's name.
// No-op when the new name is identical to the current value.
func (u *User) ChangeName(name string) error {
	if name == "" {
		return ErrNameRequired()
	}
	if name == u.name {
		return nil // no-op
	}
	u.name = name
	u.updatedAt = time.Now()
	return nil
}

// ChangeEmail updates the user's email address.
// Format validation is performed here; uniqueness is enforced at the repository level.
func (u *User) ChangeEmail(email string) error {
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return ErrInvalidEmail()
	}
	if addr.Address == u.email {
		return nil // no-op
	}
	u.email = addr.Address
	u.updatedAt = time.Now()
	return nil
}

// ChangeRole updates the user's role.
// No-op when the new role is identical to the current value.
func (u *User) ChangeRole(role Role) error {
	if !role.IsValid() {
		return ErrInvalidRole()
	}
	if role == u.role {
		return nil // no-op
	}
	u.role = role
	u.updatedAt = time.Now()
	return nil
}
