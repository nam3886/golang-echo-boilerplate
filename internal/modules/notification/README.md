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

- **SMTP transient error (4xx)** — handler returns error; Watermill requeues for retry
- **SMTP permanent error (5xx)** — acked immediately; logged with `module=notification`. Bad addresses won't be fixed by retrying.
- **Template render failure** — acked immediately to avoid infinite retry loop; fix template and redeploy
- **Schema mismatch** — acked immediately; error logged with `module=notification`
