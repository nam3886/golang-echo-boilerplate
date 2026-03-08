package domain

import (
	sharederr "github.com/gnha/gnha-services/internal/shared/errors"
)

// Module-specific domain errors — constructor functions return fresh instances
// to prevent data races when errors are wrapped concurrently.

// ErrEmailRequired indicates the email field is missing.
func ErrEmailRequired() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "email is required")
}

// ErrInvalidEmail indicates the email format is invalid.
func ErrInvalidEmail() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "invalid email format")
}

// ErrNameRequired indicates the name field is missing.
func ErrNameRequired() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "name is required")
}

// ErrInvalidRole indicates the role value is not recognized.
func ErrInvalidRole() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "invalid role")
}

// ErrPasswordRequired indicates the hashed password is missing.
func ErrPasswordRequired() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "hashed password is required")
}

// ErrInvalidArgument indicates a generic invalid argument.
func ErrInvalidArgument() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "invalid argument")
}

// ErrUserNotFound indicates the requested user does not exist.
func ErrUserNotFound() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeNotFound, "user not found")
}

// ErrEmailTaken indicates the email is already in use.
func ErrEmailTaken() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeAlreadyExists, "email already taken")
}
