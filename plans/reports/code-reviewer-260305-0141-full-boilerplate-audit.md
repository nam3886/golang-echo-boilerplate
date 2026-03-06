# Boilerplate Production-Readiness Audit

**Date:** 2026-03-05
**Build status:** `go build ./...` PASS, `go vet ./...` PASS
**Files audited:** 48 non-generated Go files + infra configs
**Codebase:** ~4,685 Go LOC (41 non-generated files)

---

## Summary Scorecard

| # | Area | Grade | One-line verdict |
|---|------|-------|------------------|
| 1 | Project Structure & Module Pattern | **A** | Consistent hexagonal layers; mechanically copyable |
| 2 | Error Handling Consistency | **A** | Full chain: domain -> Connect RPC -> HTTP; no gaps |
| 3 | Auth & Security | **C** | JWT utilities exist but AuthService has NO Go implementation; RBAC defined but never applied |
| 4 | Event System | **A-** | Consistent publish pattern across all mutations; minor audit actor fallback issue |
| 5 | Database & Repository Pattern | **B** | Solid patterns but CreateUser returns stale entity; SELECT * leaks password hashes |
| 6 | Config & Infrastructure | **A-** | Robust config with validation; missing Fx lifecycle hooks for pool shutdown |
| 7 | Observability | **B** | OTel setup complete; WithInsecure() hardcoded; logger provided but not injected into app layer |
| 8 | Testing Infrastructure | **F** | Testcontainer helpers exist; ZERO test files |
| 9 | Documentation | **B+** | Accurate and thorough; adding-a-module has inconsistencies with actual code patterns |
| 10 | DX (Developer Experience) | **A-** | Excellent Taskfile; lefthook wired; missing monitor compose file |
| 11 | Health Checks & Graceful Shutdown | **C** | /healthz is a dummy OK; no dependency checks; pools not shut down via Fx |
| 12 | Dead Code & Inconsistencies | **B** | model.BaseModel unused; cron scheduler starts empty; Swagger hardcoded to user only |

---

## Detailed Findings

### 1. Project Structure & Module Pattern (Grade: A)

**Evidence:** Consistent hexagonal layers across the user module:
- `domain/` = entity + repository interface + errors
- `app/` = one handler per use case (create, get, list, update, delete)
- `adapters/postgres/` = sqlc-backed repository
- `adapters/grpc/` = handler + mapper + routes
- `module.go` = Fx wiring

**What works for new devs:**
- Copy `internal/modules/user/` -> `internal/modules/product/`, rename, and the structure is immediately clear
- Fx module pattern is consistent: `fx.Provide` for dependencies, `fx.Invoke` for route registration
- `docs/adding-a-module.md` provides step-by-step instructions

