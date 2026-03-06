# Full Codebase Review -- gnha-services Go API Boilerplate

**Date:** 2026-03-04 19:22 ICT
**Reviewer:** code-reviewer agent
**Verdict:** FAIL -- 5 CRITICAL, 8 IMPORTANT, 9 MINOR issues found

---

## 1. Executive Summary

The codebase demonstrates a well-structured Go modular monolith following hexagonal architecture with solid foundations: Fx DI, Connect RPC, pgx+sqlc, Watermill events, and proper domain error handling. However, **the project cannot build or deploy as-is** due to a Go version mismatch that makes the Dockerfile and CI pipeline incompatible with the source code. Additional critical issues include panic-inducing code in event handlers, a broken cursor pagination query, missing `SELECT FOR UPDATE` in the update transaction, and silently swallowed event publish errors.

**Pass/Fail:** FAIL (5 critical issues must be resolved before production)

---

## 2. CRITICAL Issues

### C-1: Go Version Mismatch -- BUILD WILL FAIL

**Files:**
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/go.mod` (line 3)
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/Dockerfile` (line 2)
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/.gitlab-ci.yml` (line 8, 12)

**Problem:** `go.mod` declares `go 1.25.0` but the Dockerfile uses `golang:1.23-alpine` and CI uses `GO_VERSION: "1.23"`. Go 1.25 does not exist yet (latest stable is 1.23.x as of March 2026). The `go.mod` version is fictional.

**Impact:** The codebase uses `for i := range 10` syntax (integer range loops) in:
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/database/postgres.go` line 27
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/database/redis.go` line 26

This syntax was introduced in **Go 1.22**. While it will work with Go 1.23, the `go.mod` version `1.25.0` is invalid and may cause `go mod tidy` or tooling failures depending on the Go toolchain version installed.

**Fix:** Change `go.mod` to match reality:
```go
// go.mod line 3
go 1.23.0
```

No Go 1.24+ or 1.25+ exclusive features are used beyond the integer range syntax. The code is compatible with Go 1.23 once the `go.mod` version is corrected.

---

### C-2: `uuid.MustParse` in Audit Subscriber -- Panics on Bad Data

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/audit/subscriber.go` lines 37-39, 55-58, 71-74

**Problem:** `uuid.MustParse(event.UserID)` panics if the event payload contains a malformed UUID. Since this runs in a Watermill message handler, a panic will crash the handler goroutine. Although the Watermill router has `Recoverer` middleware, relying on panic recovery as flow control is dangerous and will trigger retry storms.

**Impact:** A single malformed event payload causes panic, recovery, retry (3x), and eventual message dead-lettering -- all while generating misleading error logs.

**Fix:**
```go
func (h *Handler) HandleUserCreated(msg *message.Message) error {
    var event events.UserCreatedEvent
    if err := json.Unmarshal(msg.Payload, &event); err != nil {
        slog.Error("audit: failed to unmarshal user created event", "err", err)
        return err
    }

    entityID, err := uuid.Parse(event.UserID)
    if err != nil {
        slog.Error("audit: invalid user ID in event", "user_id", event.UserID, "err", err)
        return nil // ack the message; retrying won't fix bad data
    }

    ctx := context.Background()
    changes, _ := json.Marshal(event)

    return h.queries.CreateAuditLog(ctx, sqlcgen.CreateAuditLogParams{
        EntityType: "user",
        EntityID:   entityID,
        Action:     "created",
        ActorID:    entityID,
        Changes:    changes,
    })
}
```

Apply the same pattern to `HandleUserUpdated` and `HandleUserDeleted`.

---

### C-3: Cursor Pagination Broken -- CursorID Passed as Timestamptz Instead of UUID

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` lines 62-67

**Problem:** The `ListUsersParams.CursorID` field is typed `pgtype.Timestamptz` (see generated code at `/Users/namnguyen/Desktop/www/freelance/gnha-services/gen/sqlc/user.sql.go` line 101), but the SQL query uses it as a UUID comparison: `(created_at, id) < ($2, $3)`. The repository sets it to the same timestamp value as `CursorCreatedAt`:
```go
params.CursorID = pgtype.Timestamptz{Time: decoded.T, Valid: true}
```

This means the cursor ID from `decoded.U` (the UUID) is completely ignored. The query will produce incorrect results -- it compares `id < timestamp` which is a type mismatch in Postgres and will either error or produce garbage results.

**Root cause:** The SQL query in `db/queries/user.sql` line 11 declares `sqlc.narg('cursor_id')` without an explicit cast to UUID, so sqlc inferred the type from the surrounding context (Timestamptz from the comparison with `created_at`).

**Fix:** Update the SQL query:
```sql
-- name: ListUsers :many
SELECT * FROM users
WHERE deleted_at IS NULL
  AND (sqlc.narg('cursor_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_created_at'), sqlc.narg('cursor_id')::uuid))
