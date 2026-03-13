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

- **Postgres unavailable** — handler returns an error; Watermill requeues the message for retry
- **Bad event payload (schema mismatch)** — acked immediately (permanent failure, retrying won't help); error logged with `module=audit`
- **Invalid UUID in event** — acked immediately; error logged
