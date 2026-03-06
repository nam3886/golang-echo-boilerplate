# Full Boilerplate Code Review

**Date:** 2026-03-05
**Scope:** All 60 Go source files, SQL, Proto, Docker, CI/CD
**LOC:** ~4,685 (Go, excluding generated)
**Verdict:** Strong foundation with several critical gaps that must be fixed before production use.

---

## Overall Assessment

The boilerplate demonstrates solid architectural thinking: clean hexagonal boundaries, proper Fx DI wiring, idiomatic Go patterns (unexported domain fields, sentinel errors, closure-based UoW). The toolchain (Taskfile, lefthook, golangci-lint, Air, buf, sqlc) is well-chosen. However, the project has one showstopper (no auth service implementation), zero test files, and several security/correctness gaps that undermine its usefulness as a starter template.

**Grade: B-** -- Good bones, needs targeted fixes before teams should clone it.

---

## CRITICAL Issues (Must Fix)

### C-1: AuthService proto defined but NO Go implementation

**File:** `proto/auth/v1/auth.proto` defines Login, RefreshToken, Logout.
**File:** `gen/proto/auth/v1/authv1connect/auth.connect.go` generates the interface.
**Problem:** No `internal/modules/auth/` directory exists. No handler implements `AuthServiceHandler`. Users literally cannot authenticate. The JWT generation utilities exist (`internal/shared/auth/jwt.go`), the refresh_token and api_keys tables exist (`db/migrations/00002_auth_tables.sql`), the sqlc queries exist (`db/queries/auth.sql`) -- but nothing connects them.

**Impact:** The entire auth middleware (`internal/shared/middleware/auth.go`) requires a Bearer token, but there is no endpoint to obtain one. The boilerplate is non-functional for any authenticated operation.

**Fix:** Implement `internal/modules/auth/` with handler, routes, and Fx module. This is the single highest-priority item.

### C-2: CreateUser returns stale domain entity (wrong ID, wrong timestamps)

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user.go:52-58`
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go:98-114`

`domain.NewUser()` generates a UUID client-side (line 55 of `domain/user.go`), but `repo.Create()` uses `INSERT ... RETURNING *` where the DB generates its own UUID via `gen_random_uuid()`. The RETURNING row is discarded -- `Create()` returns `error` only. The handler returns the domain entity with the **wrong ID** and **wrong timestamps** (Go `time.Now()` vs DB `NOW()`).

**Fix:** Either:
- (a) Pass the domain-generated ID to the INSERT (add `id` column to CreateUserParams), or
- (b) Change `Create()` to return the DB-generated entity: `Create(ctx, user) (*domain.User, error)` and reconstitute from the RETURNING row.

Option (b) is cleaner since it keeps the DB as the source of truth for IDs and timestamps.

### C-3: No server-side password validation

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user.go:47`
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/domain/user.go:43-63`

`NewUser()` validates email, name, and role -- but never validates the password. An empty string `""` is happily hashed and stored. The proto validation (`min_len = 8`) only applies if the Connect interceptor is wired, but the seed command and any internal callers bypass it entirely.

**Fix:** Add `if len(hashedPassword) == 0` check in `NewUser()`, or better: add a `ValidateRawPassword(password string) error` function called in `CreateUserHandler.Handle()` before hashing. Enforce minimum length, complexity rules at the domain/app layer.

---

## HIGH Priority

### H-1: RBAC middleware defined but never applied

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/rbac.go` -- `RequirePermission()` and `RequireRole()` exist.
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/grpc/routes.go:22` -- only `Auth()` is applied.

Any authenticated user (viewer, member, admin) can create, update, and delete users. The RBAC middleware is dead code.

**Fix:** Apply RBAC to the route group or use Connect interceptors:
```go
g := e.Group(path, appmw.Auth(cfg, rdb), appmw.RequireRole("admin"))
```
Or implement fine-grained per-RPC interceptors.

### H-2: Zero test files

**File:** `internal/shared/testutil/` has excellent infrastructure (Postgres, Redis, RabbitMQ testcontainers, fixtures).
**Problem:** Zero `*_test.go` files in the entire repository. `go test ./internal/...` will report 0% coverage.

**Impact:** For a boilerplate, this sets a bad precedent. At minimum, provide:
- Domain entity unit tests (NewUser validation, ChangeName, ChangeRole)
- Repository integration tests using testcontainers
- Handler unit tests with mocked repo
- One end-to-end Connect RPC test