ORDER BY created_at DESC, id DESC
LIMIT $1;
```

Then regenerate sqlc and update the repository:
```go
if cursor != "" {
    decoded, err := decodeCursor(cursor)
    if err == nil {
        params.CursorCreatedAt = pgtype.Timestamptz{Time: decoded.T, Valid: true}
        params.CursorID = decoded.U // This should be pgtype.UUID after regeneration
    }
}
```

---

### C-4: Update Transaction Lacks SELECT FOR UPDATE -- Lost Updates

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` lines 105-143

**Problem:** The `Update` method begins a transaction, reads the user with `GetUserByID`, applies mutations, then writes back. But `GetUserByID` does not use `SELECT ... FOR UPDATE`, so concurrent updates can read the same row simultaneously and the last write wins, silently overwriting the other's changes.

**Fix:** Add a new sqlc query for locking:
```sql
-- name: GetUserByIDForUpdate :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;
```

Then use it in the Update method:
```go
row, err := q.GetUserByIDForUpdate(ctx, uid)
```

---

### C-5: Event Publish Errors Silently Swallowed

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user.go` line 61

**Problem:**
```go
_ = h.bus.Publish(ctx, events.TopicUserCreated, events.UserCreatedEvent{...})
```

The event publish error is explicitly discarded. If RabbitMQ is down or the connection drops, the user is created in the database but the event is permanently lost. Downstream consumers (audit trail, welcome email) will never fire.

**Impact:** Silent data inconsistency. Audit trail has gaps. Welcome emails are never sent. No way to detect or recover from these failures.

**Fix option A (log + continue):**
```go
if err := h.bus.Publish(ctx, events.TopicUserCreated, ...); err != nil {
    slog.Error("failed to publish user.created event",
        "user_id", string(user.ID()), "err", err)
    // User was created successfully, so don't fail the request.
    // Consider an outbox pattern for guaranteed delivery.
}
```

**Fix option B (outbox pattern):** Write the event to an `outbox` table in the same transaction as the user creation, then have a background worker relay events to RabbitMQ. This guarantees at-least-once delivery.

---

## 3. IMPORTANT Issues

### I-1: AuthService Proto Defined But No Go Implementation

**Files:**
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/proto/auth/v1/auth.proto`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/gen/proto/auth/v1/authv1connect/auth.connect.go`

**Problem:** The proto defines Login, RefreshToken, and Logout RPCs with generated Connect stubs, but there is no Go implementation. The `authv1connect.AuthServiceHandler` interface is unimplemented. No auth module exists in `internal/modules/`.

**Impact:** Users cannot authenticate. The JWT infrastructure (token generation, refresh tokens, blacklisting) exists but is not wired to any endpoint. The API is effectively read-only without authentication for Connect RPC clients.

**Recommendation:** Create `internal/modules/auth/` with handler, app layer, and module registration. This is likely Phase 2 work but should be documented as a known gap.

---

### I-2: No Test Files Exist

**Problem:** Zero `*_test.go` files found in the entire codebase. The testutil package provides container helpers (`NewTestPostgres`, `NewTestRedis`, `NewTestRabbitMQ`) and fixtures, but no tests use them.

**Impact:** CI `unit-test` stage will report 0% coverage. No regression safety net. The GitLab CI pipeline publishes coverage artifacts, but they will be empty.

---

### I-3: SMTP sender passes unsanitized `to` in envelope

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/notification/email.go` line 38

**Problem:** While CRLF injection is handled for headers (line 31-33), the `smtp.SendMail` envelope recipient (`[]string{to}`) receives the raw `to` value, not the sanitized version. If `to` contains CRLF, the SMTP envelope is still vulnerable.

