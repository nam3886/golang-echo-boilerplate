# Danh gia Kien truc va Cau truc Du an - gnha-services

## Code Review Summary

### Scope
- **Files**: 53 hand-written `.go` files in `internal/` + `cmd/`
- **Focus**: Architecture, module organization, DDD compliance, DI wiring, naming
- **Directories reviewed**: `cmd/`, `internal/`, `proto/`, `db/`, `deploy/`, `gen/`

### Overall Assessment

**Score: 8.3/10** -- This is a well-structured Go modular monolith boilerplate. The hexagonal architecture is clearly applied, Fx DI wiring is clean, and the domain layer is properly isolated. The codebase demonstrates strong architectural discipline for its size. Issues are mostly at the "polish" level rather than structural.

---

## Architecture Strengths

### 1. Clean Hexagonal Layering (9/10)
The user module demonstrates textbook hexagonal architecture:
```
user/
  domain/     -- entities, repository port, domain errors (zero external deps)
  app/        -- use case handlers (depends only on domain + shared interfaces)
  adapters/
    grpc/     -- Connect RPC handler, proto mapper, route registration
    postgres/ -- sqlc-backed repository implementation
```

**What works well:**
- Domain layer has ZERO infrastructure imports -- only `uuid` and `time`
- Repository port (`domain.UserRepository`) is a clean interface
- Application handlers accept interfaces, not concrete types
- Adapters depend inward (toward domain), never the reverse

### 2. Fx DI Wiring (8.5/10)
Each module exposes a single `fx.Module` var -- consistent, scannable, composable.

- `fx.Annotate` with `fx.As(new(domain.UserRepository))` properly binds interface to impl
- Event handlers use `group:"event_handlers"` tag for plugin-style registration
- `main.go` composes modules declaratively in ~35 lines

### 3. Domain Entity Design (8/10)
- Unexported fields + getters enforce invariant protection
- `Reconstitute()` separates creation (validated) from hydration (trusted)
- `NewUser()` validates at construction time
- `ChangeName()` / `ChangeRole()` enforce business rules

### 4. Event-Driven Architecture (8/10)
- Watermill + RabbitMQ with proper AMQP durable queues
- OTel trace propagation into message metadata -- excellent for distributed tracing
- Retry middleware (3 retries, 1s backoff)
- Clean handler registration via Fx groups

### 5. Shared Infrastructure (8.5/10)
- Config struct with env parsing, validation (JWT secret >= 32 chars)
- Middleware chain with correct ordering (recovery first, rate limit last)
- Centralized error types mapping to HTTP + Connect RPC codes
- DB + Redis + OTel shutdown hooks registered properly

---

## Architecture Weaknesses

### CRITICAL -- None

### HIGH Priority

#### H-1: Audit Module Creates Its Own `sqlcgen.Queries` Instance
**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/audit/module.go` (line 12)

```go
fx.Provide(func(pool *pgxpool.Pool) *sqlcgen.Queries {
    return sqlcgen.New(pool)
}),
```

The audit module provides `*sqlcgen.Queries` to the Fx container. If any other module also provides `*sqlcgen.Queries`, Fx will fail at startup with a duplicate type error. This works now because only audit uses it, but it is a latent wiring bomb.

**Fix**: Scope it locally or use `fx.Private`:
```go
fx.Provide(fx.Private, func(pool *pgxpool.Pool) *sqlcgen.Queries {
    return sqlcgen.New(pool)
}),
```

#### H-2: Event Types Live in `shared/events` Instead of Domain Packages
**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/events/topics.go`

`UserCreatedEvent`, `UserUpdatedEvent`, `UserDeletedEvent` are defined in `shared/events`. In DDD, domain events belong to the domain that emits them. This creates a subtle coupling: audit and notification modules import `shared/events` for event structs, which is acceptable for shared infrastructure, but the event *payload definitions* should live closer to their bounded context.

**Impact**: As modules grow, `shared/events/topics.go` becomes a God file that every module depends on. Adding a new module (e.g., `order`) means editing shared code.

**Recommendation**: Move event payloads to `user/domain/events.go` (or `user/app/events.go`). Keep only topic constants and `EventBus` in shared. Consumers deserialize from JSON so they only need the struct, which can be duplicated per consumer (anti-corruption layer) or kept in a shared contract package.

### MEDIUM Priority

#### M-1: Audit and Notification Modules Lack Hexagonal Structure
**Files**: `internal/modules/audit/`, `internal/modules/notification/`

