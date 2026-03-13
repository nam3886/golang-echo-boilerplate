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

`Auth(cfg, rdb)` — validates the JWT and injects `AuthUser` into the request context.
Returns 401 if the token is missing, expired, or invalid.

No Echo-level permission middleware is applied. All permission enforcement
is handled exclusively by `RBACInterceptor` at the Connect RPC layer.

Source: `internal/shared/middleware/rbac_interceptor.go`

### Connect RPC Interceptor: Permission Enforcement

`RBACInterceptor(procedurePerms map[string]Permission)` — a `connect.UnaryInterceptorFunc`
that maps exact procedure paths to required permissions.

Each module defines its own `procedurePerms` map in `adapters/grpc/routes.go` and passes
it to `RBACInterceptor`. There is no global shared map — each service owns its permissions.

**ALL procedures** (read, write, delete) must be mapped. Unmapped procedures under a
registered service prefix are denied by default (fail-closed).

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

3. Define a per-module procedure map and pass it to `RBACInterceptor` in
   `internal/modules/order/adapters/grpc/routes.go`:
   ```go
   var orderProcedurePerms = map[string]appmw.Permission{
       orderv1connect.OrderServiceGetOrderProcedure:    appmw.PermOrderRead,
       orderv1connect.OrderServiceListOrdersProcedure:  appmw.PermOrderRead,
       orderv1connect.OrderServiceCreateOrderProcedure: appmw.PermOrderWrite,
       orderv1connect.OrderServiceUpdateOrderProcedure: appmw.PermOrderWrite,
       orderv1connect.OrderServiceDeleteOrderProcedure: appmw.PermOrderDelete,
   }

   func RegisterRoutes(e *echo.Echo, handler *OrderServiceHandler, cfg *config.Config, rdb *redis.Client) {
       path, h := orderv1connect.NewOrderServiceHandler(handler,
           connect.WithInterceptors(
               appmw.RBACInterceptor(orderProcedurePerms),
               validate.NewInterceptor(),
           ),
       )
       g := e.Group(path, appmw.Auth(cfg, rdb))
       g.Any("*", echo.WrapHandler(http.StripPrefix(path, h)))
   }
   ```

4. Include the permission strings when issuing tokens
   (`auth.GenerateAccessToken`).

> **Note:** `task module:create` generates `routes.go` with the procedure map pre-filled.
> After scaffolding, verify the generated `Perm{Name}*` constants exist in `rbac.go`.
