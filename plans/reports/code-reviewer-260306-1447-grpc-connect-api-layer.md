# Code Review: gRPC/Connect-RPC API Layer & Proto

**Date:** 2026-03-06
**Scope:** Proto definitions, Connect-RPC handlers, middleware chain, app service layer, request validation, pagination
**Files reviewed:** 20 (proto, handler, mapper, routes, middleware, app services, repository, SQL, migrations)

---

## Overall Assessment

The API layer is well-architected. Proto definitions use buf/validate correctly, the handler is thin and delegates cleanly to app-layer use cases, error mapping is exhaustive, and the middleware chain is ordered sensibly. The RBAC interceptor at the Connect level is a good pattern. A few issues warrant attention.

**Score: 8.5/10**

---

## Critical Issues

None.

---

## High Priority

### H-1: Internal error messages leak to clients via Connect

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/grpc/mapper.go:32`

```go
return connect.NewError(connect.CodeInternal, err)
```

When `err` is NOT a `DomainError`, the raw error (which may contain SQL queries, stack traces, or internal paths) is wrapped directly into the Connect error and sent to the client. Connect serializes `err.Error()` as the error message.

**Fix:** Replace with a generic message for non-domain errors:

```go
// Log the real error, return a generic one
slog.Error("unexpected error in handler", "err", err)
return connect.NewError(connect.CodeInternal, errors.New("internal error"))
```

### H-2: `UpdateUser` always sends both name and role to DB even if only one changed

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go:147-153`

The `Update` method always sends both `name` and `role` as `Valid: true` to the SQL `COALESCE` update, even if the closure only changed one field. This means if only `name` is changed, `role` is still written (to the same value). While functionally correct due to `COALESCE`, it:
- Generates unnecessary WAL entries for unchanged columns
- Makes audit trail diffing harder (no way to know what actually changed)

**Impact:** Low-medium in practice, but worth noting for correctness. The SQL `COALESCE` approach silently overwrites with same value.

### H-3: Missing pagination index for keyset query

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/db/migrations/00001_initial_schema.sql`

The `ListUsers` SQL uses `ORDER BY created_at DESC, id DESC` with keyset condition `(created_at, id) < (?, ?)`, but there is no composite index on `(created_at DESC, id DESC)` where `deleted_at IS NULL`. The existing indexes (`idx_users_email`, `idx_users_active`) do not cover this query. This will cause a sequential scan on tables with >10k rows.

**Fix:** Add a migration:

```sql
CREATE INDEX idx_users_list ON users (created_at DESC, id DESC) WHERE deleted_at IS NULL;
```

---

## Medium Priority

### M-1: `ListUsersRequest.cursor` has no validation

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/proto/user/v1/user.proto:48`

The `cursor` field has no buf/validate constraint. A malformed base64 string is silently ignored (line 66-69 of repository.go -- `if err == nil`), which means an invalid cursor returns results from the beginning of the list rather than an error. Clients with corrupted cursors will silently get duplicate pages.

**Recommendation:** Either:
- Return `CodeInvalidArgument` on malformed cursor, OR
- Document the silent-reset behavior as intentional (lenient pagination)

### M-2: `toProto` maps password hash through domain getter

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/grpc/mapper.go:14-23`

The `toProto` mapper does not include `password` in the proto `User` message (correct), but `domain.User.Password()` is still a public getter. Any new proto field or mapper change could accidentally expose it. The `ListUsers` handler iterates all users calling `toProto`, meaning password hashes flow through the entire call chain before being stripped.

**Recommendation:** Consider removing `Password()` getter from the domain entity and instead providing a `VerifyPassword(hash)` method, or at minimum add a comment warning.

### M-3: Connect interceptor order -- validate runs before RBAC

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/grpc/routes.go:18-19`

```go
connect.WithInterceptors(
    validate.NewInterceptor(),  // runs first
    appmw.RBACInterceptor(),    // runs second
)
```

Validation runs before RBAC, meaning unauthenticated or unauthorized users get validation errors (e.g., "email must be valid") instead of permission-denied errors. This leaks API schema information. RBAC should run first.

**Fix:** Swap order:

```go
connect.WithInterceptors(
    appmw.RBACInterceptor(),
    validate.NewInterceptor(),
)
```