The user module has clean `domain/ -> app/ -> adapters/` separation. The audit and notification modules are flat: `module.go` + `subscriber.go` (+ `sender.go`, `email.go`).

**Impact**: Acceptable for event handlers that are essentially "leaf" modules, but inconsistent with the established pattern. If audit or notification grow (e.g., audit search, notification preferences), they will need restructuring.

**Recommendation**: Keep flat for now (YAGNI), but document the convention: "Leaf modules (event-only, no API surface) may use a flat structure."

#### M-2: `create_user.go` Imports `middleware` Package Directly
**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user.go` (line 14)

```go
appmw "github.com/gnha/gnha-services/internal/shared/middleware"
```

The `app` layer calls `appmw.GetClientIP(ctx)`. This creates a dependency from the application layer to an HTTP middleware package. In hexagonal architecture, the app layer should not know about HTTP transport concerns.

**Fix**: Extract `GetClientIP` into a transport-agnostic package (e.g., `shared/context/request.go`) or pass IP address as part of the command struct:
```go
type CreateUserCmd struct {
    Email     string
    Name      string
    Password  string
    Role      string
    IPAddress string  // populated by adapter
}
```

#### M-3: `UserRepository.List` Returns 4 Values
**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/domain/repository.go` (line 12)

```go
List(ctx context.Context, limit int, cursor string) ([]*User, string, bool, error)
```

Returning 4 values is a Go smell. The `string` (nextCursor) and `bool` (hasMore) are pagination metadata that should be a struct.

**Recommendation**:
```go
type ListResult struct {
    Users      []*User
    NextCursor string
    HasMore    bool
}
List(ctx context.Context, limit int, cursor string) (*ListResult, error)
```

#### M-4: Repository Creates `sqlcgen.New(r.pool)` on Every Method Call
**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go`

Every method does `q := sqlcgen.New(r.pool)`. While `sqlcgen.New` is cheap (just wraps a DBTX), it is repeated 6 times. Store it as a field.

#### M-5: `cursor` Is a Raw `string` in the Domain Repository Interface
Cursor-based pagination with base64-encoded JSON is an infrastructure concern. The domain port should not expose opaque string cursors. This leaks the pagination strategy into the domain contract.

**Alternative**: Define a `PageToken` value object in domain, or accept a domain-specific filter struct.

### LOW Priority

#### L-1: Cron Scheduler Starts with Zero Jobs
**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/cron/module.go`

The scheduler starts but no jobs are registered. This is fine for a boilerplate, but the cron lifecycle hook runs regardless.

#### L-2: `repository.go` at 222 Lines
Slightly exceeds the 200-line guideline from `development-rules.md`. The cursor helpers (`encodeCursor`, `decodeCursor`, `cursorPayload`) could move to a `cursor.go` file.

#### L-3: Package Name `grpc` Is Misleading
**Path**: `internal/modules/user/adapters/grpc/`

The adapter uses Connect RPC, not raw gRPC. Naming the package `grpc` is confusing. Consider `connectrpc` or just `transport`.

#### L-4: No `internal/modules/user/app/` Interface for Bus
The app handlers depend on concrete `*events.EventBus`. For testability, this should be an interface:
```go
type EventPublisher interface {
    Publish(ctx context.Context, topic string, event any) error
}
```

---

## Module Coupling Analysis

```
cmd/server/main.go
  |-- shared.Module       (config, DB, Redis, OTel, logger)
  |-- auth.NewPasswordHasher
  |-- events.Module       (AMQP pub/sub, Watermill router)
  |-- user.Module         (domain, app, grpc adapter, postgres adapter)
  |     |-- depends on: domain.UserRepository (interface), auth.PasswordHasher, events.EventBus
  |-- audit.Module        (event subscriber -> sqlc)
  |     |-- depends on: events.HandlerRegistration, sqlcgen.Queries, pgxpool.Pool
  |-- notification.Module (event subscriber -> SMTP)
  |     |-- depends on: events.HandlerRegistration, config.Config
  |-- cron.Module         (scheduler + Redis lock)
```

**Coupling verdict**: Low. Modules communicate via events (async). The only shared dependencies are infrastructure types (config, DB pool, event bus). No module imports another module directly. This is clean modular monolith design.

**One exception**: `app/create_user.go` imports `shared/middleware` (see M-2 above).

---

## DDD Compliance

