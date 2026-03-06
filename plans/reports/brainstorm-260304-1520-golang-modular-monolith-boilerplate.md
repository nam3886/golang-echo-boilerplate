# Brainstorm: Go Modular Monolith API Boilerplate

**Date:** 2026-03-04
**Status:** Agreed

---

## Problem Statement

Need a production-ready Go boilerplate for API development:
- Modular monolith architecture (split to micro later if needed)
- Great DX: hot reload, linting, testing, debugging
- Full-featured: auth, gRPC, events, observability
- Maintainable, scalable, follows Go idioms

## Agreed Tech Stack

| Concern | Choice | Rationale |
|---------|--------|-----------|
| **Architecture** | Modular Monolith | Bounded contexts, 1 binary, split later |
| **HTTP Framework** | Echo v4 | Clean error handling, stable, user familiar |
| **gRPC** | Connect RPC (same port) | HTTP/1.1 compatible, curl-debuggable, mount on Echo |
| **Database** | sqlc + pgx/v5 | Type-safe SQL-first, zero reflection |
| **Migrations** | goose | Simple, SQL-based, pairs with sqlc |
| **DI** | Uber Fx | Lifecycle hooks, graceful shutdown, modular |
| **Events** | Watermill + RabbitMQ | Pub/sub abstraction, outbox pattern, CQRS |
| **Config** | envconfig | 12-factor, struct tags, zero deps |
| **Auth** | golang-jwt/jwt v5 + RBAC | Standard JWT, role-based access |
| **Validation** | go-playground/validator v10 | Struct tags, 16k stars |
| **Logging** | slog (stdlib) | Go 1.21+, no deps |
| **Observability** | OpenTelemetry SDK | Tracing, metrics, health checks |
| **API Docs** | swaggo/swag | Generate Swagger from comments |
| **Hot Reload** | air | Standard for Go |
| **Linting** | golangci-lint v2 | Includes staticcheck |
| **Testing** | testify + testcontainers-go | Real infra, no mocking |
| **Containerization** | Docker + Docker Compose | Dev environment |

## Project Structure

```
gnha-services/
├── cmd/
│   └── server/
│       └── main.go                 # Single entrypoint, Uber Fx app
├── internal/
│   ├── modules/
│   │   ├── user/                   # Example bounded context
│   │   │   ├── domain/
│   │   │   │   ├── entity.go       # User entity, value objects
│   │   │   │   ├── repository.go   # Port interface
│   │   │   │   └── errors.go       # Domain-specific errors
│   │   │   ├── app/
│   │   │   │   ├── commands.go     # Write operations (CQRS command side)
│   │   │   │   ├── queries.go      # Read operations (CQRS query side)
│   │   │   │   └── service.go      # Application service (orchestrates)
│   │   │   ├── adapters/
│   │   │   │   ├── postgres/
│   │   │   │   │   └── repository.go   # sqlc-based repository impl
│   │   │   │   ├── http/
│   │   │   │   │   ├── handler.go      # Echo handlers
│   │   │   │   │   └── routes.go       # Route registration
│   │   │   │   └── grpc/
│   │   │   │       └── handler.go      # Connect RPC handlers
│   │   │   └── module.go           # Uber Fx module definition
│   │   └── auth/                   # Auth module (same structure)
│   │       └── ...
│   ├── shared/
│   │   ├── config/
│   │   │   └── config.go           # envconfig struct
│   │   ├── database/
│   │   │   └── postgres.go         # pgx pool setup
│   │   ├── middleware/
│   │   │   ├── auth.go             # JWT middleware
│   │   │   ├── cors.go             # CORS
│   │   │   ├── logging.go          # Request logging
│   │   │   ├── recovery.go         # Panic recovery
│   │   │   └── otel.go             # OpenTelemetry middleware
│   │   ├── errors/
│   │   │   └── errors.go           # Shared error types + handler
│   │   ├── response/
│   │   │   └── response.go         # Standard API response format
│   │   ├── pagination/
│   │   │   └── cursor.go           # Cursor-based pagination
│   │   └── events/
│   │       └── publisher.go        # Watermill publisher setup
│   └── server/
│       └── server.go               # Echo server setup, route mounting
├── db/
│   ├── migrations/                 # goose SQL migrations
│   │   ├── 001_create_users.sql
│   │   └── ...
│   ├── queries/                    # sqlc SQL queries
│   │   ├── users.sql
│   │   └── ...
│   └── sqlc.yaml                   # sqlc config
├── proto/                          # Connect RPC .proto files
│   └── user/
│       └── v1/
│           └── user.proto
├── buf.yaml                        # buf config for proto
├── buf.gen.yaml                    # buf codegen config
├── docs/
│   └── swagger/                    # Generated swagger docs
├── scripts/
│   ├── migrate.sh                  # Migration helper
│   └── seed.sh                     # DB seeding
├── .air.toml                       # Hot reload config
├── .golangci.yml                   # Linter config
├── docker-compose.yml              # Postgres, RabbitMQ, Redis, Jaeger
├── Dockerfile                      # Multi-stage build
├── Makefile                        # Dev commands
├── go.mod
└── go.sum
```

## Architecture Principles