### M-4: `permissionForProcedure` uses prefix matching which is fragile

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/rbac_interceptor.go:51-52`

```go
if strings.HasPrefix(method, prefix) {
```

This matches any method starting with "Create", "Update", or "Delete". If a new RPC like `CreateUserInvitation` is added, it would silently require `PermUserWrite` even if it should have different permissions. An explicit map from full procedure name to permission is safer.

**Recommendation:** Use full procedure constants from `userv1connect`:

```go
var procedurePermissions = map[string]Permission{
    userv1connect.UserServiceCreateUserProcedure: PermUserWrite,
    userv1connect.UserServiceUpdateUserProcedure: PermUserWrite,
    userv1connect.UserServiceDeleteUserProcedure: PermUserDelete,
}
```

### M-5: `UpdateUserRequest` allows empty update (no fields set)

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/proto/user/v1/user.proto:57-61`

Both `name` and `role` are optional. A request with only `id` is valid per proto validation, causing a full round-trip to the DB (SELECT FOR UPDATE + UPDATE + COMMIT) that changes nothing.

**Fix:** Add validation in the handler or app layer:

```go
if cmd.Name == nil && cmd.Role == nil {
    return nil, sharederr.New(sharederr.CodeInvalidArgument, "at least one field must be provided")
}
```

---

## Low Priority

### L-1: `codeToConnect` map missing zero-value guard

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/grpc/mapper.go:29`

If `domErr.Code` is not in `codeToConnect`, the map returns `connect.Code(0)` which is `connect.CodeCanceled`. This is misleading. Add a fallback:

```go
code, ok := codeToConnect[domErr.Code]
if !ok {
    code = connect.CodeInternal
}
```

### L-2: Proto `User.role` is a plain `string` instead of an enum

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/proto/user/v1/user.proto:21`

Using `string` for `role` in the response message means clients don't get type safety. Consider defining a `Role` enum in proto. The request messages already use `string.in` validation for the same values.

### L-3: `SanitizeHeader` in request_log.go is dead code

Previously reported. Still present. No callers reference it.

### L-4: Swagger CDN URLs are unpinned

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/swagger.go:62-65`

`https://unpkg.com/swagger-ui-dist@5/` will auto-resolve to the latest 5.x, which could break the UI unexpectedly. Pin to a specific version (e.g., `@5.17.14`).

---

## Positive Observations

1. **Thin handlers:** The gRPC handler layer is purely a mapping layer -- no business logic leaks in. Each method is 5-10 lines.
2. **buf/validate integration:** Proto-level validation via `connectrpc.com/validate` interceptor eliminates boilerplate validation code.
3. **Compile-time interface check:** `var _ userv1connect.UserServiceHandler = (*UserServiceHandler)(nil)` on line 39 of handler.go catches drift early.
4. **Error mapping is exhaustive:** 8 domain error codes mapped to Connect codes, with a catch-all `CodeInternal`.
5. **Middleware chain is well-ordered:** Recovery > RequestID > Logger > BodyLimit > Gzip > Security > CORS > Timeout > RateLimit. Auth/RBAC correctly at route-group level.
6. **Keyset pagination:** Proper cursor-based (not offset) pagination with `limit+1` trick for `hasMore`.
7. **Transactional updates:** `SELECT ... FOR UPDATE` in a transaction prevents concurrent mutation races.
8. **RBAC at two levels:** Echo middleware for route-group base permission, Connect interceptor for per-procedure write/delete.
9. **Soft delete everywhere:** Queries consistently filter `WHERE deleted_at IS NULL`.
10. **TypeScript codegen:** buf.gen.yaml includes `buf.build/bufbuild/es` for frontend type safety.

---

## Metrics

| Metric | Value |
|--------|-------|
| Proto messages | 10 (5 req/res pairs + User) |
| RPCs | 5 (CRUD + List) |
| Middleware layers | 9 global + 2 route-group |
| Error code mappings | 8 domain -> Connect |
| Validation rules | 8 (email, uuid, string length, enum) |
| SQL queries | 7 |
| Indexes | 3 (missing 1 for pagination) |

---

## Recommended Actions (priority order)

1. **[HIGH] Fix internal error leakage** in `domainErrorToConnect` -- replace raw err with generic message
2. **[HIGH] Add composite index** `(created_at DESC, id DESC) WHERE deleted_at IS NULL` for pagination
3. **[MEDIUM] Swap interceptor order** -- RBAC before validate to prevent schema leakage
4. **[MEDIUM] Use explicit procedure map** in RBAC interceptor instead of prefix matching
5. **[MEDIUM] Reject or document invalid cursors** in pagination
6. **[MEDIUM] Guard empty UpdateUser** requests at app or handler layer
7. **[LOW] Add zero-value guard** to `codeToConnect` map lookup
8. **[LOW] Pin Swagger CDN version**
9. **[LOW] Consider proto enum for Role**

---

## Unresolved Questions

- Is silent cursor reset on malformed pagination cursor intentional (lenient) or a bug?
- Should `Password()` getter be removed from domain entity to prevent accidental exposure?
- Is the Connect handler meant to also serve gRPC-Web (browser clients)? If so, CORS `AllowHeaders` may need `Grpc-Status` and `Grpc-Message`.
