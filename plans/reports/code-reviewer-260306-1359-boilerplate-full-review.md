# Comprehensive Code Review: gnha-services Go Boilerplate

**Date:** 2026-03-06
**Reviewer:** code-reviewer agent
**Scope:** All 58 hand-written Go source files (~3,571 LOC) + infrastructure configs
**Overall Score: 8.3 / 10**

---

## Executive Summary

This is a well-architected Go modular monolith boilerplate. The hexagonal architecture is properly enforced, Fx DI wiring is clean, and the security posture is solid for a boilerplate. The codebase follows Go idioms, has proper error handling, and includes a scaffold tool that enforces consistent module structure. Main gaps: RBAC is too coarse for write/delete (relies on interceptor but route-level only checks `user:read`), test coverage is thin for app-layer error paths, and a few resource management edges exist.

---

## Findings by Category

### 1. CRITICAL Issues

**None.** No data-loss bugs, no exposed secrets, no injection vectors found.

### 2. HIGH Priority

#### H-1: RBAC Granularity Gap (MEDIUM-HIGH)
- **File:** `/internal/shared/middleware/rbac_interceptor.go:12-16`, `/internal/modules/user/adapters/grpc/routes.go:26`
- **Problem:** Route group applies only `PermUserRead`. The `RBACInterceptor` adds write/delete checks at the Connect RPC level, but this is a defense-in-depth mismatch. If the interceptor is accidentally removed or bypassed, all mutations become read-only protected.
- **Impact:** A user with only `user:read` permission could theoretically reach write endpoints if the interceptor is misconfigured.
- **Fix:** Apply distinct middleware per route group or enforce write/delete permissions redundantly at the Echo level.

#### H-2: Audit Module Creates Own sqlcgen.Queries Instance (Potential Conflict)
- **File:** `/internal/modules/audit/module.go:12-14`
- **Problem:** `fx.Provide(func(pool *pgxpool.Pool) *sqlcgen.Queries { return sqlcgen.New(pool) })` creates a module-scoped `*sqlcgen.Queries`. If another module also provides `*sqlcgen.Queries` to Fx, it will conflict. This should use a named/annotated dependency.
- **Impact:** Adding a second module that needs `*sqlcgen.Queries` will cause Fx to panic at startup.
- **Fix:** Use `fx.Annotate` with a name tag, e.g., `fx.ResultTags(\`name:"audit_queries"\`)`.

#### H-3: No Email Validation in Domain Entity
- **File:** `/internal/modules/user/domain/user.go:44`
- **Problem:** `NewUser` checks `email == ""` but does not validate email format. Protobuf validation catches this at the API layer, but direct calls to `NewUser` (e.g., seed, tests, future internal services) bypass protobuf validation.
- **Impact:** Invalid emails can be persisted if the domain is called outside the gRPC handler.
- **Fix:** Add basic email format validation (contains `@`, reasonable length) in `NewUser`.

### 3. MEDIUM Priority

#### M-1: create_user_test.go Missing Error-Path Tests
- **File:** `/internal/modules/user/app/create_user_test.go`
- **Problem:** Only tests success + email-taken. Missing: invalid role, hash failure, repo.Create failure, event publish failure paths.
- **Impact:** Low confidence in error handling correctness for the app layer.

#### M-2: ListUsers Returns Password Hashes Through Call Chain
- **File:** `/internal/modules/user/adapters/postgres/repository.go:187-198`
- **Problem:** `toDomain()` reconstitutes the full user including password hash. `ListUsers` returns `[]*domain.User` with password hashes available. The proto mapper (`toProto`) strips it, but the hash is available in memory throughout the call chain.
- **Impact:** Low risk since `toProto` correctly omits password. But a future mapper mistake could leak hashes.
- **Mitigation:** Consider a `UserSummary` projection for list operations that excludes password.

#### M-3: Cron Stop() Does Not Wait for Running Jobs
- **File:** `/internal/shared/cron/scheduler.go:62-64`
- **Problem:** `s.cron.Stop()` returns a channel (`<-chan struct{}`) that signals when running jobs complete, but the code ignores it. On shutdown, a running cron job may be terminated mid-execution.
- **Fix:** `ctx := s.cron.Stop(); <-ctx.Done()` or use the returned channel.

#### M-4: PII in Audit Trail Events
- **File:** `/internal/shared/events/topics.go:13-20`, `/internal/modules/audit/subscriber.go:52-60`
- **Problem:** `UserCreatedEvent` includes email and name. These are stored verbatim in `audit_logs.changes` JSONB. Under GDPR, this PII in audit logs complicates right-to-erasure requests.
- **Impact:** Compliance risk if deployed in EU jurisdiction.
- **Mitigation:** Store only entity IDs in audit trail; look up current data when needed.

#### M-5: Audit ip_address Always NULL
- **File:** `/internal/modules/audit/subscriber.go:54-60`, `/gen/sqlc/audit.sql.go:27`
- **Problem:** `CreateAuditLogParams.IpAddress` is never set. The audit_logs table has an `ip_address INET` column but no code populates it.
- **Impact:** Audit trail lacks client IP for forensic investigation.
- **Fix:** Propagate client IP through event metadata or message context.