### H-3: Request ID from client not validated

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/request_id.go:19-21`

Client-supplied `X-Request-ID` is accepted verbatim with no length or content validation. An attacker could send a multi-megabyte header value that gets stored in logs, propagated to downstream services, and used as a context value.

**Fix:**
```go
if id == "" || len(id) > 128 || !isValidRequestID(id) {
    id = uuid.NewString()
}
```

### H-4: Audit ActorID always equals EntityID when no auth context

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/audit/subscriber.go:24-35`

`parseActorID()` falls back to `entityID` when `actorIDStr` is empty. This means audit logs show the user created/deleted themselves, losing the actual actor identity. For system operations (seed, cron), the actor should be a sentinel like `uuid.Nil` or a dedicated "system" UUID.

### H-5: Email uniqueness check is TOCTOU race

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user.go:39-45`

The handler checks `GetByEmail` then calls `Create`. Between these calls, another request could insert the same email. The repository's `Create()` does handle the PG unique violation (`23505`) at line 108 of `repository.go`, but the app-layer check returns `ErrEmailTaken` from the wrong path -- meaning the error message/code may differ depending on timing.

**Fix:** Remove the app-layer `GetByEmail` check entirely. Rely solely on the DB constraint + PG error mapping in `repository.go:107-109`. This is both correct and faster (one query instead of two).

### H-6: OTel WithInsecure in production

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/observability/tracer.go:22`
**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/observability/metrics.go:20`

`otlptracegrpc.WithInsecure()` is hardcoded. In production, trace data (which may include PII from spans) is sent unencrypted.

**Fix:** Conditionally apply `WithInsecure()` based on `cfg.IsDevelopment()` or add a `OTEL_INSECURE` config flag.

---

## MEDIUM Priority

### M-1: Postgres/Redis connection pools not shut down via Fx lifecycle

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/module.go`

The `shared.Module` provides `NewPostgresPool` and `NewRedisClient` but never registers `OnStop` hooks to close them. When Fx shuts down, connections are leaked. The pgxpool and redis client are GC'd eventually but won't drain gracefully.

**Fix:** Add:
```go
fx.Invoke(func(lc fx.Lifecycle, pool *pgxpool.Pool) {
    lc.Append(fx.Hook{OnStop: func(_ context.Context) error { pool.Close(); return nil }})
})
```

### M-2: Event publish errors are fire-and-forget

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user.go:73-76`

Event publish failures are logged but silently swallowed. The user gets a success response, but audit/notification never fires. For a boilerplate, this is an acceptable tradeoff (document it), but consider an outbox pattern for production.

### M-3: Watermill router uses `context.Background()` ignoring Fx shutdown

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/events/subscriber.go:59`

`router.Run(context.Background())` ignores the Fx lifecycle context. While `router.Close()` is called in OnStop, using `context.Background()` means the router won't respect the Fx shutdown timeout.

### M-4: ListUsers query returns password hashes

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/db/queries/user.sql:11` (`SELECT *`)

All user queries use `SELECT *` which includes the `password` column. While the `toProto()` mapper doesn't expose it, the password hash travels through the entire call chain unnecessarily. Use explicit column lists excluding `password` for read queries.

### M-5: Cron scheduler has no registered jobs

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/cron/scheduler.go`

The scheduler starts but has zero jobs. The `AddJob()` method is never called. Consider adding at least the obvious one: `DeleteExpiredRefreshTokens` (the query exists in `db/queries/auth.sql:15`).

### M-6: `model.BaseModel` is unused

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/model/base.go`

This struct exists but nothing references it. The domain entity uses its own fields, sqlc generates its own models. Dead code.

### M-7: Audit subscriber does DRY violation

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/audit/subscriber.go`

`HandleUserCreated`, `HandleUserUpdated`, `HandleUserDeleted` are nearly identical (unmarshal, parse entityID, insert audit log). Extract a generic handler:
```go
func (h *Handler) handleEvent(msg *message.Message, action string, extractIDs func([]byte) (string, string, error)) error { ... }
```

### M-8: Swagger UI hardcodes user.swagger.json

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/swagger.go:30`

Only `user/v1/user.swagger.json` is loaded. When the auth service is implemented, its spec won't appear. Use a spec list or combined spec.

---

## LOW Priority

### L-1: `UserID` type is `string` not `uuid.UUID`

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/domain/user.go:10`

Using `type UserID string` means every repository call must parse it back to `uuid.UUID`. Consider `type UserID uuid.UUID` to avoid repeated parsing.

### L-2: Inconsistent error wrapping patterns