**What will confuse new devs:**
- The `adding-a-module.md` example uses exported domain fields (`ID uuid.UUID`) while the actual user module uses unexported fields + getters + `Reconstitute()`. The doc example teaches a DIFFERENT pattern than what exists in code. See [Section 9](#9-documentation-grade-b) for details.

### 2. Error Handling Consistency (Grade: A)

**Chain is complete and consistent:**

1. **Domain errors** defined in `internal/shared/errors/domain_error.go` (lines 9-17) with typed `ErrorCode`
2. **Module-specific errors** in `internal/modules/user/domain/errors.go` using `sharederr.New()`
3. **Connect RPC mapping** in `internal/modules/user/adapters/grpc/mapper.go` (lines 26-44) via `domainErrorToConnect()`
4. **HTTP mapping** in `internal/shared/middleware/error_handler.go` via `ErrorHandler`
5. **Error codes documented** in `docs/error-codes.md`

**Strengths:**
- All error codes map to both HTTP and Connect RPC codes consistently
- Sentinel errors (`ErrNotFound`, `ErrUnauthorized`, etc.) used throughout
- `errors.As()` / `errors.Is()` used correctly
- Unknown errors fall through to generic 500 (safe default)

**One gap:** `domainErrorToConnect()` falls through to `connect.CodeInternal` for wrapped errors where the inner `DomainError` is not the top-level type. Example: `fmt.Errorf("creating user: %w", err)` in `create_user.go:58` wraps the domain error. `errors.As()` will still find it, so this is actually fine. No issue here.

### 3. Auth & Security (Grade: C)

**CRITICAL: AuthService proto defined with NO Go implementation.**

File `proto/auth/v1/auth.proto` defines `Login`, `RefreshToken`, `Logout` RPCs. Generated Go code exists in `gen/proto/auth/`. But:
- No `internal/modules/auth/` directory exists
- No handler implements `authv1connect.AuthServiceHandler`
- No routes register the auth service
- No login/logout/refresh logic anywhere

This means: **Users cannot authenticate.** The JWT token generation utilities exist (`internal/shared/auth/jwt.go`) but there is no endpoint to call them.

**DB tables exist but are unused:**
- `refresh_tokens` table (migration `00002_auth_tables.sql`)
- `api_keys` table (migration `00002_auth_tables.sql`)
- `db/queries/auth.sql` has full CRUD queries for both
- Generated sqlc code exists in `gen/sqlc/auth.sql.go`
- `internal/shared/auth/apikey.go` has `GenerateAPIKey()` / `HashAPIKey()`
- NONE of these are used by any Go application code

**RBAC defined but never applied:**
- `internal/shared/middleware/rbac.go` defines `RequirePermission()` and `RequireRole()`
- Permission constants: `PermUserRead`, `PermUserWrite`, `PermUserDelete`, `PermAdminAll`
- `internal/modules/user/adapters/grpc/routes.go:22` applies `appmw.Auth()` but NOT any RBAC middleware
- Result: any authenticated user can CRUD any user

**Other security observations (positive):**
- JWT validation correctly checks signing method (lines 40-42 of jwt.go)
- Token blacklist via Redis is implemented in auth middleware
- Password hashing uses argon2id with constant-time comparison
- Security headers applied globally (HSTS, X-Frame-Options, CSP, etc.)
- Rate limiting via Redis sliding window
- SMTP CRLF injection sanitized
- CORS configured via env vars

### 4. Event System (Grade: A-)

**Consistent publish pattern across ALL mutations:**
- `create_user.go:62-76` -> `TopicUserCreated`
- `update_user.go:52-63` -> `TopicUserUpdated`
- `delete_user.go:30-41` -> `TopicUserDeleted`

All follow the same pattern:
1. Extract ActorID from `auth.UserFromContext(ctx)`
2. Publish after successful DB write
3. Log errors but don't fail the handler

**Subscriber handling is consistent:**
- `audit/subscriber.go` handles all 3 event types identically
- `notification/subscriber.go` handles `user.created` only (appropriate)
- Both use `msg.Context()` for trace propagation
- Both use `uuid.Parse()` (not `MustParse()`) safely

**Event bus correctly propagates OTel trace context** via `propagation.MapCarrier` in `events/bus.go:57`.

**Minor issues:**
- **Audit ActorID fallback** (`audit/subscriber.go:24-35`): When ActorID is empty (e.g., system-initiated creation during seed), it falls back to EntityID. This means the audit log shows the user created themselves. For a boilerplate, this is acceptable but should be documented.
- **Watermill router uses `context.Background()`** (`events/subscriber.go:59`): The router ignores Fx shutdown context. When the Fx app shuts down, `router.Close()` is called (line 66) which should stop it, but in-flight messages may still use background context.

### 5. Database & Repository Pattern (Grade: B)

**Strengths:**
- Consistent error wrapping with `fmt.Errorf("action: %w", err)`
- `pgx.ErrNoRows` mapped to `sharederr.ErrNotFound` everywhere
- Postgres unique constraint (23505) caught and mapped to domain error
- `UPDATE` uses `SELECT FOR UPDATE` within transaction
- Cursor-based pagination with base64-encoded JSON keyset
- sqlc type overrides configured correctly (`uuid.UUID`, `time.Time`, `json.RawMessage`)

**Issues:**

**B-1: CreateUser returns stale domain entity** (`create_user.go:52-58`)
```go
user, err := domain.NewUser(...)  // generates UUID in Go
if err := h.repo.Create(ctx, user); err != nil { ... }
return user, nil  // returns Go-generated entity
```
But `db/queries/user.sql:19-21`:
```sql
INSERT INTO users (email, name, password, role)
VALUES ($1, $2, $3, $4) RETURNING *;
```
The SQL does NOT use the Go-generated UUID -- it uses `gen_random_uuid()` from the DB default. The domain entity's `id`, `created_at`, `updated_at` are ALL different from what the DB actually stored. The `Create` repository method (`repository.go:98-114`) ignores the `RETURNING *` row entirely.

**B-2: SELECT * returns password hashes** (`db/queries/user.sql`)
All queries use `SELECT *` which includes the `password` column. The `toDomain()` function (`repository.go:182-193`) reconstitutes the full entity including password hash. This flows through the handler chain. While `toProto()` in `mapper.go` correctly omits password from the proto response, the hash is still available in the domain entity passed around in memory. Not a security vulnerability per se (it never reaches the client), but it's an anti-pattern that could surprise new devs.

**B-3: App-layer email uniqueness check is TOCTOU race** (`create_user.go:39-45`)
The app layer checks email uniqueness with `GetByEmail()`, then creates with `Create()`. Between these two calls, another request could create the same email. The DB constraint (`users_email_key`) catches it anyway in `repository.go:108`, making the app-layer check redundant. Remove the app-layer check or document it as a "fast fail for UX" optimization.

### 6. Config & Infrastructure (Grade: A-)

**Config loading is robust:**
- Required vars: `DATABASE_URL`, `REDIS_URL`, `RABBITMQ_URL`, `JWT_SECRET` (all marked `required`)
- JWT_SECRET minimum length validation (32 chars) at load time
- Sensible defaults for optional vars
- `.env.example` complete and accurate

**Docker setup:**
- Multi-stage Dockerfile (builder + runtime)
- `alpine:3.19` minimal runtime
- Docker HEALTHCHECK configured
- Dev compose has all services with healthchecks
- Production compose has Traefik + TLS + 2 replicas

**CI/CD complete:**
- 4 stages: quality, test, build, deploy
- Generated code staleness check
- Unit + integration tests (with DB/Redis/RabbitMQ services)
- Staging auto-deploy on main, production manual on tags

**Issues:**

**I-1: Postgres/Redis pools not shut down via Fx lifecycle hooks** (`internal/shared/module.go`)
The `shared.Module` provides `database.NewPostgresPool` and `database.NewRedisClient` but does NOT register `OnStop` hooks to call `pool.Close()` or `rdb.Close()`. The OTel providers DO have shutdown hooks (lines 26-33). This is an inconsistency -- when the app shuts down, DB/Redis connections may linger.

**I-2: `docker-compose.monitor.yml` referenced in Taskfile but does not exist**
`Taskfile.yml:140`: `docker compose -f deploy/docker-compose.monitor.yml up -d`
File `deploy/docker-compose.monitor.yml` does not exist. Running `task monitor:up` will fail.

### 7. Observability (Grade: B)

**What's good:**
- Structured logging: JSON in production, text in development (`observability/logger.go`)
- OTel tracing + metrics configured with OTLP gRPC exporter
- Trace context propagated through event bus
- OTel providers flush on Fx shutdown
- Request logger sanitizes sensitive headers (Authorization, Cookie)
- Recovery middleware logs stack traces with slog

**Issues:**

**O-1: `WithInsecure()` hardcoded** (`observability/tracer.go:22`, `metrics.go:20`)
Both tracer and meter use `otlptracegrpc.WithInsecure()` / `otlpmetricgrpc.WithInsecure()` unconditionally. In production with an external OTel collector, this sends telemetry over unencrypted gRPC. Should be conditioned on `cfg.IsDevelopment()` or a dedicated config flag.

**O-2: Logger not injected into app layer**
`NewLogger()` returns `*slog.Logger` and sets it as default via `slog.SetDefault()`, but app layer code uses `slog.ErrorContext()` directly (global). The Fx-provided `*slog.Logger` is never consumed by any module. This works due to `SetDefault()`, but it's inconsistent with the DI philosophy -- the logger cannot be replaced in tests.

### 8. Testing Infrastructure (Grade: F)

**Zero test files exist.** `find . -name "*_test.go"` returns nothing.

**What's prepared:**
- `internal/shared/testutil/db.go` - testcontainers Postgres helper
- `internal/shared/testutil/redis.go` - testcontainers Redis helper
- `internal/shared/testutil/rabbitmq.go` - testcontainers RabbitMQ helper
- `internal/shared/testutil/fixtures.go` - user fixtures (Default, Admin, Viewer)
- CI pipeline has unit + integration test stages
- Taskfile has `test`, `test:integration`, `test:coverage` targets
- `.golangci.yml` configured
- `.lefthook.yml` runs tests on pre-push

**But:** All of this infrastructure is unused. CI will report 0% coverage. `task test` will succeed (no tests = no failures) but is meaningless.

**For a boilerplate this is problematic** because new devs have no example tests to follow. The `testutil` helpers exist but there's no demonstration of how to wire them together.

### 9. Documentation (Grade: B+)

**What's accurate:**
- `docs/architecture.md` accurately describes request flow, event flow, middleware chain
- `docs/code-standards.md` matches actual code patterns (entity encapsulation, error handling, pagination)
- `docs/error-codes.md` matches `domain_error.go` codes
- `README.md` is concise with correct commands
- `docs/project-changelog.md` reflects actual recent changes

**What will confuse new devs:**

**D-1: `docs/adding-a-module.md` teaches a DIFFERENT entity pattern than actual code**

The doc example (lines 84-93):
```go
type Product struct {
    ID   uuid.UUID
    Name string
}
```
Exported fields, no encapsulation, no `Reconstitute()`, no getters.

The actual user module uses unexported fields + getters + `Reconstitute()`:
```go
type User struct {
    id        UserID
    email     string
    ...
}
func (u *User) ID() UserID { return u.id }
func Reconstitute(...) *User { ... }
```

A new dev following the doc will create a module with a completely different entity pattern than what exists. This defeats the purpose of a boilerplate.

**D-2: `adding-a-module.md` error example uses wrong API**

Doc shows (lines 121-124):
```go
ErrProductNotFound = errors.NewNotFound("product not found")
```
But `internal/shared/errors/domain_error.go` has NO `NewNotFound()` function. The correct API is:
```go
ErrProductNotFound = errors.New(errors.CodeNotFound, "product not found")
```

**D-3: `adding-a-module.md` repository example omits error wrapping**

Doc shows (lines 187-191):
```go
if err != nil {
    return nil, domain.ErrProductNotFound
}
```
This swallows the original error. The actual user repository wraps with `fmt.Errorf("getting user by id: %w", err)` and checks `pgx.ErrNoRows` before returning the domain error.

**D-4: Architecture doc claims "RBAC Middleware (role check)" in request flow**
But RBAC is never applied to any route. This gives a false impression to new devs.

### 10. DX (Developer Experience) (Grade: A-)

**Excellent:**
- `task dev:setup` is one-command onboarding (install tools, start infra, migrate, seed)
- `task dev` hot reloads via Air
- `task generate` runs buf + sqlc
- `task check` = lint + test
- `.lefthook.yml` has pre-commit (lint + generated check) and pre-push (tests)
- `.golangci.yml` with sensible linter set
- `.env.example` complete
- Seed data with admin/member/viewer users

**Issues:**

**DX-1: `task monitor:up` references non-existent `docker-compose.monitor.yml`** (same as I-2)

**DX-2: No `task dev:stop` or `task dev:down`** to stop infra containers. Dev must manually `docker compose -f deploy/docker-compose.dev.yml down`.

### 11. Health Checks & Graceful Shutdown (Grade: C)

**Health endpoints are dummy stubs** (`cmd/server/main.go:50-55`):
```go
e.GET("/healthz", func(c echo.Context) error {
    return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
})
e.GET("/readyz", func(c echo.Context) error {
    return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
})
```
Neither checks Postgres, Redis, or RabbitMQ connectivity. The Kubernetes readiness probe (or Docker HEALTHCHECK which uses `/healthz`) will report healthy even when the database is down.

**Graceful shutdown:**
- Echo server shutdown: YES (via Fx OnStop hook, `main.go:73-76`)
- OTel providers: YES (via `registerOTelShutdown`, `shared/module.go:26-33`)
- Watermill router: YES (via `subscriber.go:64-68`)
- Cron scheduler: YES (via `cron/scheduler.go:55-67`)
- **Postgres pool: NO** -- `database.NewPostgresPool` never registers `pool.Close()` in Fx lifecycle
- **Redis client: NO** -- `database.NewRedisClient` never registers `rdb.Close()` in Fx lifecycle

### 12. Dead Code & Inconsistencies (Grade: B)

**Dead code:**
- `internal/shared/model/base.go`: `BaseModel` struct is never used anywhere. Domain entities use their own fields. Remove it.
- `internal/shared/auth/apikey.go`: `GenerateAPIKey()` / `HashAPIKey()` are never called. DB tables + queries exist but no handler uses them.
- `db/queries/auth.sql`: Full auth query set (refresh tokens, API keys) -- generated sqlc code exists but zero Go application code consumes it.
- Cron scheduler starts with zero registered jobs. `DeleteExpiredRefreshTokens` query exists but is never registered as a cron job.

**Naming inconsistencies:**
- Repository naming: user module uses `PgUserRepository` (prefixed with Pg); adding-a-module doc shows just `Repository`. New devs won't know which convention to follow.
- Constructor naming: user repo uses `NewPgUserRepository`; doc example shows `NewRepository`.

**Swagger hardcoded:**
`internal/shared/middleware/swagger.go:30` hardcodes:
```
url:"/swagger/spec/user/v1/user.swagger.json"
```
When a new module is added, its swagger spec won't be visible. Should either list all specs dynamically or document that this needs manual update.

---

## Critical Issues (Must Fix Before Sharing)

| # | Severity | File:Line | Issue | Impact on new devs |
|---|----------|-----------|-------|-------------------|
| C-1 | CRITICAL | `proto/auth/v1/auth.proto` (no Go impl) | AuthService proto defined but has NO Go handler, routes, or module | New dev clones boilerplate, tries to authenticate -- nothing works. They waste hours looking for the auth handler. |
| C-2 | CRITICAL | `cmd/server/main.go:50-55` | /healthz and /readyz are dummy stubs that don't check DB/Redis/RabbitMQ | In production, orchestrator thinks app is healthy when DB is down. Pages happen at 3 AM. |
| C-3 | HIGH | `internal/modules/user/app/create_user.go:52-58` + `adapters/postgres/repository.go:98-114` | CreateUser returns domain entity with Go-generated UUID that differs from DB-generated UUID | API returns wrong ID to client. Every subsequent GetUser with that ID fails. Silent data inconsistency. |
| C-4 | HIGH | `internal/shared/database/postgres.go` + `redis.go` | No Fx lifecycle OnStop hooks for pool/client shutdown | Connection leaks on restart. In CI/containers, not critical. In production with rolling deploys, connections accumulate. |
| C-5 | HIGH | `docs/adding-a-module.md` lines 84-93 | Doc teaches exported-field entity pattern; actual code uses unexported+getters+Reconstitute | New dev creates inconsistent module. Two patterns in one codebase. Team reviews flag it, causing rework. |

## Consistency Issues (Should Fix)

| # | File | Issue |
|---|------|-------|
| S-1 | `docs/adding-a-module.md:121-124` | `errors.NewNotFound()` does not exist; actual API is `errors.New(errors.CodeNotFound, ...)` |
| S-2 | `docs/adding-a-module.md:187-191` | Repository error handling omits `pgx.ErrNoRows` check and error wrapping; teaches wrong pattern |
| S-3 | `internal/modules/user/adapters/grpc/routes.go:22` | Auth applied but RBAC never applied; `docs/architecture.md` mentions RBAC in request flow |
| S-4 | `internal/shared/middleware/swagger.go:30` | Swagger UI hardcoded to user.swagger.json; won't show new module APIs |
| S-5 | `internal/shared/observability/tracer.go:22` + `metrics.go:20` | `WithInsecure()` unconditional; should check `cfg.IsProduction()` |
| S-6 | Repository naming: `PgUserRepository` vs doc's `Repository` | Pick one convention and document it |
| S-7 | `internal/shared/middleware/request_id.go:19-21` | Client-supplied X-Request-ID accepted without length/content validation; could be used for log injection |
| S-8 | `internal/shared/events/subscriber.go:59` | Watermill router runs with `context.Background()` instead of Fx-provided context |
| S-9 | `internal/modules/audit/subscriber.go:24-35` | ActorID falls back to EntityID when empty -- misleading audit trail for system operations |

## Missing Pieces (Nice to Have)

| # | What | Why |
|---|------|-----|
| N-1 | At least ONE example `_test.go` file | New devs need a test to copy, not just testutil helpers |
| N-2 | `task dev:down` / `task dev:stop` | Currently no way to stop dev infra via task |
| N-3 | `deploy/docker-compose.monitor.yml` | Referenced in Taskfile but doesn't exist |
| N-4 | Remove `internal/shared/model/base.go` | Dead code; confuses new devs who think they should use it |
| N-5 | Remove or implement auth module | Either implement Login/Refresh/Logout or remove the proto + DB tables + queries |
| N-6 | Example cron job registration | Scheduler starts empty; `DeleteExpiredRefreshTokens` query exists but isn't wired |
| N-7 | `adding-a-module.md` section on event subscribers | Doc covers proto+sql+handler but not "how to subscribe to events from another module" |

---

## New Dev Onboarding Test

**Scenario: "I'm a new dev. I want to add a 'product' module."**

### Step 1: Read docs
- Start with `README.md` -> links to `docs/adding-a-module.md`. Good.
- The doc has 6 clear steps. Good.

### Step 2: Create proto definition
- Doc example is clear. Developer can follow it. **PASS.**

### Step 3: Create SQL queries + migration
- Doc example is clear. **PASS.**

### Step 4: Generate code
- `task generate` -- works. **PASS.**

### Step 5: Create module structure -- **MULTIPLE CONFUSION POINTS**

**Stuck Point 1:** Domain entity pattern.
Doc shows `type Product struct { ID uuid.UUID; Name string }` (exported).
Dev looks at existing user module for reference and sees `type User struct { id UserID; email string }` (unexported + getters + Reconstitute).
**Dev is confused: which pattern do I follow?** If they follow the doc, their module is inconsistent with the rest. If they follow the code, the doc is wrong.

**Stuck Point 2:** Domain errors.
Doc shows `errors.NewNotFound(...)`. Dev tries it -- **compilation error**. They must read the actual error package to discover `errors.New(errors.CodeNotFound, ...)`.

**Stuck Point 3:** Repository error handling.
Doc shows bare `return nil, domain.ErrProductNotFound` without checking `pgx.ErrNoRows` first. Dev follows this and gets incorrect behavior (any DB error returns "not found").

**Stuck Point 4:** Repository naming.
Doc shows `type Repository struct` and `func NewRepository(...)`.
Existing code uses `type PgUserRepository struct` and `func NewPgUserRepository(...)`.
Dev picks one; 50% chance it's inconsistent with existing code.

### Step 6: Register in main.go
- Doc example is clear. **PASS.**

### Step 7: Add event subscribers (NOT IN DOC)
Dev wants their product module to publish events and have audit trail. There's NO documentation for this. They must reverse-engineer the user module's `create_user.go` and `audit/subscriber.go`. Doable but not "mechanical."

### Step 8: Add RBAC
Dev sees RBAC middleware exists but no example of it being applied. `routes.go` only shows auth, not RBAC. Dev has to guess how to wire it.

**Verdict on onboarding: 4 out of 8 steps have friction points.** A new dev CAN figure it out, but they'll spend 1-2 hours on things that should be zero-thought mechanical copy-paste.

---

## Verdict

### NOT READY

The boilerplate has **excellent architectural bones** -- the hexagonal pattern is clean, Fx wiring is elegant, the middleware chain is production-quality, and the event system is well-designed.

However, it fails the core requirement of a boilerplate: **a new dev cannot follow the established patterns without making decisions or hitting errors.**

### Conditions for "READY":

**Must fix (blocking):**
1. Fix CreateUser to return correct entity (sync domain UUID with DB, or read RETURNING row)
2. Fix health checks to verify DB/Redis/RabbitMQ connectivity
3. Add Fx lifecycle hooks for Postgres pool and Redis client shutdown
4. Rewrite `docs/adding-a-module.md` to match actual code patterns (unexported fields, correct error API, proper repository error handling)
5. Either implement AuthService or remove the proto/tables/queries (half-implemented auth is worse than no auth -- it suggests the feature exists when it doesn't)

**Should fix (high value):**
6. Apply RBAC to user routes as a working example, or remove RBAC code + doc references
7. Add at least one example test file demonstrating testutil usage
8. Remove dead code (model/base.go)
9. Condition OTel WithInsecure() on environment
10. Create the missing `docker-compose.monitor.yml` or remove the Taskfile target

After these 10 items, the boilerplate would be genuinely production-ready and mechanically copyable.
