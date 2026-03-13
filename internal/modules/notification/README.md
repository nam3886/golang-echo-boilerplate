# Notification Module

## Purpose

Sends transactional emails (welcome, etc.) by subscribing to domain events via RabbitMQ and delivering through SMTP.

## Structure

```
subscriber.go    — Watermill event handler (HandleUserCreated → welcome email)
sender.go        — Sender interface + SMTP implementation
module.go        — fx.Module wiring
```

## Events Consumed

| Topic | Handler | Action |
|-------|---------|--------|
| `user.created` | `HandleUserCreated` | Sends welcome email via SMTP |

## Idempotency

Watermill provides at-least-once delivery. Duplicate welcome emails on retry are tolerable (low frequency, low impact). For stricter dedup, add a Redis `SET NX` check on `msg.UUID` with TTL matching the message retention period before calling `sender.Send`.

## Dependencies

- `internal/shared/events/contracts` — event struct definitions
- SMTP server (configured via `SMTP_*` env vars; Mailpit in dev)
- No imports from other domain modules

## Failure Modes

| Dependency | Failure | Behavior |
|------------|---------|----------|
| SMTP | Transient error (4xx) | Handler returns error; Watermill requeues for retry |
| SMTP | Permanent error (5xx) | Acked immediately; logged with `module=notification`. Bad addresses won't be fixed by retrying. |
| SMTP | Template render failure | Acked immediately to avoid infinite retry loop; fix template and redeploy |
| RabbitMQ | Unavailable | Events not consumed; welcome emails delayed until recovery. No data loss with durable queues. |
| Schema | Mismatch | Acked immediately; error logged with `module=notification` |

## Design Decisions

### Non-Idempotent Welcome Email (C4 Exception)

Welcome emails use fire-and-forget delivery. Duplicate emails on Watermill retry are tolerated.

**Rationale:** Welcome emails have no financial or compliance impact. Adding Redis `SET NX` dedup would introduce infra complexity disproportionate to the risk.

**⚠️ Do NOT copy this pattern for financial, billing, or compliance-sensitive notifications.** Those must use Redis `SET NX msg.UUID` dedup before calling `sender.Send`.
