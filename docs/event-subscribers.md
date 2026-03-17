# Event Subscribers

## Publisher Abstraction Levels

The event system uses three publisher layers — depend on the interface, never on implementations:

- **`events.EventPublisher` interface** — Used by app layer (handlers, domain). Only public abstraction.
- **`events.EventBus` struct** — Internal wrapper that marshals events to JSON and publishes via Watermill. Retry logic and dead-letter routing are handled by the subscriber router middleware (`subscriber.go`), not by EventBus.
- **`message.Publisher` (Watermill)** — Raw message broker interface. Never use directly in app code.

Always inject `events.EventPublisher` into handlers; fx wires it from `EventBus`.

## Event Contracts Location

All shared event types and topic constants live in:
`internal/shared/events/contracts/` (e.g., `user_events.go`, `{name}_events.go` per module)

External modules must import from `contracts` directly — never from another module's
`domain/` package. This preserves the no-cross-module-imports rule.

Topic constants: `TopicUserCreated`, `TopicUserUpdated`, `TopicUserDeleted`.

### Import Convention

- **Within the owning module** (e.g., search indexer inside `user/adapters/search/`):
  Import from `domain/events.go` — acceptable because the adapter is part of the same module.
- **Cross-module subscribers** (e.g., audit, notification):
  Import from `internal/shared/events/contracts/` — required by the no-cross-module-imports rule.

The `domain/events.go` file re-exports contracts via type aliases for ergonomic internal use.
External subscribers must always use `contracts` to avoid coupling between modules.

## How to Subscribe

Implement `message.NoPublishHandlerFunc` — unmarshal the payload, process, return `nil` to ack or
a non-nil error to nack. Always return `nil` on schema errors (retrying won't fix them).

## Registration Pattern

Return `[]events.HandlerRegistration` tagged with `group:"event_handlers"`.
The router collects all registrations from every module at startup.

```go
fx.Provide(fx.Annotate(
    provideMyHandlers,
    fx.ResultTags(`group:"event_handlers"`),
))

func provideMyHandlers(h *MyHandler) []events.HandlerRegistration {
    return []events.HandlerRegistration{
        {Name: "mymodule.user_created", Topic: contracts.TopicUserCreated, HandlerFunc: h.HandleUserCreated},
    }
}
```

`HandlerRegistration` fields — `Name` (unique across all handlers), `Topic`, `HandlerFunc`.

Source: `internal/shared/events/subscriber.go`

## Logging Convention

Use `slog.ErrorContext(ctx, "...", "handler", h.Name, "err", err)` inside handler methods so
structured logs carry the request context (trace ID, request ID). Use `slog.WarnContext` for
recoverable issues (e.g., schema mismatches that are acked). Never use `log.Printf` or
bare `fmt.Println` in subscriber code.

## Reference Implementation

The audit subscriber is the canonical example.

- Handler methods: `internal/modules/audit/subscriber.go`
- Registration: `provideHandlers` in `internal/modules/audit/module.go`

The subscriber returns `nil` (ack) on schema errors and a real error (nack) on DB failures,
triggering the router's retry middleware (3 retries, 1s initial, 0.5 jitter).
Full module wiring reference: `internal/modules/audit/module.go`.

## Dead Letter Queue

Each handler queue is configured with `x-dead-letter-exchange: dlx`.
After all retries are exhausted, the message is routed to `{topic}.dlq`
(e.g. `user.created.dlq`) via the `dlx` direct exchange.

DLQ queues are declared automatically at startup via `DeclareDLQQueues`.
Failure to declare is non-fatal — a warning is logged and the service starts anyway.

Source: `internal/shared/events/dlq.go`

## Error Handling

| Handler return | Watermill action |
|----------------|-----------------|
| `nil` | Ack — message consumed |
| `error` | Nack — retry up to 3x with backoff, then dead-letter |

Always ack (`return nil`) on unrecoverable errors (bad schema, missing data).
Only return errors for transient failures (network, DB, ES unavailable).

## Idempotency Patterns

Some event handlers require idempotency to safely handle message retries without side effects.

### Notification Handler (Redis-backed Deduplication)

The notification subscriber deduplicates messages using Redis SET NX:

```go
const dedupTTL = 24 * time.Hour

// Dedup check uses event.EventID (not msg.UUID) so publisher retries
// with a new Watermill message are still caught as duplicates.
dedupKey := event.EventID
if dedupKey == "" {
    dedupKey = msg.UUID // fallback for legacy events without EventID
}
if h.rdb != nil {
    exists, err := h.rdb.Exists(ctx, "notification:dedup:"+dedupKey).Result()
    if err != nil {
        // Fail-open: Redis issue should not block email delivery.
        slog.WarnContext(ctx, "notification: dedup check failed, proceeding to send", ...)
    } else if exists > 0 {
        slog.InfoContext(ctx, "notification: duplicate message, skipping", ...)
        return nil
    }
}

// ... send email ...

// Commit dedup key AFTER successful send — crash before this point allows redelivery.
if h.rdb != nil {
    h.rdb.SetNX(ctx, "notification:dedup:"+dedupKey, "1", dedupTTL)
}
```

**Strategy:** Use `event.EventID` as the dedup key (not Watermill's `msg.UUID`). Publisher retries may produce a new Watermill UUID but always carry the same `EventID`, so duplicates are caught. The dedup key is committed **after** a successful send — a crash between send and commit allows redelivery, preventing silent message loss.

**Fail-open behavior:** If Redis is unavailable, the error is logged as a warning and email delivery proceeds. This prioritizes availability over strict at-most-once semantics. Enable only if email duplication is acceptable and Redis HA is not available.

### Audit Handler (Database-backed Idempotency)

The audit subscriber uses the database's primary key constraint for idempotency:

```go
// Use Watermill message UUID as audit log primary key.
// Falls back to a deterministic UUID derived from event.EventID so that
// publisher-side retries with different Watermill UUIDs still collide on
// ON CONFLICT (id) DO NOTHING.
msgID, err := uuid.Parse(msg.UUID)
if err != nil {
    var baseEvent struct {
        EventID string `json:"event_id"`
    }
    if jsonErr := json.Unmarshal(msg.Payload, &baseEvent); jsonErr == nil && baseEvent.EventID != "" {
        msgID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(baseEvent.EventID))
    } else {
        msgID = uuid.NewSHA1(uuid.NameSpaceOID, msg.Payload)
    }
    slog.WarnContext(msg.Context(), "audit: invalid msg UUID, using event-derived deterministic ID", ...)
}

return h.writer.CreateAuditLog(msg.Context(), sqlcgen.CreateAuditLogParams{
    ID: msgID,  // Primary key — ON CONFLICT (id) DO NOTHING silently deduplicates
    // ...
})
```

The SQL query includes `ON CONFLICT (id) DO NOTHING`, which silently ignores duplicate inserts when a retry has the same ID. The deterministic fallback (`uuid.NewSHA1`) ensures that even if the Watermill UUID changes between retries, the same `EventID` produces the same derived UUID — preserving idempotency.

**Advantage:** No separate dedup cache needed — the database enforces idempotency via primary key constraint.

## Creating New Event Types

1. Add topic constants and event structs to `internal/shared/events/contracts/`.
2. Publish from the owning module's app handler via `events.EventBus.Publish(ctx, topic, event)`.
3. Subscribers import from `contracts`, never from the publishing module's `domain/`.
4. Register handlers with `group:"event_handlers"` tag as shown above.
5. If idempotency is required, use either Redis dedup (fail-open, requires HA) or database primary key (fail-closed, more reliable).
