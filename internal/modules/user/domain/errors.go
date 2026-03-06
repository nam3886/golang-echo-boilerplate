package domain

import (
	sharederr "github.com/gnha/gnha-services/internal/shared/errors"
)

// Module-specific domain errors.
var (
	ErrEmailRequired = sharederr.New(sharederr.CodeInvalidArgument, "email is required")
	ErrNameRequired  = sharederr.New(sharederr.CodeInvalidArgument, "name is required")
	ErrInvalidRole   = sharederr.New(sharederr.CodeInvalidArgument, "invalid role")
	ErrUserNotFound  = sharederr.New(sharederr.CodeNotFound, "user not found")
	ErrEmailTaken    = sharederr.New(sharederr.CodeAlreadyExists, "email already taken")
)