#### M-6: UpdateUser Optional Field Validation Missing in Proto
- **File:** `/proto/user/v1/user.proto:57-61`
- **Problem:** `UpdateUserRequest.name` and `role` are `optional string` but lack validation constraints (no `min_len`, no `in` for role). A client can send `name: ""` or `role: "superadmin"`.
- **Impact:** Domain layer catches these, but protobuf validation should be the first line of defense for consistency with CreateUser.
- **Fix:** Add `[(buf.validate.field).string = {min_len: 1, max_len: 255}]` on name and role `in` constraint.

#### M-7: Rate Limiter Key Uses Unauthenticated IP Before Auth
- **File:** `/internal/shared/middleware/rate_limit.go:32-37`, `/internal/shared/middleware/chain.go:42`
- **Problem:** Rate limiting (position 9 in chain) runs before auth (applied at route group). So `rateLimitKey` always falls back to IP for all requests, even authenticated ones. The user-based rate limiting path is unreachable for global middleware.
- **Impact:** Authenticated users share IP-based limits; per-user limiting is effectively unused.
- **Fix:** Move rate limit after auth, or accept IP-based as intentional for global level.

### 4. LOW Priority

#### L-1: repository.go at 222 Lines
- **File:** `/internal/modules/user/adapters/postgres/repository.go`
- **Problem:** Slightly over the 200-line guideline from `development-rules.md`.
- **Fix:** Extract cursor helpers to a `cursor.go` file (saves ~22 lines).

#### L-2: Hardcoded Version "0.1.0" in OTel Resource
- **Files:** `/internal/shared/observability/tracer.go:36`, `/internal/shared/observability/metrics.go:33`
- **Problem:** `semconv.ServiceVersion("0.1.0")` is hardcoded. Should be injected via ldflags or config.
- **Fix:** Add `Version` field to config or use build-time injection.

#### L-3: Seed Passwords Are Weak
- **File:** `/cmd/seed/main.go:25-27`
- **Problem:** `Admin@123456` etc. are weak seed passwords. If seed accidentally runs in production, these are trivially guessable.
- **Fix:** Generate random passwords in production mode, or gate seed behind `IsDevelopment()`.

#### L-4: CORS AllowCredentials with Wildcard Risk
- **File:** `/internal/shared/middleware/chain.go:27-36`
- **Problem:** `AllowCredentials: true` with configurable origins. If `CORSOrigins` env is set to `*`, browsers will reject credentials. Not a bug currently (default is `localhost:3000`), but fragile.

#### L-5: Missing Content-Security-Policy Header
- **File:** `/internal/shared/middleware/security.go`
- **Problem:** Good security headers but no CSP. The Swagger UI page loads external CDN scripts without CSP restriction.
- **Fix:** Add CSP at minimum for non-Swagger routes.

#### L-6: No Pagination Index for Keyset Query
- **File:** `/db/migrations/00001_initial_schema.sql`
- **Problem:** The `ListUsers` query uses `ORDER BY created_at DESC, id DESC` with a keyset condition, but there is no composite index on `(created_at, id)`. The partial index `idx_users_email` and `idx_users_active` do not cover this query.
- **Fix:** Add `CREATE INDEX idx_users_pagination ON users (created_at DESC, id DESC) WHERE deleted_at IS NULL;`

---

## Architecture & Design Assessment (9/10)

**Strengths:**
- Clean hexagonal architecture: domain has zero external dependencies
- Fx module boundaries are crisp; each module is self-contained
- Closure-based `Update(ctx, id, func(*User) error)` pattern for transactional UoW is elegant
- Event bus with OTel trace propagation across AMQP messages
- Scaffold tool enforces identical structure for all future modules
- Proto-first API design with buf validation

**Weaknesses:**
- No login/auth endpoint (acknowledged as boilerplate gap)
- Single `Queries` instance for audit could conflict with future modules (H-2)

## Security Assessment (8/10)

**Strengths:**
- JWT signing method validation (prevents algorithm confusion attack)
- Argon2id with proper parameters (3 iterations, 64MB, 4 threads)
- Constant-time password comparison
- Token blacklist via Redis
- SMTP CRLF injection prevention
- XSS protection via `html.EscapeString` in Swagger builder
- X-Request-ID length limit (128 chars)
- SQL injection fully mitigated by sqlc parameterized queries
- Security headers: HSTS, X-Frame-Options DENY, X-Content-Type-Options nosniff

**Weaknesses:**
- RBAC only enforces `user:read` at route level; write/delete via interceptor only (H-1)
- No CSP header (L-5)
- Seed passwords weak (L-3)
- No password complexity validation in domain (proto has `min_len: 8` only)

## Code Quality Assessment (8.5/10)

