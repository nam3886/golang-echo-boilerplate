# Full Codebase Review -- GNHA Services Go Boilerplate

**Date:** 2026-03-06
**Reviewer:** code-reviewer
**Scope:** All 47 hand-written Go files under `internal/` and `cmd/`
**Build status:** Clean (vet + build pass)

---

## File-by-File Review

### cmd/server/main.go
**Purpose:** Application entry point, Fx wiring, Echo setup, health probes
**Score:** 8/10
**Issues:**
- (RESOLVED from previous) Health probes now check DB + Redis -- good
- Minor: `os.Exit(1)` in goroutine (line 79) bypasses Fx shutdown. If Echo fails to bind the port, resources leak before exit. Prefer signaling Fx to stop instead.
- Pool dependency in `newEcho` is only for readiness check -- clean enough.

### internal/shared/module.go
**Purpose:** Aggregates shared infrastructure Fx providers + lifecycle hooks
**Score:** 9/10
**Issues:**
- (RESOLVED) DB + Redis shutdown hooks now registered
- (RESOLVED) OTel providers flush on shutdown
- Minor: `mp.Shutdown` error is silently discarded (line 33). Should log.

### internal/shared/config/config.go
**Purpose:** Env-based config loading with validation
**Score:** 9/10
**Issues:** None significant. JWT secret length check is good. CORS, SMTP defaults are sensible.

### internal/shared/database/postgres.go
**Purpose:** Postgres pool creation with exponential backoff retry
**Score:** 8/10
**Issues:**
- Minor: Retry uses linear backoff (`(i+1)*second`), comment says "retry logic" but not exponential. Acceptable for startup.
- Pool params (MaxConns=25, MinConns=5) are hardcoded. Consider making configurable for production tuning, but YAGNI for a boilerplate.

### internal/shared/database/redis.go
**Purpose:** Redis client creation with retry
**Score:** 8/10
**Issues:**
- Same linear backoff pattern as postgres -- consistent, fine.
- `PoolSize = 10 * runtime.NumCPU()` could be large on beefy machines. Acceptable.

### internal/shared/observability/logger.go
**Purpose:** Structured slog logger (text for dev, JSON for prod)
**Score:** 9/10
**Issues:** None. Clean, does what it should.

### internal/shared/observability/tracer.go
**Purpose:** OTel tracer provider with OTLP gRPC export
**Score:** 9/10
**Issues:**
- (RESOLVED from H-5) `WithInsecure()` now only in development. Good.
- Hardcoded `ServiceVersion("0.1.0")` -- should come from build-time variable eventually.

### internal/shared/observability/metrics.go
**Purpose:** OTel meter provider with OTLP gRPC export
**Score:** 9/10
**Issues:** Same `ServiceVersion` hardcode as tracer. Otherwise clean.

### internal/shared/auth/password.go
**Purpose:** Argon2id password hashing + verification
**Score:** 9/10
**Issues:**
- Params (time=3, memory=64K, threads=4, keyLen=32) are OWASP-recommended. Good.
- Constant-time comparison via `subtle.ConstantTimeCompare`. Good.
- Reads stored params from the hash for verification (forward-compatible). Good.

### internal/shared/auth/jwt.go
**Purpose:** JWT access/refresh token generation + validation
**Score:** 8/10
**Issues:**
- HMAC algorithm enforcement in `ParseWithClaims` keyfunc. Good.
- `GenerateRefreshToken` generates random bytes but there is no storage/rotation mechanism. Expected for boilerplate stage.
- No `Issuer` claim set in tokens. Minor but could help with multi-service setups.

### internal/shared/auth/context.go
**Purpose:** Auth user context injection/extraction
**Score:** 9/10
**Issues:**
- `HasPermission` grants all to `admin` role OR `admin:*` permission. Dual-path is slightly confusing but functional.
- `UserFromContext` returns nil for unauthenticated -- callers must nil-check. Pattern is consistent across codebase.

### internal/shared/errors/domain_error.go
**Purpose:** Domain error type with HTTP status mapping
**Score:** 9/10
**Issues:**
- Clean sentinel pattern with `errors.As` support.
- `Unwrap()` support for error chain traversal. Good.
- Sentinel errors are pointer-based -- `errors.Is` works on identity comparison. This is correct.

### internal/shared/events/bus.go
**Purpose:** EventBus wrapping Watermill AMQP publisher with OTel propagation
**Score:** 8/10
**Issues:**
- OTel trace propagation via message metadata. Good.
- `NewPublisher`/`NewSubscriber` use `NewDurableQueueConfig` -- durable by default. Good.
- No publisher `Close()` lifecycle hook. The publisher is not shut down explicitly. Watermill publisher should be closed on shutdown to flush pending messages. **HIGH**

