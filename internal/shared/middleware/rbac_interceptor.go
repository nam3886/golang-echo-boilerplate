package middleware

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	"github.com/gnha/gnha-services/internal/shared/auth"
)

// procedurePermissions maps Connect RPC procedure suffixes to required permissions.
var procedurePermissions = map[string]Permission{
	"Create": PermUserWrite,
	"Update": PermUserWrite,
	"Delete": PermUserDelete,
}

// RBACInterceptor checks permissions based on the Connect RPC procedure name.
// Read operations require user:read (enforced at Echo group level).
// Write/delete operations require additional permissions checked here.
func RBACInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			procedure := req.Spec().Procedure // e.g. "/user.v1.UserService/CreateUser"
			requiredPerm := permissionForProcedure(procedure)
			if requiredPerm == "" {
				return next(ctx, req)
			}

			user := auth.UserFromContext(ctx)
			if user == nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, nil)
			}
			if !user.HasPermission(string(requiredPerm)) {
				return nil, connect.NewError(connect.CodePermissionDenied, nil)
			}

			return next(ctx, req)
		}
	}
}

// permissionForProcedure extracts the method name and returns the required permission.
func permissionForProcedure(procedure string) Permission {
	// procedure format: "/package.Service/MethodName"
	idx := strings.LastIndex(procedure, "/")
	if idx < 0 {
		return ""
	}
	method := procedure[idx+1:]
	for prefix, perm := range procedurePermissions {
		if strings.HasPrefix(method, prefix) {
			return perm
		}
	}
	return ""
}