**Strengths:**
- Consistent naming: Go idioms followed throughout
- Unexported domain fields with getters -- proper encapsulation
- Sentinel errors with typed `DomainError` mapped to HTTP + Connect codes
- Error wrapping with `fmt.Errorf("context: %w", err)` consistently
- No dead code (previous SanitizeHeader issue resolved)
- Clean Fx module composition

**Weaknesses:**
- Duplicate error-to-code mapping: `codeToHTTP` in `domain_error.go` and `codeToConnect` in `mapper.go` must stay in sync manually
- Three nearly identical audit handler methods (HandleUserCreated/Updated/Deleted) could use a generic helper

## Testing Assessment (6.5/10)

**Strengths:**
- Domain unit tests cover all validation paths
- Integration tests use testcontainers (real Postgres, Redis, RabbitMQ)
- Integration tests cover CRUD + pagination + soft delete
- Mock generation via `go generate` with uber-go/mock

**Weaknesses:**
- App layer: only 2 test cases for CreateUserHandler (M-1)
- No tests for UpdateUserHandler, DeleteUserHandler, GetUserHandler, ListUsersHandler
- No middleware tests (auth, rate limit, RBAC, error handler)
- No notification/audit subscriber tests
- No end-to-end/API integration tests

## Infrastructure Assessment (8.5/10)

**Strengths:**
- Multi-stage Dockerfile with minimal alpine runtime
- Docker healthcheck on liveness endpoint
- Compose with service health dependencies
- Traefik reverse proxy with Let's Encrypt auto-TLS
- GitLab CI: 4-stage pipeline (quality, test, build, deploy)
- Generated code staleness check in CI
- Taskfile with comprehensive DX commands
- Air hot reload properly configured
- Lefthook pre-commit (lint + generated check) + pre-push (tests)

**Weaknesses:**
- CI unit-test job references `coverage.xml` artifact but only generates `coverage.out` (cobertura report will be empty)
- No staging/production migration step in CI deploy
- Deploy uses SSH-based docker compose (works but fragile at scale)

## Graceful Shutdown Assessment (8/10)

**Strengths:**
- Echo server: proper `e.Shutdown(ctx)` on Fx OnStop
- Server error: uses `shutdowner.Shutdown(fx.ExitCode(1))` instead of `os.Exit(1)`
- DB: pool.Close() + rdb.Close() registered
- AMQP: publisher + subscriber closed on shutdown
- OTel: TracerProvider + MeterProvider shutdown
- Watermill router: context cancellation + router.Close()

**Weaknesses:**
- Cron Stop() does not drain running jobs (M-3)
- Shutdown order depends on Fx hook registration order; no explicit ordering guarantees

---

## Positive Observations

1. **Architecture discipline** -- domain package has zero imports from adapters or infrastructure
2. **Security-first defaults** -- JWT algorithm validation, Argon2id, CRLF prevention, header sanitization
3. **DX excellence** -- one-command setup (`task dev:setup`), scaffold tool, hot reload, pre-commit hooks
4. **Production-ready infra** -- healthchecks, Traefik TLS, 2-replica deploy, graceful shutdown chain
5. **Proto-first design** -- buf validation, generated OpenAPI, TypeScript types from proto
6. **Event-driven extensibility** -- Fx group tags for handler registration; new modules just provide handlers

---

## Recommended Actions (Prioritized)

1. **[HIGH]** Add composite index for pagination: `CREATE INDEX idx_users_pagination ON users (created_at DESC, id DESC) WHERE deleted_at IS NULL;` (L-6 -- marked low but will cause full table scans at scale)
2. **[HIGH]** Fix audit module Fx conflict potential -- annotate `*sqlcgen.Queries` with a name tag (H-2)
3. **[MEDIUM]** Add proto validation on UpdateUserRequest optional fields (M-6)
4. **[MEDIUM]** Add error-path tests for app layer handlers (M-1)
5. **[MEDIUM]** Wait for running cron jobs on shutdown (M-3)
6. **[MEDIUM]** Populate audit ip_address from request context (M-5)
7. **[LOW]** Extract cursor helpers from repository.go to reduce file length (L-1)
8. **[LOW]** Inject version string via ldflags (L-2)
9. **[LOW]** Gate seed command behind development mode (L-3)
10. **[LOW]** Fix CI coverage artifact format mismatch (coverage.xml not generated)

---

## Metrics

| Metric | Value |
|--------|-------|
| Total Go LOC (hand-written) | ~3,571 |
| Files reviewed | 58 source + 12 config |
| Domain unit tests | 11 test functions |
| App unit tests | 2 test functions |
| Integration tests | 5 test functions |
| Middleware tests | 0 |
| Linting config | golangci-lint with 11 linters |
| Test coverage (estimated) | ~30-40% (domain high, app/middleware low) |

---

## Unresolved Questions

1. Is the rate limiter intentionally IP-only at global level, or should it be per-user after auth?
2. Is the audit `ip_address` column intended for future use or should it be removed?
3. Should the seed command be restricted to development environments only?
4. Is GDPR compliance a requirement (affects PII in audit trail decision)?
