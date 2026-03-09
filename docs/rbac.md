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

## Two Enforcement Layers

### 1. Echo Middleware (route group level)

`RequirePermission(perms ...Permission)` — applied to an Echo route group.
Checks that the authenticated user holds **all** listed permissions.

`RequireRole(roles ...string)` — alternative role-based check (at least one role must match).

Source: `internal/shared/middleware/rbac.go`

### 2. Connect RPC Interceptor (procedure level)

`RBACInterceptor()` — a `connect.UnaryInterceptorFunc` that maps exact procedure paths
to required permissions via `procedurePermissions`.

**ALL procedures** (read, write, delete) must be mapped here for registered services.
Read procedures are mapped to `read` permissions even though the Echo group provides
the same check — the interceptor enforces fail-closed safety by denying any unmapped procedure.

Source: `internal/shared/middleware/rbac_interceptor.go`

## Permission Flow

```
JWT token  →  auth middleware validates + injects AuthUser into context
AuthUser.Permissions ([]string from JWT "perms" claim)
  →  RequirePermission / RBACInterceptor calls user.HasPermission(perm)
     →  HasPermission iterates Permissions; returns true if exact match OR "admin:*"
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

2. Protect the Echo route group with the read permission:
   ```go
   g := e.Group(path, appmw.Auth(cfg, rdb), appmw.RequirePermission(middleware.PermOrderRead))
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
