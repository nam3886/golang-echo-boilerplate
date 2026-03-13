# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## IMPORTANT RULE

Whenever you run shell commands, ALWAYS execute them via:
```
bash -lc "<command>"
```
Do NOT use zsh. Do NOT rely on the system default shell. Do NOT omit `bash -lc` even for simple commands.

## Build & Development Commands

This project uses **go-task** (`task`) as the task runner (NOT Make).

| Command | Purpose |
|---------|---------|
| `task dev:setup` | Bootstrap: install tools, start infra, migrate, seed |
| `task dev:tools` | Install all dev tools (air, sqlc, buf, goose, mockgen, lefthook) |
| `task dev` | Hot-reload server (air) on :8080 |
| `task dev:deps` | Start Postgres/Redis/RabbitMQ/ES/Mailpit containers |
| `task dev:stop` | Stop infrastructure containers |
| `task generate` | Run all codegen (proto + sqlc + mocks) |
| `task generate:proto` | buf lint + buf generate (proto → Go/TS/OpenAPI) |
| `task generate:sqlc` | sqlc generate (SQL → Go) |
| `task generate:mocks` | go generate ./... (mockgen) |
| `task lint` | golangci-lint run ./... |
| `task test` | Unit tests with race detector + coverage |
| `task test:integration` | Integration tests (requires Docker for testcontainers) |
| `task test:coverage` | Run unit tests + generate HTML coverage report |
| `task check` | lint + test combined |
| `task build` | Production binary → bin/server |
| `task migrate:up` | Run pending goose migrations |
| `task migrate:down` | Rollback last migration |
| `task migrate:status` | Show current migration status |
| `task migrate:create -- <name>` | Create new SQL migration |
| `task seed` | Populate test data |
| `task db:reset` | Drop + recreate dev DB, run migrations, seed |
| `task docker:build` | Build Docker image |
| `task docker:run` | Run Docker image with .env |
| `task clean` | Remove build artifacts |
| `task module:create name=<name>` | Scaffold a full CRUD module |

### Running a Single Test

```bash
go test -race -run TestFunctionName ./internal/modules/user/app/...
```

### Running Integration Tests for One Package

```bash
go test -race -tags=integration -run TestName ./internal/modules/user/adapters/postgres/...
```

## Architecture

**Simplified Hexagonal Architecture** — modular monolith with extract-ready microservices.

### Stack

Go 1.26 | Echo v4 | Connect RPC (gRPC-HTTP) | PostgreSQL (sqlc) | Redis | RabbitMQ (Watermill) | Elasticsearch (optional) | Uber fx (DI) | OpenTelemetry

### Module Structure

Each domain module in `internal/modules/<name>/` follows:

```
domain/          # Entity (unexported fields + getters + mutations), repository interface, errors
app/             # Use-case handlers (CreateXHandler, GetXHandler, etc.) + unit tests
adapters/
  postgres/      # sqlc-based repository implementation + integration tests
  grpc/          # Connect RPC handler, routes, proto↔domain mapper
  search/        # Elasticsearch adapter (optional)
module.go        # fx.Module wiring (providers + invokers)
```

### Request Flow

```
HTTP → Echo global middleware → Echo route group Auth middleware
  → Connect RPC handler → RBACInterceptor → protovalidate interceptor
  → App handler (business logic) → Repository interface → Postgres adapter (sqlc)
  → Event publish (Watermill/RabbitMQ) → Audit + Notification subscribers
```

### Key Architectural Rules

