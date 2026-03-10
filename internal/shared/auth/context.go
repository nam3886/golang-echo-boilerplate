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
		TokenID:     claims.ID,
	})
}

// UserFromContext extracts the authenticated user from context.
func UserFromContext(ctx context.Context) *AuthUser {
	if u, ok := ctx.Value(userContextKey).(*AuthUser); ok {
		return u
	}
	return nil
}

// ActorIDFromContext returns the authenticated user's ID, or empty string if unauthenticated.
func ActorIDFromContext(ctx context.Context) string {
	if u := UserFromContext(ctx); u != nil {
		return u.UserID
	}
	return ""
}

// HasPermission checks if the claims contain the required permission.
// Uses flat matching with a single admin wildcard (admin:*).
// Namespace wildcards (e.g., user:*) are intentionally not supported
// to keep the permission model simple. Expand matching logic here
// if hierarchical permissions become needed.
// Admin users must have "admin:*" in their permissions claim — role alone is not sufficient.
func (u *AuthUser) HasPermission(perm string) bool {
	for _, p := range u.Permissions {
		if p == perm || p == "admin:*" {
			return true
		}
	}
	return false
}