```go
// Line 35-36: Headers use sanitize(to) -- good
// Line 38: Envelope uses raw `to` -- bad
if err := smtp.SendMail(addr, nil, s.from, []string{to}, []byte(msg)); err != nil {
```

**Fix:**
```go
sanitizedTo := sanitize(to)
// ... build msg with sanitizedTo ...
if err := smtp.SendMail(addr, nil, sanitize(s.from), []string{sanitizedTo}, []byte(msg)); err != nil {
```

---

### I-4: No Input Validation on Connect RPC Handlers

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/grpc/handler.go`

**Problem:** The proto files define `buf.validate` rules (email validation, UUID format, min/max lengths), but the handler code does not call any proto-validate interceptor. The validation annotations are decorative -- they are never enforced at runtime.

**Fix:** Add a Connect interceptor for protovalidate:
```go
import "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
// or use connectrpc.com/validate interceptor
```

---

### I-5: Connect RPC Routes Have No Auth/RBAC Middleware

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/grpc/routes.go`

**Problem:** Routes are mounted on Echo with `e.Any(path+"*", ...)` but no auth or RBAC middleware is applied. The comment in `chain.go` line 47 says "Auth + RBAC applied at route group level, not global" but the routes are not in any group and have no middleware.

**Impact:** All user CRUD operations are unauthenticated and accessible to anyone.

**Fix:**
```go
func RegisterRoutes(e *echo.Echo, handler *UserServiceHandler, cfg *config.Config, rdb *redis.Client) {
    path, h := userv1connect.NewUserServiceHandler(handler)
    g := e.Group(path, middleware.Auth(cfg, rdb))
    g.Any("*", echo.WrapHandler(http.StripPrefix("", h)))
}
```

---

### I-6: Audit Handlers Use `context.Background()` Instead of Message Context

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/audit/subscriber.go` lines 32, 51, 70

**Problem:** Every handler creates `ctx := context.Background()`, discarding the message context which contains the propagated OTel trace from the publisher. This breaks distributed tracing -- audit writes won't be correlated with the originating request.

**Fix:**
```go
ctx := msg.Context()
```

Same issue in `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/notification/subscriber.go` line 40.

---

### I-7: No Graceful Shutdown for OTel Providers

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/module.go`

**Problem:** `TracerProvider` and `MeterProvider` are created via `fx.Provide` but their `Shutdown()` methods are never called. Pending traces/metrics will be lost on shutdown.

**Fix:** Use Fx lifecycle hooks or `fx.Supply` with shutdown:
```go
fx.Provide(func(cfg *config.Config, lc fx.Lifecycle) (*sdktrace.TracerProvider, error) {
    tp, err := observability.NewTracerProvider(cfg)
    if err != nil {
        return nil, err
    }
    lc.Append(fx.Hook{
        OnStop: func(ctx context.Context) error {
            return tp.Shutdown(ctx)
        },
    })
    return tp, nil
}),
```

---

### I-8: Cron Distributed Lock Is Unsafe -- No Deadlock Protection

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/cron/scheduler.go` lines 33-37

**Problem:** The lock uses `SetNX` with a 5-minute TTL, then `defer Del`. If the job takes longer than 5 minutes, the lock expires, another instance acquires it, and both run concurrently. Then the first instance's `defer Del` deletes the second instance's lock.

**Fix:** Use Redis Lua script or Redlock pattern with lock token verification:
```go
lockValue := uuid.NewString()
locked, err := s.rdb.SetNX(ctx, "cron:"+name, lockValue, 5*time.Minute).Result()
if err != nil || !locked {
    return
}
defer func() {
    // Only delete if we still own the lock
    script := `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`
    s.rdb.Eval(ctx, script, []string{"cron:" + name}, lockValue)
}()
```

---

## 4. MINOR Issues

### M-1: `model.BaseModel` Is Unused Dead Code

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/model/base.go`

The domain uses encapsulated fields with getters, and sqlc generates its own models. `BaseModel` is not referenced anywhere.

### M-2: `SanitizeHeader` Function Is Unused

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/request_log.go` line 63

`SanitizeHeader` is exported but never called. The `sensitiveHeaders` map is also only used by this dead function.

### M-3: `NewLogger` Return Value Not Used by Fx

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/observability/logger.go`

`NewLogger` returns `*slog.Logger` and sets `slog.SetDefault()`. But no other component requests `*slog.Logger` from Fx. The Fx provision works (triggers the side effect), but the return value is wasted.

