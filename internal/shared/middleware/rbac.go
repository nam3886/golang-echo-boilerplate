package middleware

// Permission represents an RBAC permission string.
type Permission string

const (
	PermUserRead   Permission = "user:read"
	PermUserWrite  Permission = "user:write"
	PermUserDelete Permission = "user:delete"
	PermAdminAll   Permission = "admin:*"
	// ADD_PERMISSION_HERE — scaffold injects new permissions above this line.
)
