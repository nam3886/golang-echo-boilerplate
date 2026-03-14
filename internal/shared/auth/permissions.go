package auth

// rolePermissions maps each role string to its granted permissions.
// Must stay in sync with internal/shared/middleware/rbac.go permission constants.
var rolePermissions = map[string][]string{
	"admin":  {"admin:*"},
	"member": {"user:read", "user:write"},
	"viewer": {"user:read"},
}

// PermissionsForRole returns the permissions granted to a given role.
// Returns nil for unrecognized roles (RBAC interceptor will deny access).
func PermissionsForRole(role string) []string {
	perms, ok := rolePermissions[role]
	if !ok {
		return nil
	}
	// Return a copy to prevent caller mutation.
	out := make([]string, len(perms))
	copy(out, perms)
	return out
}
