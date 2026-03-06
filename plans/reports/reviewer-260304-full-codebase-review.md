# Full Codebase Review - gnha-services

**Date:** 2026-03-04
**Reviewer:** code-reviewer
**Scope:** All Go source files, SQL, protobuf, Docker, configuration (55 files, ~2400 LOC non-generated)

---

## Overall Assessment

The codebase demonstrates solid architectural decisions: simplified hexagonal architecture with proper domain encapsulation, Fx dependency injection, typed domain errors, cursor pagination, and distributed event publishing. The previous review round fixed several critical issues (SELECT FOR UPDATE, OTel shutdown, context propagation, uuid.Parse over MustParse in audit). The code is clean, well-organized, and follows Go conventions.

However, several important issues remain -- primarily around missing auth implementation, zero test coverage, incomplete RBAC enforcement on RPC routes, and a pagination cursor bug.

---

## CRITICAL

### C-1: AuthService proto defined but NO Go implementation exists
**Files:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/proto/auth/v1/auth.proto`, generated code at `/Users/namnguyen/Desktop/www/freelance/gnha-services/gen/proto/auth/v1/`
**Impact:** Login, refresh token, and logout RPCs are defined in proto and generated but never implemented. Users cannot authenticate -- the entire JWT flow is dead code with no way to obtain tokens.
**Fix:** Implement `internal/modules/auth/` module with Login, RefreshToken, and Logout handlers using the existing `auth.sql` queries and `auth.GenerateAccessToken`/`GenerateRefreshToken` helpers.

### C-2: Pagination nextCursor calculated from wrong row when hasMore=true
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go:80-86`
**Impact:** The repository builds `nextCursor` from the last element of the full result set (which includes the extra "probe" row). When `list_users.go` truncates `users = users[:limit]`, the cursor still points to the (limit+1)th row, skipping one record on the next page.
**Fix:**
```go
// In repository.go List(), build nextCursor ONLY from the rows that will be returned.
// OR: move cursor construction to the app layer after truncation.
// Simplest fix in list_users.go:
if hasMore {
    users = users[:limit]
    last := users[len(users)-1]
    if uid, err := uuid.Parse(string(last.ID())); err == nil {
        nextCursor = encodeCursor(last.CreatedAt(), uid) // recalculate from truncated slice
    }
} else {
    nextCursor = ""
}
```
The current code has the app layer using `nextCursor` returned by the repo (built from the extra row), then setting `nextCursor = ""` when !hasMore -- but when hasMore is true, it keeps the repo's cursor which was built from all returned rows including the probe row. This will work correctly only if the repo returns `limit+1` rows and builds the cursor from `users[len(users)-1]` which IS the (limit+1)th row. Actually, let me re-examine...

The repo builds cursor from `users[len(users)-1]` BEFORE truncation. The app layer requests `limit+1` rows. If 21 rows come back, repo cursor points to row 21. App truncates to 20 rows. Next page starts from row 21's cursor -- this actually skips nothing because the cursor comparison is `(created_at, id) < (cursor_t, cursor_id)`, so it fetches rows BEFORE cursor. The cursor from row 21 would fetch rows 22+. But we want rows 21+. So row 21 IS skipped.

**Confirmed bug.** The cursor should be computed from the LAST row that IS included (row 20), not the probe row (row 21).

### C-3: SoftDelete does not check if row was actually deleted
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go:145-152`
**Impact:** `SoftDeleteUser` is `:exec` which doesn't return affected rows. If the user doesn't exist or is already deleted, the operation silently succeeds with no error. The caller (`delete_user.go`) returns nil, and a gRPC DeleteUser response is sent as success.
**Fix:** Change the query to `:execrows` or `:one` with RETURNING, then check if 0 rows were affected and return `ErrNotFound`.
```sql
-- name: SoftDeleteUser :execrows
UPDATE users SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL;
```
```go
func (r *PgUserRepository) SoftDelete(ctx context.Context, id domain.UserID) error {
    uid, err := parseUserID(id)
    if err != nil {
        return err
    }
    q := sqlcgen.New(r.pool)
    rows, err := q.SoftDeleteUser(ctx, uid)
    if err != nil {
        return fmt.Errorf("soft deleting user: %w", err)
    }
    if rows == 0 {
        return sharederr.ErrNotFound
    }
    return nil
}
```

---

## IMPORTANT

### I-1: Connect RPC routes have auth but NO RBAC
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/grpc/routes.go:14-20`
**Impact:** All UserService endpoints require a valid JWT (auth middleware) but no permission checks. Any authenticated user (viewer, member) can CreateUser, UpdateUser, DeleteUser. The RBAC middleware (`RequirePermission`, `RequireRole`) is defined but never applied to any route.
**Fix:** Apply RBAC to the route group or add permission checks inside handlers:
```go
func RegisterRoutes(e *echo.Echo, handler *UserServiceHandler, cfg *config.Config, rdb *redis.Client) {
    path, h := userv1connect.NewUserServiceHandler(handler)
    g := e.Group(path, appmw.Auth(cfg, rdb))

    // Could apply role check per-endpoint. Since Connect uses a single handler
    // behind the path, consider adding an interceptor or checking permissions
    // in each handler method based on the RPC method name.
    g.Any("*", echo.WrapHandler(http.StripPrefix(path, h)))
}
```
Since Echo middleware applies to the entire group and all CRUD operations go through one `Any("*")` handler, the cleanest approach is a Connect interceptor that maps method names to required permissions.