### internal/shared/events/module.go
**Purpose:** Fx module for event infrastructure
**Score:** 9/10
**Issues:** Clean.

### internal/shared/events/subscriber.go
**Purpose:** Watermill router with retry + recovery middleware, Fx lifecycle
**Score:** 7/10
**Issues:**
- **MEDIUM (M-3 persists):** `router.Run(context.Background())` on line 59 -- when Fx signals shutdown, `router.Close()` is called, but the Run goroutine uses a background context that ignores Fx's shutdown context. This works because `Close()` stops the router, but it's not clean. The router should ideally use a cancellable context.
- Handler registration via `group:"event_handlers"` tag is elegant.
- Retry middleware (3 retries, 1s initial). Good default.

### internal/shared/events/topics.go
**Purpose:** Event topic constants + event structs
**Score:** 9/10
**Issues:** Clean event definitions with proper JSON tags.

### internal/shared/cron/scheduler.go
**Purpose:** Cron scheduler with Redis distributed locking
**Score:** 8/10
**Issues:**
- Lua unlock script verifies ownership before delete. Good.
- `SetNX` with 5-min TTL for lock. Good.
- Jobs use `context.Background()` -- no timeout on job execution. Could run indefinitely. Minor for boilerplate.

### internal/shared/cron/module.go
**Purpose:** Fx module for cron
**Score:** 9/10
**Issues:** Clean. (M-5: scheduler starts with zero jobs is by-design for boilerplate)

### internal/shared/middleware/chain.go
**Purpose:** Middleware chain setup in correct order
**Score:** 9/10
**Issues:**
- Order is correct: Recovery > RequestID > Logger > BodyLimit > Gzip > Security > CORS > Timeout > RateLimit.
- Auth + RBAC at route-group level (noted in comment). Good separation.
- `BodyLimit("10M")` is reasonable.

### internal/shared/middleware/auth.go
**Purpose:** JWT Bearer auth middleware with blacklist check
**Score:** 8/10
**Issues:**
- Blacklist check via Redis `Exists`. Good.
- Swallows Redis error on blacklist check (line 29 `blacklisted, _ :=`). If Redis is down, blacklisted tokens pass through. This is a **fail-open** choice -- acceptable for availability but should be documented.
- `extractBearerToken` uses `strings.EqualFold` for "bearer " prefix. Good case-insensitive handling.

### internal/shared/middleware/rbac.go
**Purpose:** Permission + role-based access control middleware
**Score:** 9/10
**Issues:**
- **H-1 persists:** RBAC middleware is defined but NOT applied to any routes. `routes.go` only applies `Auth`, not `RequirePermission` or `RequireRole`. Any authenticated user can CRUD all users. This remains the single biggest authorization gap.

### internal/shared/middleware/rate_limit.go
**Purpose:** Redis sliding-window rate limiter
**Score:** 8/10
**Issues:**
- Sliding window via sorted set. Correct algorithm.
- Fail-open on Redis error. Consistent with auth blacklist philosophy.
- Uses pipeline for atomic operations. Good.
- `Retry-After` header set on 429. Good.

### internal/shared/middleware/recovery.go
**Purpose:** Panic recovery with stack trace logging
**Score:** 9/10
**Issues:** Clean. Logs stack + path. Returns 500.

### internal/shared/middleware/request_id.go
**Purpose:** X-Request-ID header generation/propagation
**Score:** 7/10
**Issues:**
- **M-1 persists:** Client-supplied `X-Request-ID` is not validated for length or content. An attacker could send a multi-megabyte string. Add `len(id) > 128` check or similar.

### internal/shared/middleware/request_log.go
**Purpose:** Request logger with sensitive header redaction
**Score:** 8/10
**Issues:**
- `SanitizeHeader` function defined but never called in the logging flow. The logger doesn't log headers at all, so the function is dead code. **LOW**
- Log level based on status code (500=Error, 400=Warn, else=Info). Good pattern.

### internal/shared/middleware/security.go
**Purpose:** Security response headers
**Score:** 9/10
**Issues:**
- Good set: nosniff, DENY frame, XSS, HSTS, referrer, permissions-policy.
- No CSP header -- acceptable for API-only service.

### internal/shared/middleware/error_handler.go
**Purpose:** Centralized error handler mapping DomainError to HTTP
**Score:** 9/10
**Issues:**
- `errors.As` for both DomainError and echo.HTTPError. Good chain.
- Unhandled errors logged + generic 500. Good.
- Response committed check. Good.