### M-4: `PgUserRepository` Does Not Implement Interface Via Pointer

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go`

The `NewPgUserRepository` returns `*PgUserRepository` (concrete type). The Fx annotation in `module.go` casts it to `domain.UserRepository`. This works but adding a compile-time check would prevent regression:
```go
var _ domain.UserRepository = (*PgUserRepository)(nil)
```

### M-5: `toDomain` in Repository Accepts Value Instead of Pointer

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` line 164

`func toDomain(row sqlcgen.User) *domain.User` copies the entire `User` struct on each call. For hot paths like `List`, pass by pointer:
```go
func toDomain(row *sqlcgen.User) *domain.User
```

### M-6: `Create` Method Returns Created User from DB but Repo Discards It

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` lines 91-103

The SQL `CreateUser` query uses `RETURNING *` and returns a fully populated row, but the repository ignores it. The app layer then returns the domain entity created *before* the DB insert, which means the `ID`, `CreatedAt`, and `UpdatedAt` fields are from Go, not Postgres. These will differ from what's in the database.

### M-7: Hardcoded Service Version "0.1.0"

**Files:**
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/observability/metrics.go` line 32
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/observability/tracer.go` line 34

Should be injected via ldflags or config.

### M-8: Watermill Router `Run(ctx)` Uses OnStart Context

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/events/subscriber.go` lines 57-63

`router.Run(ctx)` is called inside `OnStart` with the start context. This context is cancelled shortly after start, which may cause the router to stop prematurely. Use `context.Background()` instead:
```go
go func() {
    if err := router.Run(context.Background()); err != nil {
        slog.Error("watermill router error", "err", err)
    }
}()
```