### I-2: Zero test files
**Impact:** No `*_test.go` files anywhere in the repository despite having complete `testutil/` infrastructure with Testcontainers for Postgres, Redis, and RabbitMQ. This is a significant quality and regression risk.
**Fix:** Prioritize tests for:
1. Domain entity logic (`domain/user.go` -- NewUser, ChangeName, ChangeRole, Role.IsValid)
2. Repository layer (CRUD, pagination, soft delete)
3. App handlers (CreateUser uniqueness check, UpdateUser partial updates)
4. Auth (JWT generation/validation, password hash/verify, token blacklist)

### I-3: buf.validate proto annotations not enforced at runtime
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/proto/user/v1/user.proto` (lines 28-31, 39, 47, 58)
**Impact:** Proto annotations like `string.email = true`, `string.min_len = 8`, `string.uuid = true` are defined but never validated server-side. The Connect RPC handler does not use a `protovalidate` interceptor. Malformed requests (empty email, short password, non-UUID IDs) reach the domain layer which may or may not catch them.
**Fix:** Add `bufbuild/protovalidate-go` interceptor:
```go
import "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go"

// In routes.go or handler constructor:
validator, _ := protovalidate.New()
interceptors := connect.WithInterceptors(
    protovalidateconnect.NewInterceptor(validator),
)
path, h := userv1connect.NewUserServiceHandler(handler, interceptors)
```

### I-4: UpdateUser does not publish user.updated event
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/update_user.go`
**Impact:** Audit trail only captures user.updated events if they're published, but `UpdateUserHandler` doesn't have an `EventBus` dependency and never publishes `TopicUserUpdated`. The audit subscriber is registered for `user.updated` but will never fire.
**Fix:** Inject `*events.EventBus` into `UpdateUserHandler` and publish after successful commit.

### I-5: DeleteUser does not publish user.deleted event
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/delete_user.go`
**Impact:** Same as I-4. The audit subscriber for `user.deleted` will never fire.

### I-6: Audit ActorID is set to EntityID (the user being acted on)
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/audit/subscriber.go:44-45,65-69,88-89`
**Impact:** Audit records always set `actor_id = entity_id`, meaning "the user created themselves." The actual admin/actor performing the action is lost. Events don't carry the actor's identity.
**Fix:** Add `ActorID` field to event structs, populated from `auth.UserFromContext(ctx)` in app handlers, and use it in audit subscriber.

### I-7: CreateUser email uniqueness check is not atomic
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user.go:38-45`
**Impact:** The check-then-insert pattern (GetByEmail, then Create) has a TOCTOU race. Two concurrent CreateUser requests with the same email could both pass the uniqueness check and one will hit a Postgres UNIQUE violation -- which surfaces as a raw `fmt.Errorf("inserting user: ...")` rather than the domain `ErrEmailTaken`.
**Fix:** Add a unique constraint violation handler in the repository:
```go
func (r *PgUserRepository) Create(ctx context.Context, user *domain.User) error {
    q := sqlcgen.New(r.pool)
    _, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{...})
    if err != nil {
        if isUniqueViolation(err) {
            return domain.ErrEmailTaken
        }
        return fmt.Errorf("inserting user: %w", err)
    }
    return nil
}