### internal/shared/middleware/swagger.go
**Purpose:** Swagger UI with auto-discovery of OpenAPI specs
**Score:** 7/10
**Issues:**
- **MEDIUM:** XSS risk in `buildSwaggerHTML` -- spec file names are interpolated directly into JS string without escaping (line 57-58). If a malicious `.swagger.json` filename contained `"`, it would break out of the JS string. Low practical risk since filenames are generated, but unsanitary.
- Production guard (`cfg.AppEnv == "production"`) should also consider staging. Currently dev + staging both serve swagger, which is fine for boilerplate.

### internal/shared/testutil/*.go
**Purpose:** Test infrastructure (containers, migrations, fixtures)
**Score:** 9/10
**Issues:**
- Testcontainers for Postgres, Redis, RabbitMQ. Good integration test setup.
- `RunMigrations` manually parses goose Up sections -- fragile but functional.
- Fixtures are simple value objects. Clean.

### internal/shared/mocks/mock_user_repository.go
**Purpose:** MockGen-generated mock
**Score:** N/A (generated)
**Issues:** None -- correctly generated.

### internal/modules/user/module.go
**Purpose:** Fx module wiring for user domain
**Score:** 9/10
**Issues:**
- `fx.As(new(domain.UserRepository))` correctly abstracts the postgres impl. Good.
- All app handlers + grpc adapter registered. Clean.

### internal/modules/user/domain/user.go
**Purpose:** User domain entity with encapsulated fields
**Score:** 9/10
**Issues:**
- Unexported fields + getters + `Reconstitute` pattern. Textbook DDD.
- `NewUser` generates UUID in domain layer. **C-2 from previous review is NOW RESOLVED** -- the Create repo method passes the domain-generated UUID to DB, and integration test confirms `got.ID() != user.ID()` would fail.
- `ChangeName`/`ChangeRole` update `updatedAt`. Good.

### internal/modules/user/domain/errors.go
**Purpose:** Module-specific domain error sentinels
**Score:** 10/10
**Issues:** None. Clean, uses shared error infrastructure correctly.

### internal/modules/user/domain/repository.go
**Purpose:** Repository port interface
**Score:** 9/10
**Issues:**
- `Update` with closure pattern (`fn func(*User) error`) is elegant for transactional UoW.
- `List` returns `(users, cursor, hasMore, error)` -- slightly wide return. Acceptable.

### internal/modules/user/app/create_user.go
**Purpose:** Create user use case
**Score:** 8/10
**Issues:**
- Double email uniqueness check (app layer + DB constraint). Belt and suspenders. Good.
- Event published after DB write. Good ordering.
- Event publish failure is logged, not propagated. Correct -- user creation should not fail because of event bus.
- **C-2 RESOLVED:** The returned `user` has the domain-generated UUID, and `repo.Create` inserts with that UUID. DB `gen_random_uuid()` is not used -- the INSERT passes the Go-side UUID.

### internal/modules/user/app/get_user.go
**Purpose:** Get user by ID
**Score:** 9/10
**Issues:** Minimal, correct delegation.

### internal/modules/user/app/update_user.go
**Purpose:** Partial user update with closure UoW
**Score:** 8/10
**Issues:**
- Uses `repo.Update` with closure that mutates domain entity inside transaction. Good.
- Event published after successful update. Good.
- `updated` variable captured from closure -- works because closure runs synchronously before return.

### internal/modules/user/app/delete_user.go
**Purpose:** Soft delete use case
**Score:** 9/10
**Issues:** Clean. Event after delete. Actor extraction consistent.

### internal/modules/user/app/list_users.go
**Purpose:** Paginated user listing
**Score:** 9/10
**Issues:**
- Limit clamped to [1, 100] with default 20. Good.
- **M-4 persists:** `List` returns full `domain.User` objects including `Password()` getter. The gRPC mapper (`toProto`) does NOT map password to the response proto, so password hashes never reach the wire. However, the hash traverses the full call chain (DB -> repo -> app -> grpc handler). A projection/DTO would be cleaner, but the proto mapper acts as the boundary. Acceptable for boilerplate.

### internal/modules/user/adapters/grpc/handler.go
**Purpose:** Connect RPC handler delegating to app layer
**Score:** 9/10
**Issues:**
- Interface compliance check (`var _ userv1connect.UserServiceHandler`). Good.
- Clean delegation, no business logic in adapter.

### internal/modules/user/adapters/grpc/mapper.go
**Purpose:** Domain-to-proto mapping + error code translation
**Score:** 9/10
**Issues:**
- `toProto` does NOT expose password. Good.
- Error code map covers all `ErrorCode` values. Complete.
- Unmapped codes fall through to `connect.CodeInternal`. Safe default.

