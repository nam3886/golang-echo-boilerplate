package adapters

import (
	"context"
	"errors"
	"fmt"

	"github.com/gnha/golang-echo-boilerplate/internal/modules/user/domain"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
)

// rolePermissions maps each role to its granted permissions.
// Must stay in sync with internal/shared/middleware/rbac.go permission constants.
var rolePermissions = map[domain.Role][]string{
	domain.RoleAdmin:  {"admin:*"},
	domain.RoleMember: {"user:read", "user:write"},
	domain.RoleViewer: {"user:read"},
}

// PermissionsForRole returns the permissions granted to a given role.
// Exported for use by the auth module's login handler.
func PermissionsForRole(role string) []string {
	perms, ok := rolePermissions[domain.Role(role)]
	if !ok {
		return nil
	}
	// Return a copy to prevent caller mutation.
	out := make([]string, len(perms))
	copy(out, perms)
	return out
}

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
		if errors.Is(err, sharederr.ErrNotFound()) {
			return "", "", "", sharederr.ErrNotFound()
		}
		return "", "", "", fmt.Errorf("credential lookup: %w", err)
	}
	return string(user.ID()), user.Password(), string(user.Role()), nil
}
