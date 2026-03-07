package domain

import (
	domainerr "github.com/gnha/gnha-services/internal/shared/errors"
)

// Module-specific domain errors.
var (
	ErrEmailRequired = domainerr.New(domainerr.CodeInvalidArgument, "email is required")
	ErrInvalidEmail  = domainerr.New(domainerr.CodeInvalidArgument, "invalid email format")
	ErrNameRequired  = domainerr.New(domainerr.CodeInvalidArgument, "name is required")
	ErrInvalidRole   = domainerr.New(domainerr.CodeInvalidArgument, "invalid role")
	ErrUserNotFound  = domainerr.New(domainerr.CodeNotFound, "user not found")
	ErrEmailTaken    = domainerr.New(domainerr.CodeAlreadyExists, "email already taken")
)