### internal/modules/user/adapters/grpc/routes.go
**Purpose:** Connect RPC route registration with auth + validation
**Score:** 7/10
**Issues:**
- `validate.NewInterceptor()` enforces buf.validate rules. Good.
- **H-1 persists:** Only `Auth` middleware applied. No RBAC. Any authenticated user (including `viewer` role) can create/update/delete users.
- Route mounting via `echo.WrapHandler(http.StripPrefix(...))` is the standard pattern.

### internal/modules/user/adapters/postgres/repository.go
**Purpose:** pgx+sqlc repository implementation
**Score:** 8/10
**Issues:**
- `GetByIDForUpdate` uses `SELECT ... FOR UPDATE` in transaction. Good.
- Unique constraint violation (23505) mapped to `ErrEmailTaken`. Good.
- `SoftDelete` checks `rows == 0` for not-found. Good.
- `toDomain` converts sqlc row to domain entity via `Reconstitute`. Good.
- Cursor pagination with base64 JSON encoding. Correct implementation.
- `parseUserID` uses `uuid.Parse` (not `MustParse`). Good.
- File is 222 lines -- slightly over the 200-line guideline. Could extract cursor helpers to separate file. **LOW**

### internal/modules/user/adapters/postgres/repository_test.go
**Purpose:** Integration tests for postgres repo
**Score:** 9/10
**Issues:**
- Build tag `integration` -- won't run in default `go test`. Good separation.
- Tests cover: Create, DuplicateEmail, GetByID NotFound, SoftDelete, List Pagination. Good coverage.

### internal/modules/user/domain/user_test.go
**Purpose:** Unit tests for domain entity
**Score:** 9/10
**Issues:**
- Tests: NewUser (success, invalid email/name/role), ChangeName (success, empty), ChangeRole (success, invalid), Role.IsValid. Good coverage of domain invariants.

### internal/modules/user/app/create_user_test.go
**Purpose:** Unit tests for create user handler
**Score:** 8/10
**Issues:**
- Tests: Success, EmailTaken. Good.
- `stubHasher` returns `"hashed_" + password`. Fine for unit tests.
- `noopPublisher` discards events. Good isolation.
- Missing test: CreateUser with invalid role, empty password, repo.Create failure, hash failure. **MEDIUM**

### internal/modules/audit/module.go + subscriber.go
**Purpose:** Audit trail via event subscription
**Score:** 8/10
**Issues:**
- `msg.Context()` used for DB operations. Good.
- `parseActorID` falls back to entityID for system operations. (M-2 acknowledged)
- Invalid UUID in event payload returns `nil` (ack) -- correct, retrying won't fix bad data.
- Three handlers with near-identical structure. Could DRY with a generic handler, but YAGNI at 3 events.

### internal/modules/notification/module.go + subscriber.go + sender.go + email.go
**Purpose:** Email notifications via SMTP
**Score:** 8/10
**Issues:**
- CRLF injection sanitization in `SMTPSender.Send`. Good.
- Q-encoding for Subject header. Good.
- `html/template` used (not `text/template`) -- auto-escapes HTML in template vars. Good XSS prevention.
- `msg.Context()` used. Good.
- Only `user.created` triggers notification. Appropriate for boilerplate.

---

## Summary Table

| Severity | Count | Description |
|----------|-------|-------------|
| CRITICAL | 0 | None remaining |
| HIGH | 2 | H-1: RBAC not applied to routes; Publisher not closed on shutdown |
| MEDIUM | 5 | M-1: Request ID unvalidated; M-3: Router uses background ctx; M-4: Password hash in call chain; Swagger XSS; Incomplete test coverage for create_user |
| LOW | 3 | SanitizeHeader dead code; repo.go slightly over 200 lines; os.Exit in goroutine |

---

## Architecture Assessment

**Score: 9/10**

Hexagonal architecture is consistently and correctly applied:
- **Domain layer** (`domain/`): Pure Go, no external dependencies, encapsulated fields, business invariants enforced in constructors and mutation methods.
- **Application layer** (`app/`): Use cases depend only on domain interfaces. Event publishing is a side effect, not a hard dependency.
- **Adapter layer** (`adapters/grpc/`, `adapters/postgres/`): Correctly bridges external concerns (proto, pgx/sqlc) to domain types via mappers and `Reconstitute`.
- **Shared infrastructure** (`shared/`): Cross-cutting concerns (auth, config, events, middleware) are well-organized and decoupled via interfaces.

The `Reconstitute` + unexported fields pattern is textbook DDD. The closure-based `Update` for transactional UoW is elegant. Module boundaries are clean -- no import cycles detected.

