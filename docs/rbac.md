# RBAC (Role-Based Access Control)

## Permission Type

Defined in `internal/shared/middleware/rbac.go`.

```
Permission = string (typed alias)
```

Built-in permissions:
- `user:read`   — list/get users
- `user:write`  — create/update users
- `user:delete` — delete users
- `admin:*`     — wildcard; grants any permission check

## Enforcement Architecture

RBAC is enforced **exclusively** at the Connect RPC interceptor level.
The Echo route group applies only `Auth` (JWT validation); permission checks happen inside Connect.

### Echo Route Group: Auth Only

`Auth(cfg, rdb)` — validates the JWT/API key and injects `AuthUser` into the request context.
Returns 401 if the token is missing, expired, or invalid.

`RequirePermission` and `RequireRole` exist in `internal/shared/middleware/rbac.go` as optional
Echo middleware for non-Connect endpoints, but are **not** applied to the Connect RPC route groups.

Source: `internal/shared/middleware/rbac.go`

### Connect RPC Interceptor: Permission Enforcement

`RBACInterceptor()` — a `connect.UnaryInterceptorFunc` that maps exact procedure paths
to required permissions via `procedurePermissions`.

**ALL procedures** (read, write, delete) must be mapped here for registered services.
Unmapped procedures under a registered service prefix are denied by default (fail-closed).

Source: `internal/shared/middleware/rbac_interceptor.go`

## Permission Flow

```
JWT token  →  Auth middleware validates + injects AuthUser into context
AuthUser.Permissions ([]string from JWT "perms" claim)
  →  RBACInterceptor looks up procedure → required permission
     →  user.HasPermission(perm): true if exact match OR "admin:*"
        → 403 PermissionDenied if false
```

Source: `internal/shared/auth/context.go` — `AuthUser` and `HasPermission`.

## Admin Wildcard

`admin:*` in the JWT `perms` claim satisfies **any** permission check.
Role alone is insufficient — the wildcard must appear in the permissions slice.

## Adding Permissions for a New Module

1. Declare permission constants in `internal/shared/middleware/rbac.go`:
   ```go
   PermOrderRead   Permission = "order:read"
   PermOrderWrite  Permission = "order:write"
   PermOrderDelete Permission = "order:delete"
   ```

2. Mount the service on an auth-protected Echo route group:
   ```go
   g := e.Group(path, appmw.Auth(cfg, rdb))
   // Permission checks are handled by RBACInterceptor, not the group middleware.
   ```

3. Register ALL procedures in `procedurePermissions` (fail-closed pattern)
   (`internal/shared/middleware/rbac_interceptor.go`):
   ```go
   orderv1connect.OrderServiceGetOrderProcedure:    PermOrderRead,
   orderv1connect.OrderServiceListOrdersProcedure:  PermOrderRead,
   orderv1connect.OrderServiceCreateOrderProcedure: PermOrderWrite,
   orderv1connect.OrderServiceUpdateOrderProcedure: PermOrderWrite,
   orderv1connect.OrderServiceDeleteOrderProcedure: PermOrderDelete,
   ```

4. Include the permission strings when issuing tokens
   (`auth.GenerateAccessToken`).

> **Note:** The `task module:create` scaffold automatically injects permission
> constants into `rbac.go` and procedure mappings into `rbac_interceptor.go`.
> After scaffolding, verify the generated entries are correct.
