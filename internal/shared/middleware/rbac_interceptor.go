package middleware

// This file implements a Connect RPC interceptor (not Echo middleware).
// It lives in the middleware package for organizational convenience since
// it shares the RBAC permission constants defined in rbac.go.

import (
	"context"
	"strings"

	"connectrpc.com/connect"
	userv1connect "github.com/gnha/golang-echo-boilerplate/gen/proto/user/v1/userv1connect"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
)

// procedurePermissions maps exact Connect RPC procedure paths to required permissions.
// Key format: "/package.Service/MethodName" (full procedure path from req.Spec().Procedure).
// ALL procedures for registered services MUST be listed here (fail-closed).
// Read procedures duplicate the Echo group-level check but ensure fail-closed safety.
var procedurePermissions = map[string]Permission{
	userv1connect.UserServiceGetUserProcedure:    PermUserRead,
	userv1connect.UserServiceListUsersProcedure:  PermUserRead,
	userv1connect.UserServiceCreateUserProcedure: PermUserWrite,
	userv1connect.UserServiceUpdateUserProcedure: PermUserWrite,
	userv1connect.UserServiceDeleteUserProcedure: PermUserDelete,
	// ADD_PROCEDURE_PERMISSION_HERE
}

// registeredServicePrefixes lists Connect service path prefixes with RBAC.
// Any procedure under these prefixes MUST be in procedurePermissions,
// otherwise the request is denied (fail-closed). This ensures new RPC methods
// are protected by default until explicitly mapped.
var registeredServicePrefixes = buildServicePrefixes(procedurePermissions)

func buildServicePrefixes(perms map[string]Permission) []string {
	seen := map[string]bool{}
	for proc := range perms {
		if idx := strings.LastIndex(proc, "/"); idx > 0 {
			prefix := proc[:idx+1]
			seen[prefix] = true
		}
	}
	prefixes := make([]string, 0, len(seen))
	for p := range seen {
		prefixes = append(prefixes, p)
	}
	return prefixes
}

// RBACInterceptor checks permissions based on the exact Connect RPC procedure path.
// All procedures for registered services must be mapped in procedurePermissions.
// Unmapped procedures under a registered service prefix are denied (fail-closed).
func RBACInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			procedure := req.Spec().Procedure // e.g. "/user.v1.UserService/CreateUser"
			requiredPerm, ok := procedurePermissions[procedure]
			if !ok {
				// Deny if procedure belongs to a registered service
				// but has no explicit permission mapping (fail-closed).
				for _, prefix := range registeredServicePrefixes {
					if strings.HasPrefix(procedure, prefix) {
						return nil, connect.NewError(connect.CodePermissionDenied, nil)
					}
				}
				// Unknown service (health, reflection, etc.) — pass through.
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