---

## Security Assessment

**Score: 7.5/10**

**Strengths:**
- Argon2id with OWASP params + constant-time comparison
- JWT HMAC algorithm enforcement
- Token blacklist support
- Security headers (HSTS, nosniff, X-Frame-Options, etc.)
- CRLF injection prevention in SMTP
- Input validation via buf.validate interceptor
- CORS with explicit origins
- Rate limiting with sliding window
- Body size limit (10M)
- Password never exposed in proto responses

**Weaknesses:**
- **H-1: No RBAC enforcement on routes** -- the biggest gap. Any authenticated user can perform any operation.
- Auth blacklist check fails open on Redis error (documented design choice, but risky)
- Request ID header not length-validated (DoS vector, though mitigated by body limit)
- No auth service implementation -- users cannot actually log in (JWT generation functions exist but no login endpoint)

---

## DX Assessment (Developer Experience)

**Score: 8/10**

**Strengths:**
- Clear file naming and organization
- Consistent patterns across modules
- MockGen for repository mocks
- Testcontainers for integration tests
- Swagger UI with auto-discovery
- Fx dependency injection with clear module boundaries
- go:generate directives for code gen

**Weaknesses:**
- No `make` or `task` shortcut for running integration tests with the `integration` build tag
- No example `.env` file in project root (config requires DATABASE_URL, REDIS_URL, RABBITMQ_URL, JWT_SECRET)
- Adding a new module requires understanding the Fx `group:"event_handlers"` pattern (could use docs guidance)
- Auth service gap means the boilerplate cannot be tested end-to-end without manually creating JWTs

---

## Previously Reported Issues -- Status Update

| ID | Status | Notes |
|----|--------|-------|
| C-1 AuthService not implemented | **CONFIRMED ABSENT** | No auth proto, no login endpoint. But re-classified: this is a boilerplate gap, not a bug. The auth *middleware* works; there's just no login flow. |
| C-2 CreateUser returns stale entity | **RESOLVED** | Domain generates UUID, repo inserts with that UUID. Integration test confirms ID match. |
| C-3 Health probes are stubs | **RESOLVED** | `/readyz` now checks DB + Redis. |
| H-1 No RBAC on routes | **STILL OPEN** | Only `Auth` middleware applied in `routes.go`. |
| H-2 Zero test files | **RESOLVED** | 3 test files now exist: domain/user_test.go, app/create_user_test.go, adapters/postgres/repository_test.go |
| H-4 Postgres/Redis not shut down | **RESOLVED** | `registerDBShutdown` in shared/module.go |
| H-5 OTel WithInsecure() hardcoded | **RESOLVED** | Now conditional on `IsDevelopment()` |
| M-6 BaseModel dead code | **RESOLVED** | Removed |
| M-7 Swagger hardcodes one spec | **RESOLVED** | Auto-discovers all .swagger.json |

---

## Overall Score

| Category | Score |
|----------|-------|
| Architecture | 9/10 |
| Security | 7.5/10 |
| Code Quality | 8.5/10 |
| Error Handling | 8.5/10 |
| Observability | 8/10 |
| Testing | 6/10 |
| DX | 8/10 |
| **Overall** | **7.9/10** |

---

## Recommended Actions (Priority Order)

1. **Apply RBAC to user routes** (H-1) -- Add `RequirePermission(PermUserWrite)` to create/update/delete, `RequirePermission(PermUserRead)` to get/list. This is a 5-line change in `routes.go`.

2. **Close Watermill publisher on shutdown** -- Add `fx.Invoke(registerPublisherShutdown)` or use Fx lifecycle to call `publisher.Close()`. Without this, pending messages may be lost on graceful shutdown.

3. **Validate X-Request-ID length** (M-1) -- Add `if len(id) > 128 { id = uuid.NewString() }` in request_id.go.

4. **Add auth/login endpoint** -- Without this, the boilerplate is not usable end-to-end. At minimum, a `POST /auth/login` that accepts email+password and returns JWT.

5. **Expand test coverage** -- add error-path tests for create_user_test.go (invalid role, hash failure, repo failure). Add unit tests for update/delete/list handlers.

6. **Remove dead `SanitizeHeader` function** or wire it into the logger.

---

## Unresolved Questions

1. Is the lack of a login endpoint intentional (boilerplate = "bring your own auth flow"), or is it a gap that should be filled before the boilerplate is considered complete?
2. Should the fail-open behavior on Redis errors (auth blacklist, rate limiting) be configurable per-environment?
3. Is the password hash flowing through the full call chain (M-4) acceptable long-term, or should a password-less projection be introduced at the repository level?
