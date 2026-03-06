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
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, ErrInvalidEmail
	}
	if name == "" {
		return nil, ErrNameRequired
	}
	if !role.IsValid() {
		return nil, ErrInvalidRole
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
func Reconstitute(id UserID, email, name, password string, role Role, createdAt, updatedAt time.Time, deletedAt *time.Time) *User {
	return &User{
		id: id, email: email, name: name, password: password, role: role,
		createdAt: createdAt, updatedAt: updatedAt, deletedAt: deletedAt,
	}
}

// Getters
func (u *User) ID() UserID        { return u.id }
func (u *User) Email() string     { return u.email }
func (u *User) Name() string      { return u.name }
func (u *User) Password() string  { return u.password }
func (u *User) Role() Role        { return u.role }
func (u *User) CreatedAt() time.Time { return u.createdAt }
func (u *User) UpdatedAt() time.Time { return u.updatedAt }
func (u *User) DeletedAt() *time.Time { return u.deletedAt }

// ChangeName updates the user's name.
func (u *User) ChangeName(name string) error {
	if name == "" {
		return ErrNameRequired
	}
	u.name = name
	u.updatedAt = time.Now()
	return nil
}

// ChangeRole updates the user's role.
func (u *User) ChangeRole(role Role) error {
	if !role.IsValid() {
		return ErrInvalidRole
	}
	u.role = role
	u.updatedAt = time.Now()
	return nil
}