Some errors use `fmt.Errorf("...: %w", err)`, others return sentinel errors directly. The wrapped errors lose the sentinel identity when using `errors.Is()`. Be consistent: wrap for context in adapters, return sentinels from domain.

### L-3: `deploy/docker-compose.dev.yml` includes Elasticsearch but no code uses it

The `ESURL` config field exists but nothing connects to Elasticsearch. Remove from dev compose or add a TODO comment.

### L-4: Hardcoded version `"0.1.0"` in OTel resource

**Files:** `tracer.go:33`, `metrics.go:31`

Extract to a build-time variable via `-ldflags`.

### L-5: `.env.example` JWT_SECRET is only 42 chars

The example value `change-me-in-production-use-a-strong-secret` is 45 chars and passes the 32-char check, but it's a weak secret. Add a comment: `# Generate with: openssl rand -base64 48`.

---

## Positive Observations

1. **Hexagonal architecture is clean.** Domain has no infrastructure imports. Ports (repository interface) are in domain package. Adapters are in dedicated packages. Textbook.

2. **Fx DI is well-structured.** Module boundaries are clean. The `fx.Annotate` + `fx.As` pattern for interface binding is correct. Event handler group injection via `group:"event_handlers"` is elegant.

3. **Password hashing uses argon2id** with constant-time comparison. Parameters are reasonable (64MB memory, 3 iterations). This is best-practice.

4. **JWT validation checks signing method** to prevent algorithm confusion attacks (`jwt.go:41-43`).

5. **Token blacklisting via Redis** is implemented in auth middleware. Ready for logout functionality.

6. **Cursor-based pagination** with keyset (created_at, id) is correct and performant. No OFFSET.

7. **Sliding window rate limiter** using Redis sorted sets is production-quality.

8. **Distributed cron locking** with Lua-script-verified unlock prevents lock theft.

9. **CRLF injection prevention** in SMTP sender sanitizes both headers and envelope recipients.

10. **Centralized error handling** with domain-to-HTTP and domain-to-Connect code mapping is clean and consistent.

11. **Security headers** (HSTS, X-Frame-Options, CSP-adjacent) are applied globally.

12. **Testcontainer utilities** are well-written with proper `t.Cleanup` patterns.

13. **CI/CD pipeline** covers lint, generated code check, unit tests, integration tests, build, and deployment with manual production gate.

14. **Proto validation annotations** with `buf.validate` and `connectrpc.com/validate` interceptor.

15. **SELECT FOR UPDATE** in the Update repository method prevents lost updates.

---

## Metrics

| Metric | Value |
|--------|-------|
| Go files (non-generated) | 41 |
| Generated Go files | 7 |
| Total Go LOC | ~4,685 |
| Test files | 0 |
| Test coverage | 0% |
| SQL migrations | 2 |
| Proto files | 2 |
| Linter config | golangci-lint with 11 linters |
| CI/CD stages | 4 (quality, test, build, deploy) |

---

## Recommended Action Plan (Priority Order)

1. **[CRITICAL] Implement AuthService** -- Login, RefreshToken, Logout handlers + routes + Fx module
2. **[CRITICAL] Fix CreateUser returning wrong ID/timestamps** -- use RETURNING row or pass domain ID to INSERT
3. **[CRITICAL] Add password validation** in domain/app layer
4. **[HIGH] Apply RBAC middleware** to user routes
5. **[HIGH] Add foundational tests** -- domain unit tests, repository integration tests, one E2E test
6. **[HIGH] Validate X-Request-ID** length and content
7. **[HIGH] Remove TOCTOU email check** -- rely on DB constraint only
8. **[MEDIUM] Register Fx shutdown hooks** for pgxpool and Redis
9. **[MEDIUM] Conditionally disable OTel insecure** mode based on environment
10. **[MEDIUM] Add expired refresh token cleanup** cron job
11. **[LOW] Remove dead code** (model.BaseModel, unused ES config)

---

## Unresolved Questions

1. Is the intent to use this boilerplate for a single project (gnha-services) or as a general-purpose Go template? This affects how prescriptive the auth implementation should be.
2. Should API key authentication be supported alongside JWT? The infrastructure exists (tables, hash utilities) but no middleware or handler uses it.
3. The `UpdateUser` event is published after the transaction commits, but the `updated` domain entity pointer is captured inside the closure. Is there a race concern if the closure's `user` pointer is later mutated? (Currently safe since nothing mutates it after, but fragile.)
4. Should the notification welcome email template be externalized (filesystem/config) rather than hardcoded?
