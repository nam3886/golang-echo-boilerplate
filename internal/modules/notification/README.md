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

Watermill provides at-least-once delivery. `HandleUserCreated` deduplicates using Redis `SET NX` on `msg.UUID` (TTL = 24h) before calling `sender.Send`. Duplicate welcome emails on Redis miss are tolerable (low frequency, low impact).

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

### Welcome Email Idempotency

`HandleUserCreated` uses Redis `SET NX msg.UUID` dedup (implemented in `subscriber.go`). On Redis miss or Redis unavailability, a duplicate email may be sent — acceptable for welcome emails (no financial/compliance impact).

**⚠️ Do NOT skip Redis dedup for financial, billing, or compliance-sensitive notifications.**