func isUniqueViolation(err error) bool {
    var pgErr *pgconn.PgError
    return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
```

### I-8: SMTP error message leaks recipient email
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/notification/email.go:38-39`
**Impact:** The error `fmt.Errorf("sending email to %s: %w", to, err)` includes the raw email address. If this error propagates up (e.g., logged or returned in non-production), it leaks PII.
**Fix:** Use a sanitized identifier: `fmt.Errorf("sending email: %w", err)` and log the recipient separately at debug level.

### I-9: Password not validated before hashing
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user.go:47`
**Impact:** Without the protovalidate interceptor (I-3), there's no server-side password length/complexity check. An empty string password would be accepted and hashed. Domain validation checks email and name but not password.
**Fix:** Add password validation in the domain layer or app layer:
```go
if len(cmd.Password) < 8 {
    return nil, sharederr.New(sharederr.CodeInvalidArgument, "password must be at least 8 characters")
}
```

### I-10: Request ID from client not validated
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/request_id.go:19-21`
**Impact:** Client-supplied `X-Request-ID` header is trusted without validation. A client could send an extremely long string or inject control characters, which end up in logs.
**Fix:** Validate that the provided ID is a reasonable length (<=128 chars) and contains only printable ASCII:
```go
if id == "" || len(id) > 128 {
    id = uuid.NewString()
}
```

### I-11: Watermill router uses context.Background() instead of Fx-provided context
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/events/subscriber.go:59`
**Impact:** `router.Run(context.Background())` ignores the lifecycle context. While `router.Close()` is called in OnStop, using the proper context would allow the router to detect shutdown signals directly.
**Fix:** Pass a cancellable context derived from the Fx lifecycle or signal handling.

---

## MINOR

### M-1: Seed data uses hardcoded passwords
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/cmd/seed/main.go:24-28`
**Impact:** Low risk if only used in dev, but the seed passwords are weak and hardcoded. If accidentally run in production, creates known-credential accounts.
**Fix:** Guard with environment check: `if !cfg.IsDevelopment() { log.Fatal("seed only runs in development") }`.

### M-2: `model.BaseModel` is unused
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/model/base.go`
**Impact:** Dead code. The User domain entity uses its own fields; sqlc generates its own model struct. `BaseModel` is imported by nothing.
**Fix:** Remove the file or use it if planned for future entities.

### M-3: Docker healthcheck uses curl but readyz could be more meaningful
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/Dockerfile:17-18`
**Impact:** The `/healthz` endpoint always returns `{"status":"ok"}` regardless of database or Redis connectivity. `/readyz` is identical. Neither checks actual dependencies.
**Fix:** Implement `/readyz` that pings Postgres and Redis:
```go
e.GET("/readyz", func(c echo.Context) error {
    if err := pool.Ping(c.Request().Context()); err != nil {
        return c.JSON(503, map[string]string{"status": "not ready"})
    }
    return c.JSON(200, map[string]string{"status": "ready"})
})
```

### M-4: CORS AllowCredentials=true with AllowOrigins from config
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/chain.go:27-36`
**Impact:** If `CORS_ORIGINS` is set to `*` in production, `AllowCredentials: true` combined with wildcard origin is a security misconfiguration (browsers will block it, but it signals intent issues).
**Fix:** Add validation in config loading that `CORS_ORIGINS` is not `*` when in production.

### M-5: OTel OTLP exporter always uses insecure transport
**Files:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/observability/tracer.go:22`, `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/observability/metrics.go:20`
**Impact:** `WithInsecure()` means trace/metric data is sent unencrypted. Fine for local/sidecar collectors, but risky if OTLP endpoint is remote.
**Fix:** Make TLS configurable via env var `OTEL_INSECURE` defaulting to `true` for dev.

### M-6: SanitizeHeader defined but never used in request logging
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/request_log.go:63-68`
**Impact:** The `sensitiveHeaders` map and `SanitizeHeader` function are defined but the `RequestLogger` middleware never logs headers, so they're dead code.
**Fix:** Remove or integrate into request logging if header logging is needed.

### M-7: Elasticsearch URL configured but no Elasticsearch usage
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/config/config.go:26`
**Impact:** `ESURL` config field exists and Elasticsearch is in dev docker-compose, but no Go code uses it. Dead config.
**Fix:** Remove until needed (YAGNI).

### M-8: Swagger OpenAPI spec file may not exist
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/swagger.go:30`
**Impact:** References `/swagger/spec/user/v1/user.swagger.json` but the buf.gen.yaml generates to `gen/openapi/` and the openapiv2 plugin output path may differ. No `gen/openapi/` directory was observed in the file listing.
**Fix:** Verify that `buf generate` actually produces the swagger file at the expected path. Run `buf generate` and check output.

### M-9: No Makefile or task runner
**Impact:** No `Makefile`, `Taskfile.yml`, or equivalent. Developers must know the exact commands for `buf generate`, `sqlc generate`, `go build`, `goose migrate`, etc.
**Fix:** Add a Makefile with standard targets: `build`, `test`, `lint`, `generate`, `migrate-up`, `seed`.

### M-10: No .gitignore or .env.example visible
**Impact:** Without `.gitignore`, `tmp/` (from Air), `.env`, and other artifacts may be committed. Without `.env.example`, new developers don't know required env vars.
**Fix:** Ensure `.gitignore` covers `tmp/`, `.env`, binaries. Add `.env.example` listing all required vars.

### M-11: MeterProvider shutdown error silently swallowed
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/module.go:29`
**Impact:** `_ = mp.Shutdown(ctx)` discards error. If metric flush fails, the operator won't know.
**Fix:** Log the error: `if err := mp.Shutdown(ctx); err != nil { slog.Error("meter provider shutdown", "err", err) }`.

---

## Positive Observations

1. **Clean hexagonal architecture** -- Domain entities with unexported fields, getters, and `Reconstitute()` pattern correctly separate persistence from business logic.
2. **Proper password hashing** -- argon2id with constant-time comparison and correct parameter extraction from stored hash.
3. **JWT signing method validation** -- `ValidateAccessToken` correctly checks for `*jwt.SigningMethodHMAC` to prevent algorithm confusion attacks.
4. **JWT secret minimum length** -- 32-char minimum enforced in `config.Load()`.
5. **Token blacklist support** -- Redis-based token blacklist check in auth middleware.
6. **Distributed locking for cron** -- Correct Redis SETNX + Lua unlock pattern with unique tokens.
7. **OTel trace context propagation** -- Events carry trace context through Watermill metadata.
8. **OTel provider shutdown** -- Properly registered via Fx lifecycle hooks (fixed from previous review).
9. **SELECT FOR UPDATE** in update transactions (fixed from previous review).
10. **uuid.Parse (not MustParse)** in audit subscriber with proper error handling (fixed from previous review).
11. **msg.Context()** used in audit/notification subscribers (fixed from previous review).
12. **SMTP CRLF injection prevention** -- Headers are sanitized and envelope recipients are also sanitized.
13. **Event publish errors logged** rather than silently swallowed (fixed from previous review).
14. **Sliding window rate limiting** with Redis sorted sets -- correct implementation.
15. **Sensitive header redaction** defined (though not yet wired into logging).
16. **Security headers** comprehensively set (HSTS, CSP, X-Frame-Options, etc.).

---

## Recommended Actions (Priority Order)

1. **[CRITICAL]** Implement AuthService (Login/Refresh/Logout) -- without this, no user can authenticate.
2. **[CRITICAL]** Fix pagination cursor to use the last included row, not the probe row.
3. **[CRITICAL]** Handle soft-delete of non-existent user (return ErrNotFound).
4. **[IMPORTANT]** Add RBAC to Connect RPC routes (via interceptor or handler-level checks).
5. **[IMPORTANT]** Add protovalidate interceptor to enforce proto field constraints.
6. **[IMPORTANT]** Publish user.updated and user.deleted events from UpdateUser/DeleteUser handlers.
7. **[IMPORTANT]** Handle Postgres unique violation in Create to properly map to ErrEmailTaken.
8. **[IMPORTANT]** Add ActorID to domain events and audit log entries.
9. **[IMPORTANT]** Add server-side password validation.
10. **[IMPORTANT]** Write tests -- at minimum domain unit tests and repository integration tests.
11. **[MINOR]** Add `/readyz` dependency checks, Makefile, .env.example, guard seed for dev-only.

---

## Metrics

| Metric | Value |
|--------|-------|
| Go source files (non-generated) | 37 |
| Generated files | 6 (sqlc + proto) |
| LOC (non-generated, approx) | ~1,400 |
| Test files | 0 |
| Test coverage | 0% |
| Critical issues | 3 |
| Important issues | 11 |
| Minor issues | 11 |
| Previously-fixed issues confirmed | 6/6 verified |

---

## Unresolved Questions

1. Is Go 1.26.0 the intended target? The Dockerfile uses `golang:1.26-alpine` which aligns, but verify the alpine tag exists for 1.26.
2. Is an auth module implementation planned separately or was it expected to be in this codebase?
3. Should the API support unauthenticated CreateUser (registration) vs admin-only user creation? This affects RBAC design.
4. Is the Elasticsearch config intended for future search functionality? If not planned soon, remove per YAGNI.