- **No cross-module imports** — modules only import from `internal/shared/`
- **Domain entities use unexported fields** — construct via `NewX()`, mutate via methods, read via getters
- **Repository is an interface in domain/** — implementation lives in `adapters/postgres/`
- **Events published after DB persistence** — subscriber failures don't cascade to client
- **Offset-based pagination** — page/page_size request, total/total_pages response

## Code Generation Pipeline

Changes to these files require regeneration:

| Source | Generator | Output |
|--------|-----------|--------|
| `proto/**/*.proto` | `buf generate` | `gen/proto/` (Go), `gen/ts/` (TS), `gen/openapi/` (Swagger) |
| `db/queries/*.sql` + `db/migrations/*.sql` | `sqlc generate` | `gen/sqlc/*.go` |
| `//go:generate` directives in domain/ | `go generate ./...` | `internal/shared/mocks/` |

After modifying `.proto` files: `task generate:proto`
After modifying SQL queries/migrations: `task generate:sqlc`
After changing repository interfaces: `task generate:mocks`

## Testing Patterns

- **Unit tests** (`*_test.go` beside source): gomock for repos, stub structs for deps (hasher, publisher)
- **Integration tests** (`//go:build integration`): testcontainers for real Postgres/Redis/RabbitMQ
- **Test stubs**: `testutil.StubHasher{}`, `testutil.NoopPublisher{}`, `testutil.CapturingPublisher{}`, `testutil.FailPublisher{}`
- **Container helpers**: `testutil.NewTestPostgres(t)`, `testutil.NewTestRedis(t)`, `testutil.RunMigrations(t, pool)`
- **Utilities**: `testutil.Ptr[T](v)` — generic helper for pointer literals in test data
- Generated code in `gen/` is committed to the repo

## Error Handling

- `DomainError` with error codes: NOT_FOUND, ALREADY_EXISTS, INVALID_ARGUMENT, PERMISSION_DENIED, etc.
- Sentinel constructors: `sharederr.ErrNotFound()`, `ErrAlreadyExists()` — return fresh instances
- Each module defines custom errors: `ErrEmailTaken`, `ErrUserNotFound`
- Use `errors.Is()` / `errors.As()` for matching, `fmt.Errorf("context: %w", err)` for wrapping

## Configuration

Environment variables parsed via `caarlos0/env` struct tags in `internal/shared/config/config.go`.

**Required:**
- `DATABASE_URL`, `REDIS_URL`, `RABBITMQ_URL`, `JWT_SECRET`

**Optional:**
- `ELASTICSEARCH_URL` — empty disables search
- `OTEL_EXPORTER_OTLP_ENDPOINT` — OpenTelemetry collector endpoint
- `OTEL_SAMPLING_RATIO` — trace sampling (0.0–1.0, default 0.01)
- `BLACKLIST_FAIL_OPEN` — token blacklist strategy (default false = fail-closed)
- `RATE_LIMIT_RPM`, `RATE_LIMIT_WINDOW`, `RATE_LIMIT_SCOPE`, `RATE_LIMIT_ALGORITHM`, `RATE_LIMIT_DISTRIBUTED` — rate limiting config
- `JWT_ACCESS_TTL`, `JWT_REFRESH_TTL` — token expiry (default 15m, 168h)

## Dev Services (docker-compose)

| Service | Port | UI |
|---------|------|----|
| PostgreSQL 16 | 5432 | — |
| Redis 7 | 6379 | — |
| RabbitMQ 3 | 5672 | http://localhost:15672 |
| Elasticsearch 8 | 9200 | — |
| Mailpit | 1025 (SMTP) | http://localhost:8025 |

## Project Documentation

- `docs/architecture.md` — hexagonal architecture details, layer responsibilities
- `docs/code-standards.md` — naming, patterns, domain/app/adapter conventions
- `docs/testing-strategy.md` — test organization, mocks, testcontainers
- `docs/adding-a-module.md` — step-by-step module creation + scaffold command
- `docs/error-codes.md` — error code → HTTP status mapping
- `docs/authentication.md` — JWT/API key auth, token generation, context helpers
- `docs/rbac.md` — RBAC permissions, interceptor architecture, adding new permissions
- `docs/event-subscribers.md` — Watermill event bus, subscriber patterns, audit/notification
