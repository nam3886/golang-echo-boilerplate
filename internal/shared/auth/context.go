package auth

import "context"

type authContextKey string

const userContextKey authContextKey = "auth_user"

// AuthUser represents the authenticated user extracted from JWT.
type AuthUser struct {
	UserID      string
	Role        string
	Permissions []string
	TokenID     string // jti — for blacklisting
}

// WithUser injects the authenticated user into the context.
func WithUser(ctx context.Context, claims *TokenClaims) context.Context {
	return context.WithValue(ctx, userContextKey, &AuthUser{
		UserID:      claims.UserID,
		Role:        claims.Role,
		Permissions: claims.Permissions,
		TokenID:     claims.RegisteredClaims.ID,
	})
}

// UserFromContext extracts the authenticated user from context.
func UserFromContext(ctx context.Context) *AuthUser {
	if u, ok := ctx.Value(userContextKey).(*AuthUser); ok {
		return u
	}
	return nil
}

// HasPermission checks if the user has a specific permission.
func (u *AuthUser) HasPermission(perm string) bool {
	for _, p := range u.Permissions {
		if p == perm || p == "admin:*" {
			return true
		}
	}
	// Admin role has all permissions
	return u.Role == "admin"
}
