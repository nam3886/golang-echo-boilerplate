# Architecture

## Overview

Golang Echo Boilerplate uses a **Simplified Hexagonal Architecture** organized as a modular monolith. Each domain module is self-contained with clear boundaries, making it straightforward to extract into a microservice if needed.

## Module Structure

```
internal/
  shared/               # Cross-cutting infrastructure
    config/             # Environment-based configuration
    database/           # Postgres + Redis clients
    auth/               # JWT, API keys, password hashing
    middleware/         # Echo middleware chain
    events/             # Watermill event bus (RabbitMQ)
    errors/             # Domain error types
    observability/      # OpenTelemetry tracing + metrics
    testutil/           # Test helpers (testcontainers)
  modules/
    user/               # User domain (Tier 1: full hexagonal)
      domain/           # Entity, repository interface, domain errors
      app/              # Command/query handlers (use cases)
      adapters/
        postgres/       # sqlc-generated repository implementation
        grpc/           # Connect RPC handler + routes
    audit/              # Audit trail (Tier 2: event subscriber only)
    notification/       # Email notification (Tier 2: event subscriber only)
```

**Tier 1 modules** (`user`, any new CRUD domain) own a full hexagonal stack: domain entity,
repository interface, app handlers, Postgres adapter, and Connect RPC routes.

**Tier 2 modules** (`audit`, `notification`) have no proto, no DB schema, and no gRPC routes.
They consist of a single event handler file and a `module.go` that registers handlers via the
`event_handlers` fx group. They react to domain events published by Tier 1 modules.

## Request Flow

```
HTTP Request
  → Echo global middleware (OTel tracing, Recovery, RequestID, RateLimit, Logger,
                            BodyLimit, Gzip, Security, CORS, Timeout, ErrorHandler)
  → Echo route group middleware: Auth (JWT validation → AuthUser injected into context)
  → Connect RPC handler
  → Connect interceptors: RBACInterceptor (permission check) → protovalidate (proto validation)
  → App handler (business logic)
  → Repository interface → Postgres adapter (sqlc queries)
  → Event publish (Watermill/RabbitMQ) — fire-and-forget after DB commit
  → Audit + Notification subscribers (async, failures logged not propagated)
  → Response to client
```

## Event Flow

```
Mutation Handler (Create/Update/Delete)
    → Persistence (repo.Create/Update/SoftDelete)
    → Extract ActorID from auth.ActorIDFromContext(ctx)
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

### Echo Global Middleware (applied to every request)
1. OTel HTTP tracing — creates span per request via `otelhttp`
2. Recovery — panic → 500 response
3. Request ID — generate/propagate X-Request-ID
4. Rate Limit — 100 req/min per IP (Redis-backed); before body parsing to prevent DDoS
5. Request Logger — structured slog with sanitized fields
6. Body Limit — 10MB max
7. Gzip — level 5 compression
8. Security Headers — HSTS, CSP, X-Frame-Options
9. CORS — configurable origins
10. Context Timeout — configurable, default 30s
11. Error Handler — centralized Connect/Domain → HTTP error mapping

### Echo Route Group Middleware (per service mount)
- **Auth** — JWT validation; injects `AuthUser` into context; returns 401 if missing/invalid

### Connect RPC Interceptors (per procedure)
- **RBACInterceptor** — maps procedure path → required permission; fail-closed (denies unmapped procedures under registered services)
- **protovalidate** — validates proto messages against buf/validate rules; returns INVALID_ARGUMENT on failure

## Key Design Decisions

- **sqlc over ORM**: Type-safe SQL without runtime reflection overhead
- **Connect RPC over REST**: HTTP/1.1 + HTTP/2 compatible, strong typing via protobuf
- **Watermill over direct RabbitMQ**: Pluggable message router with middleware support
- **fx for DI**: Compile-time dependency graph, lifecycle management
- **testcontainers**: Real Postgres/Redis/RabbitMQ in tests, no mocks for infrastructure