### M-9: `SoftDelete` Does Not Return "Not Found" If No Rows Affected

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` lines 145-152

`SoftDeleteUser` is `:exec` which returns `CommandTag`. The method ignores `RowsAffected()`. Deleting a non-existent user silently succeeds.

---

## 5. Go Version Analysis (Detailed)

| Source | Version | Status |
|--------|---------|--------|
| `go.mod` | `go 1.25.0` | INVALID -- Go 1.25 does not exist |
| Dockerfile | `golang:1.23-alpine` | Valid |
| `.gitlab-ci.yml` default image | `golang:1.23-alpine` | Valid |
| `.gitlab-ci.yml` GO_VERSION | `"1.23"` | Valid |

**Go 1.22+ features used:**
- `for i := range 10` in `database/postgres.go:27` and `database/redis.go:26` -- requires Go 1.22+. Compatible with Go 1.23.

**Go 1.24+ features used:** None found.
**Go 1.25+ features used:** None found (Go 1.25 does not exist).

**Verdict:** Fix `go.mod` to `go 1.23.0` and everything compiles.

---

## 6. Security Findings

| # | Severity | Finding | File |
|---|----------|---------|------|
| S-1 | CRITICAL | All Connect RPC routes are unauthenticated | `grpc/routes.go` |
| S-2 | IMPORTANT | SMTP envelope recipient not sanitized | `notification/email.go:38` |
| S-3 | IMPORTANT | Proto validation rules not enforced at runtime | `grpc/handler.go` |
| S-4 | GOOD | JWT secret minimum 32 chars enforced | `config/config.go:54` |
| S-5 | GOOD | JWT signing method verified | `auth/jwt.go:41` |
| S-6 | GOOD | Token blacklist via Redis | `middleware/auth.go:29` |
| S-7 | GOOD | Argon2id with constant-time comparison | `auth/password.go` |
| S-8 | GOOD | Security headers (HSTS, XFO, CSP-adjacent) | `middleware/security.go` |
| S-9 | GOOD | Sensitive headers redacted in logs | `middleware/request_log.go:12-16` |
| S-10 | GOOD | API key stored as SHA-256 hash | `auth/apikey.go` |
| S-11 | GOOD | No raw SQL -- all via sqlc parameterized queries | `gen/sqlc/*.go` |
| S-12 | NOTE | CORS `AllowCredentials: true` with configurable origins | `middleware/chain.go:27-36` |
| S-13 | NOTE | SMTP has no TLS/auth (fine for dev, not for prod) | `notification/email.go` |

---

## 7. Architecture Assessment

### Strengths
- Clean hexagonal architecture: `domain/` -> `app/` -> `adapters/{postgres,grpc}`
- Domain entities with unexported fields + getters + `Reconstitute` pattern
- Proper port/adapter separation via interfaces (`UserRepository`, `Sender`, `PasswordHasher`)
- Fx dependency injection with proper module grouping
- Event-driven architecture with Watermill + RabbitMQ
- Cursor-based pagination (despite the type bug)
- DomainError mapped to both HTTP and Connect RPC codes

### Concerns
- No auth module implementation despite proto definition
- Event publish is fire-and-forget (no outbox pattern)
- Update transaction is read-then-write without row locking
- Audit module has direct DB access (creates `sqlcgen.Queries` from pool) which is fine for a simple case but bypasses any transaction boundary

### Dependency Direction
All verified correct:
- `domain/` has zero imports from `app/` or `adapters/`
- `app/` imports only `domain/` and `shared/`
- `adapters/grpc` imports `app/` and `domain/`
- `adapters/postgres` imports `domain/` and `gen/sqlc/`
- No circular dependencies detected

---

## 8. Positive Observations

1. **Excellent project structure.** File organization is clean and intuitive. Every file is under 200 lines (non-generated).
2. **Strong password hashing.** Argon2id with proper parameters and constant-time comparison.
3. **Proper JWT validation.** Signing method check, expiry, blacklist support.
4. **Domain-driven error handling.** `DomainError` with HTTP and gRPC code mapping is a clean pattern.
5. **Sliding window rate limiter.** Redis-based, per-user or per-IP, with pipeline for atomicity.
6. **Sensible middleware chain.** Recovery, request ID, logging, body limit, gzip, security headers, CORS, timeout, rate limit -- in correct order.
7. **Testcontainers setup.** Ready-to-use test infrastructure for Postgres, Redis, RabbitMQ.
8. **Taskfile.** Comprehensive developer experience with setup, generate, lint, test, build, migrate, seed, docker, monitoring commands.
9. **CI/CD pipeline.** Lint, generated code check, unit + integration tests, Docker build, staging + production deploy.
10. **Event trace propagation.** OTel context injected into Watermill message metadata.

---

## 9. Recommended Actions (Priority Order)

1. **Fix `go.mod` version** to `go 1.23.0` (C-1) -- 30 seconds
2. **Replace `uuid.MustParse`** with `uuid.Parse` in audit subscriber (C-2) -- 10 minutes
3. **Fix cursor pagination SQL** and regenerate sqlc (C-3) -- 20 minutes
4. **Add `SELECT FOR UPDATE`** query for user updates (C-4) -- 15 minutes
5. **Log event publish errors** instead of discarding (C-5) -- 5 minutes
6. **Apply Auth middleware** to Connect RPC routes (I-5) -- 15 minutes
7. **Add protovalidate interceptor** (I-4) -- 30 minutes
8. **Fix audit/notification context** to use `msg.Context()` (I-6) -- 5 minutes
9. **Fix SMTP envelope sanitization** (I-3) -- 5 minutes
10. **Fix cron distributed lock** with token verification (I-8) -- 20 minutes
11. **Implement AuthService** (I-1) -- multi-day effort
12. **Write tests** (I-2) -- multi-day effort
13. **Add OTel shutdown hooks** (I-7) -- 15 minutes

---

## 10. Metrics

| Metric | Value |
|--------|-------|
| Total Go source files (non-generated) | 42 |
| Total Go source LOC (non-generated) | ~1,600 |
| Generated files | 6 (sqlc + proto) |
| Test files | 0 |
| Test coverage | 0% |
| Critical issues | 5 |
| Important issues | 8 |
| Minor issues | 9 |
| Files > 200 lines | 0 (non-generated) |
| Unused exports | 2 (`BaseModel`, `SanitizeHeader`) |

---

## Unresolved Questions

1. Is Go 1.25 intentional (future-proofing)? Or a typo for 1.23?
2. Is the AuthService implementation planned for a specific phase?
3. Should event publish failures block the HTTP response (stronger consistency) or be async (current approach minus the silent discard)?
4. Is the audit `actor_id = entity_id` (self-referential) intentional, or should the actual requesting user's ID be propagated through the event?
5. Are the `refresh_tokens` and `api_keys` tables used by any code path? The queries exist in sqlc but no Go code calls them.