| Aspect | Status | Notes |
|--------|--------|-------|
| Bounded contexts | OK | user, audit, notification as separate modules |
| Entity encapsulation | Strong | Unexported fields, getters, validation in constructors |
| Value objects | Partial | `UserID`, `Role` are typed but `email` is bare `string` |
| Repository port | Clean | Interface in domain, impl in adapter |
| Domain events | Partial | Events defined in shared, not in domain package (see H-2) |
| Application services | Good | Command handlers with single responsibility |
| Anti-corruption layer | Good | `toDomain()` and `toProto()` mappers at adapter boundaries |
| Aggregate root | N/A | Single entity, no aggregate complexity yet |

---

## Naming Consistency

| Convention | Followed | Examples |
|------------|----------|----------|
| Package names | Mostly | `domain`, `app`, `postgres`, `config` -- all good. `grpc` is misleading (L-3) |
| File names | Good | `create_user.go`, `domain_error.go`, `request_log.go` -- snake_case, descriptive |
| Struct names | Good | `CreateUserHandler`, `PgUserRepository`, `UserServiceHandler` |
| Interface names | Good | `UserRepository`, `Sender`, `PasswordHasher` |
| Fx modules | Consistent | All use `var Module = fx.Module("name", ...)` pattern |
| Error vars | Consistent | `Err` prefix: `ErrEmailRequired`, `ErrUserNotFound` |

---

## Dependency Analysis (go.mod)

**Direct deps**: 18 -- reasonable for the feature set.

| Dependency | Purpose | Verdict |
|------------|---------|---------|
| `uber/fx` | DI container | Correct choice for modular monolith |
| `echo/v4` | HTTP framework | Mature, good middleware ecosystem |
| `connectrpc` | RPC framework | Modern gRPC-compatible, HTTP/1.1 friendly |
| `watermill` + `watermill-amqp` | Event bus | Good abstraction over RabbitMQ |
| `pgx/v5` + `sqlc` | Database | Best-in-class Go Postgres combo |
| `go-redis/v9` | Cache + rate limit + cron lock | Appropriate |
| `otel/*` | Observability | Production-grade telemetry |
| `testcontainers-go` | Integration tests | Modern approach |
| `caarlos0/env` | Config parsing | Simple, well-maintained |

No red flags. No unnecessary dependencies.

---

## Taskfile & DX

**Taskfile.yml** is comprehensive: dev setup, code generation (proto + sqlc + mocks), lint, test (unit + integration + coverage), build, migrate, seed, Docker, monitoring.

**Positive**:
- `dev:setup` is a single command to onboard
- `generate` chains proto, sqlc, and mocks
- `module:create` scaffold command exists
- `.air.toml` correctly excludes `gen/`, test files, and non-Go dirs

**Issue**: `dev:deps` uses `sleep 5` for health check. Should use `docker compose up -d --wait` or a retry loop.

---

## Recommended Actions (Prioritized)

1. **[HIGH]** Add `fx.Private` to audit module's `sqlcgen.Queries` provider to prevent DI collision
2. **[MEDIUM]** Extract `GetClientIP` out of middleware package; pass IP via command struct to keep app layer transport-agnostic
3. **[MEDIUM]** Wrap `List` return values in a `ListResult` struct
4. **[MEDIUM]** Move event payload structs closer to their bounded context (or document the shared contract pattern as intentional)
5. **[LOW]** Rename `adapters/grpc` to `adapters/connectrpc` or `adapters/transport`
6. **[LOW]** Extract cursor helpers from `repository.go` to stay under 200-line guideline
7. **[LOW]** Define `EventPublisher` interface for testability of app handlers

---

## Positive Observations

- Shutdown hygiene is thorough: DB pool, Redis, AMQP pub/sub, OTel providers, cron scheduler, Echo server -- all have `OnStop` hooks
- Readiness probe (`/readyz`) checks both Postgres and Redis -- production-ready
- CRLF injection prevention in SMTP sender
- Distributed cron locking with Redis + Lua unlock script
- `Reconstitute()` pattern avoids bypassing validation for persistence hydration
- Error mapping is consistent across HTTP and Connect RPC
- Proto validation interceptor (`connectrpc.com/validate`) catches bad input before hitting app logic

---

## Unresolved Questions

1. Should event payload structs remain in `shared/events` (simpler) or move to domain packages (purer DDD)? This is a design decision, not a bug.
2. The `Sender` interface in notification is minimal (`Send(ctx, to, subject, body)`). Will it need `SendBatch` or template selection later? Current design is YAGNI-compliant.
3. No login/auth endpoint exists yet -- the JWT middleware validates tokens but there is no token issuance endpoint. Is this intentional for the boilerplate scope?
