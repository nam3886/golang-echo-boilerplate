package domain

import (
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
)

// Module-specific domain errors — constructor functions return fresh instances
// to prevent data races when errors are wrapped concurrently.

// ErrUserIDRequired indicates a user ID was not provided.
func ErrUserIDRequired() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "user.id_required", "user ID is required")
}

// ErrInvalidEmail indicates the email format is invalid.
func ErrInvalidEmail() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "user.invalid_email", "invalid email format")
}

// ErrNameRequired indicates the name field is missing.
func ErrNameRequired() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "user.name_required", "name is required")
}

// ErrNameTooLong indicates the name exceeds the maximum allowed length.
func ErrNameTooLong() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "user.name_too_long", "name must be 255 characters or less")
}

// ErrInvalidRole indicates the role value is not recognized.
func ErrInvalidRole() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "user.invalid_role", "invalid role")
}

// ErrPasswordRequired indicates the hashed password is missing.
func ErrPasswordRequired() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "user.password_required", "hashed password is required")
}

// ErrInvalidHashedPassword indicates the password is not a valid argon2id hash.
// This catches wiring bugs where plaintext leaks through to the domain layer.
func ErrInvalidHashedPassword() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "user.invalid_hashed_password", "password must be a valid argon2id hash")
}

// ErrUserNotFound indicates the requested user does not exist.
func ErrUserNotFound() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeNotFound, "user.not_found", "user not found")
}

// ErrEmailTaken indicates the email is already in use.
func ErrEmailTaken() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeAlreadyExists, "user.email_taken", "email already taken")
}

// ErrInvalidUserID indicates the user ID is not a valid UUID.
func ErrInvalidUserID() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "user.invalid_id", "invalid user ID format")
}

// ErrInvalidPagination indicates the pagination values would overflow int32.
func ErrInvalidPagination() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "user.invalid_pagination", "pagination values too large")
}