1. **Dependencies point inward**: domain → app → adapters. Domain has zero infra knowledge.
2. **No cross-module DB joins**: Modules communicate via service interfaces or events.
3. **`internal/` enforces encapsulation**: Compile-time boundary enforcement.
4. **Each module = bounded context**: Self-contained package tree.
5. **CQRS via Watermill**: Commands mutate state, events trigger side effects async.
6. **Outbox pattern**: Reliable event publishing (write event + data in same DB transaction).

## Key Design Decisions

### Echo + Connect RPC Same Port
- Echo serves REST endpoints normally
- Connect RPC handlers mount as `http.Handler` on Echo
- Single port = simpler deployment, health checks, load balancing
- Connect RPC supports JSON over HTTP/1.1 = curl-debuggable

### sqlc Workflow
```
Write SQL in db/queries/*.sql
  → sqlc generate
  → Type-safe Go code in internal/modules/*/adapters/postgres/
```
- For dynamic queries (complex filters): use raw pgx with squirrel or manual SQL builder
- sqlc handles 90%+ of queries; manual for the rest

### Uber Fx Module Pattern
Each module exports an `fx.Option`:
```go
// internal/modules/user/module.go
var Module = fx.Options(
    fx.Provide(postgres.NewRepository),
    fx.Provide(app.NewService),
    fx.Provide(http.NewHandler),
)
```
Main wires everything:
```go
// cmd/server/main.go
fx.New(
    config.Module,
    database.Module,
    user.Module,
    auth.Module,
    server.Module,
)
```

### Error Handling
- Domain errors → application errors → HTTP/gRPC error codes
- Central error handler middleware in Echo
- Structured error response: `{code, message, details}`

### Auth Flow
- JWT access token (short-lived) + refresh token (long-lived, DB-stored)
- RBAC middleware checks role claims
- Per-module permission definitions

## Makefile Commands (DX)

```makefile
dev         # air hot reload
build       # go build
test        # go test ./...
test-int    # testcontainers integration tests
lint        # golangci-lint run
migrate-up  # goose up
migrate-new # goose create
sqlc        # sqlc generate
proto       # buf generate
swagger     # swag init
docker-up   # docker compose up -d
docker-down # docker compose down
```

## Docker Compose Services

- **postgres:16** — Primary database
- **rabbitmq:3-management** — Message broker (management UI :15672)
- **redis:7** — Caching
- **jaeger:all-in-one** — Distributed tracing UI (:16686)

## Implementation Priority (YAGNI)

### P0 — Foundation (Boilerplate Core)
- [ ] Project structure + go.mod
- [ ] envconfig + config struct
- [ ] pgx pool + goose migrations
- [ ] sqlc setup + first query
- [ ] Echo server + graceful shutdown
- [ ] Uber Fx wiring
- [ ] Error handling middleware
- [ ] Standard response format
- [ ] slog structured logging
- [ ] Makefile + .air.toml + .golangci.yml
- [ ] Dockerfile + docker-compose.yml
- [ ] Example "user" module (full vertical slice)

### P1 — Auth & API
- [ ] JWT auth middleware
- [ ] RBAC middleware
- [ ] Request validation (validator v10)
- [ ] Cursor pagination
- [ ] swaggo/swag setup
- [ ] Health checks (/healthz, /readyz)

### P2 — Events & Observability
- [ ] Watermill + RabbitMQ setup
- [ ] Outbox pattern
- [ ] OpenTelemetry tracing middleware
- [ ] Jaeger integration
- [ ] Connect RPC setup (buf + proto)

### P3 — Enhancement
- [ ] Redis caching (cache-aside pattern)
- [ ] Rate limiting
- [ ] Idempotency keys
- [ ] Integration tests (testcontainers)

## Rejected Alternatives

| Option | Reason |
|--------|--------|
| go-kratos | Microservices-first, overkill for modular monolith |
| go-zero | DSL lock-in (goctl), Chinese-market focus |
| Fiber v3 | FastHTTP breaks net/http compatibility |
| GORM | Reflection overhead, auto-migrate risky, hides SQL |
| Google Wire | Deprecated internally by Google (2024) |
| Kafka | Heavy infra, overkill for monolith |
| Viper | Heavy, reflect-based, too much for 12-factor config |

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| sqlc can't handle dynamic queries | Use squirrel/raw pgx for complex filters (<10% of queries) |
| Echo v5 breaking changes | Target v4 (stable until Dec 2026), migration guide available |
| Watermill learning curve | Start with simple pub/sub, add outbox when needed |
| Connect RPC + Echo same port | Well-documented pattern, Connect is just http.Handler |
| Module boundaries leak | Code review, `internal/` enforcement, lint rules |

## Unresolved Questions

1. **sqlc `emit_interface`**: Use sqlc's interface generation or manual repository ports? → Recommend manual ports (cleaner domain boundary)
2. **Watermill outbox + pgx transactions**: Coordinate via manual publish-after-commit or Watermill's built-in PostgreSQL outbox? → Research during implementation
3. **Elasticsearch**: Not included in P0-P3. Add when search requirements are clear.
4. **Redis for pub/sub**: Use only for caching now. Watermill + RabbitMQ for events.

## Next Steps

1. Create detailed implementation plan (phases with tasks)
2. Scaffold project structure
3. Implement P0 foundation
4. Iterate through P1-P3 based on need
