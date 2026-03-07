package middleware

import (
	"context"

	"connectrpc.com/connect"
	"github.com/gnha/gnha-services/internal/shared/auth"
)

// procedurePermissions maps exact Connect RPC procedure paths to required permissions.
// Key format: "/package.Service/MethodName" (full procedure path from req.Spec().Procedure).
// Read-only methods are omitted — they are gated at the Echo route group level via RequirePermission.
var procedurePermissions = map[string]Permission{
	"/user.v1.UserService/CreateUser": PermUserWrite,
	"/user.v1.UserService/UpdateUser": PermUserWrite,
	"/user.v1.UserService/DeleteUser": PermUserDelete,
}

// RBACInterceptor checks permissions based on the exact Connect RPC procedure path.
// Read operations require user:read (enforced at Echo group level).
// Write/delete operations require additional permissions checked here.
func RBACInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			procedure := req.Spec().Procedure // e.g. "/user.v1.UserService/CreateUser"
			requiredPerm, ok := procedurePermissions[procedure]
			if !ok {
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
