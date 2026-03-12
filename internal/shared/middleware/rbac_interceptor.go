package middleware

// This file implements a Connect RPC interceptor (not Echo middleware).
// It lives in the middleware package for organizational convenience since
// it shares the RBAC permission constants defined in rbac.go.

import (
	"context"
	"sort"
	"strings"

	"connectrpc.com/connect"
	"github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
)

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
	sort.Strings(prefixes) // deterministic order for debug logging
	return prefixes
}

// RBACInterceptor checks permissions based on the exact Connect RPC procedure path.
// The caller supplies the full procedure→permission map (assembled via fx group).
// All procedures for registered services must be mapped.
// Unmapped procedures under a registered service prefix are denied (fail-closed).
func RBACInterceptor(procedurePerms map[string]Permission) connect.UnaryInterceptorFunc {
	servicePrefixes := buildServicePrefixes(procedurePerms)
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			procedure := req.Spec().Procedure // e.g. "/user.v1.UserService/CreateUser"
			requiredPerm, ok := procedurePerms[procedure]
			if !ok {
				// Deny if procedure belongs to a registered service
				// but has no explicit permission mapping (fail-closed).
				for _, prefix := range servicePrefixes {
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
