# Audit Module

## Purpose

Records an immutable audit trail of all user-affecting operations by subscribing to domain events from RabbitMQ.

## Structure

```
subscriber.go    — Watermill event handlers (HandleUserCreated/Updated/Deleted)
module.go        — fx.Module wiring
```

## Events Consumed

| Topic | Handler | Action recorded |
|-------|---------|-----------------|
| `user.created` | `HandleUserCreated` | `created` |
| `user.updated` | `HandleUserUpdated` | `updated` |
| `user.deleted` | `HandleUserDeleted` | `deleted` |

## Idempotency

Uses Watermill `msg.UUID` as the `audit_logs` primary key. `ON CONFLICT (id) DO NOTHING` deduplicates redeliveries — safe for at-least-once delivery.

## Dependencies

- `gen/sqlc` — generated `CreateAuditLog` query
- `internal/shared/events/contracts` — event struct definitions
- No imports from other domain modules

## Failure Modes

| Dependency | Failure | Behavior |
|------------|---------|----------|
| PostgreSQL | Unavailable | Handler returns error; Watermill requeues for retry |
| RabbitMQ | Unavailable | Events not consumed; audit rows delayed until reconnect |
| Bad event payload | Schema mismatch | Acked immediately (permanent); error logged with `module=audit` |
| msg.UUID | Invalid UUID | Idempotency compromised; warned and a new UUID is generated |
