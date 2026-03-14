package adapters

import (
	"context"
	"errors"
	"fmt"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
)

// CredentialAdapter implements auth.CredentialLookup using the user repository.
type CredentialAdapter struct {
	repo domain.UserRepository
}

// NewCredentialAdapter constructs the adapter.
// Panics if repo is nil.
func NewCredentialAdapter(repo domain.UserRepository) *CredentialAdapter {
	if repo == nil {
		panic("NewCredentialAdapter: repo must not be nil")
	}
	return &CredentialAdapter{repo: repo}
}

// GetByEmail retrieves credential data for the given email address.
// Returns sharederr.ErrNotFound if no active user exists.
func (a *CredentialAdapter) GetByEmail(ctx context.Context, email string) (userID, hashedPassword, role string, err error) {
	user, err := a.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound()) {
			return "", "", "", sharederr.ErrNotFound()
		}
		return "", "", "", fmt.Errorf("credential lookup: %w", err)
	}
	return string(user.ID()), user.Password(), string(user.Role()), nil
}
