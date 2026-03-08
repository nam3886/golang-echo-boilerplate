# Architecture

## Overview

GNHA Services uses a **Simplified Hexagonal Architecture** organized as a modular monolith. Each domain module is self-contained with clear boundaries, making it straightforward to extract into a microservice if needed.

## Module Structure

```
internal/
  shared/               # Cross-cutting infrastructure
    config/             # Environment-based configuration
    database/           # Postgres + Redis clients
    auth/               # JWT, API keys, password hashing
    middleware/         # Echo middleware chain
    events/             # Watermill event bus (RabbitMQ)
    cron/               # Scheduled jobs
    errors/             # Domain error types
    observability/      # OpenTelemetry tracing + metrics
    testutil/           # Test helpers (testcontainers)
  modules/
    user/               # User domain
      domain/           # Entity, repository interface, domain errors
      app/              # Command/query handlers (use cases)
      adapters/
        postgres/       # sqlc-generated repository implementation
        grpc/           # Connect RPC handler + routes
    audit/              # Audit trail event subscriber
    notification/       # Email notification event subscriber
```

## Request Flow

```
HTTP Request
    → Echo Router
    → Middleware Chain (recovery, request-id, logger, body-limit, gzip, security, cors, timeout, rate-limit)
    → Connect RPC Handler (grpc/handler.go)
    → Auth Middleware (JWT/API key validation)
    → RBAC Middleware (role check)
    → App Handler (app/command.go or app/query.go)
    → Repository Interface (domain/repository.go)
    → Postgres Adapter (adapters/postgres/repository.go)
    → Database (sqlc queries)
```

## Event Flow

```
Mutation Handler (Create/Update/Delete)
    → Persistence (repo.Create/Update/SoftDelete)
    → Extract ActorID from auth.UserFromContext(ctx)
    → EventBus.Publish(topic, payload)
    → Watermill Router
    → RabbitMQ Exchange
    → Per-Handler Subscribers (each via SubscriberFactory with unique queue)
    → Handler logic (DB write, email send, etc.)
    → Log errors (don't fail handler if event publishing fails)
```

**Event Payload Pattern:**
- All events include UserID (resource), ActorID (who initiated action), and At (timestamp)
- ActorID enables full audit trail correlation across mutations
- Event publishing failures are logged but don't cascade to client responses (graceful degradation)
- Published *after* successful database persistence to ensure consistency

**Subscriber Architecture:**
- Each module's event handler gets its own AMQP subscriber via `SubscriberFactory`
- Prevents round-robin message distribution — every handler receives all messages on its topic
- Queue name = `{topic}_{handlerName}` (e.g., `user.created_audit.user_created`)
- Handlers are registered in `module.go` via `event_handlers` fx group
- See `internal/modules/audit/module.go` and `internal/modules/notification/module.go` for examples

## Middleware Chain Order

### Echo Middleware (HTTP Layer)
1. Recovery — panic → 500 response
2. Request ID — generate/propagate X-Request-ID
3. Request Logger — structured slog with sanitized fields
4. Body Limit — 10MB max
5. Gzip — level 5 compression
6. Security Headers — HSTS, CSP, X-Frame-Options
7. CORS — configurable origins
8. Context Timeout — 30s global timeout
9. Rate Limit — 100 req/min per IP (Redis-backed)
10. Auth + RBAC — applied at route group level (JWT/API key validation)

### Connect RPC Interceptors (RPC Layer)
- **protovalidate** — Declarative request validation via `connectrpc.com/validate` interceptor
  - Validates protobuf messages against buf/validate rules
  - Returns 400 (INVALID_ARGUMENT) for validation failures
  - Applied before handler execution

## Key Design Decisions

- **sqlc over ORM**: Type-safe SQL without runtime reflection overhead
- **Connect RPC over REST**: HTTP/1.1 + HTTP/2 compatible, strong typing via protobuf
- **Watermill over direct RabbitMQ**: Pluggable message router with middleware support
- **fx for DI**: Compile-time dependency graph, lifecycle management
- **testcontainers**: Real Postgres/Redis/RabbitMQ in tests, no mocks for infrastructure
