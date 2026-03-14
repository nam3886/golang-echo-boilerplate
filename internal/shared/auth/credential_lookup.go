package auth

import "context"

// CredentialLookup retrieves credential data for authentication without cross-module imports.
// The user module provides a thin adapter implementing this interface.
type CredentialLookup interface {
	// GetByEmail returns the userID, hashedPassword, and role for the given email.
	// Returns sharederr.ErrNotFound if no active user exists with this email.
	GetByEmail(ctx context.Context, email string) (userID, hashedPassword, role string, err error)
}
