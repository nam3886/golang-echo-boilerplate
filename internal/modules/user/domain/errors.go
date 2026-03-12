package domain

import (
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
)

// Module-specific domain errors — constructor functions return fresh instances
// to prevent data races when errors are wrapped concurrently.

// ErrInvalidEmail indicates the email format is invalid.
func ErrInvalidEmail() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "invalid email format")
}

// ErrNameRequired indicates the name field is missing.
func ErrNameRequired() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "name is required")
}

// ErrNameTooLong indicates the name exceeds the maximum allowed length.
func ErrNameTooLong() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "name must be 255 characters or less")
}

// ErrInvalidRole indicates the role value is not recognized.
func ErrInvalidRole() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "invalid role")
}

// ErrPasswordRequired indicates the hashed password is missing.
func ErrPasswordRequired() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeInvalidArgument, "hashed password is required")
}

// ErrUserNotFound indicates the requested user does not exist.
func ErrUserNotFound() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeNotFound, "user not found")
}

// ErrEmailTaken indicates the email is already in use.
func ErrEmailTaken() *sharederr.DomainError {
	return sharederr.New(sharederr.CodeAlreadyExists, "email already taken")
}
