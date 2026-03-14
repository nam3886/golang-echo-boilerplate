# User Module

## Purpose

Manages user accounts: creation, profile updates, soft deletion, role assignment, and authentication-related queries.

## Structure

```
domain/          — User entity, UserRepository interface, domain errors
app/             — Use-case handlers (Create, Get, List, Update, Delete)
adapters/
  postgres/      — sqlc-based PgUserRepository + integration tests
  grpc/          — Connect RPC handler, routes, proto↔domain mapper
  search/        — Elasticsearch indexer (optional, nil-safe)
module.go        — fx.Module wiring
```

## Key Entities

- `User` — core aggregate with unexported fields; construct via `NewUser()`, mutate via `ChangeEmail/ChangeName/ChangeRole`, read via getters

## Events Published

| Topic | Event | When |
|-------|-------|------|
| `user.created` | `UserCreatedEvent` | After successful Create |
| `user.updated` | `UserUpdatedEvent` | After field change (skipped if nothing changed) |
| `user.deleted` | `UserDeletedEvent` | After SoftDelete |

## Pagination

⚠️ `page_size` is clamped server-side: `0 → 20`, `>100 → 100`. The effective `page_size` is reflected back in the response — clients must not assume the requested value was honored.

## Dependencies

- `internal/shared/auth` — JWT, password hashing, context helpers
- `internal/shared/events` — event bus publishing
- `internal/shared/middleware` — RBAC, auth middleware
- `internal/shared/errors` — domain error constructors
- No imports from other domain modules

## Failure Modes

- **Postgres unavailable** — fail-closed; all handlers return 5xx
- **Redis unavailable** — auth middleware fails open for token blacklist reads (configurable; see `auth.go`)
- **Elasticsearch unavailable** — fail-open; indexer is nil-safe, search errors are logged but do not block the request
- **RabbitMQ unavailable** — event publish errors are logged at ERROR level but do not fail the handler (fire-and-forget after DB commit)
