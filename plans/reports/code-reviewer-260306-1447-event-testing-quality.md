# Code Review: Event System, Testing & Code Quality

**Scope:** Event bus, subscribers, notification, cron, test infra, domain model, scaffolding
**Files reviewed:** 20 files across `internal/shared/events/`, `internal/modules/audit/`, `internal/modules/notification/`, `internal/shared/cron/`, `internal/shared/mocks/`, `internal/modules/user/domain/`, `internal/modules/user/app/`, `internal/modules/user/adapters/postgres/`, `internal/shared/testutil/`, `cmd/seed/`, `cmd/scaffold/`
**LOC (hand-written):** ~850 across reviewed files

---

## Overall Assessment

Solid boilerplate with well-structured event-driven architecture. Watermill + AMQP integration is clean and production-ready for the scope. Domain model follows hexagonal architecture correctly with good encapsulation. Test infrastructure uses real dependencies (testcontainers) which is the right call. Main gaps: test coverage is thin at the app layer, audit subscriber has repetitive code, and cron starts with zero jobs.

**Score: 8.0/10** for the subsystem under review.

---

## Critical Issues

None.

---

## High Priority

### H-1: `create_user_test.go` missing error-path tests

Only 2 tests exist: happy path and email-taken. Missing coverage for:
- Invalid role (domain validation failure)
- Hasher failure (infra error)
- Repository Create failure (DB error)
- Empty email/name (domain validation)
- Event publish failure (should not roll back user creation -- verify this behavior)

The happy path test does not verify that the event was actually published. The `noopPublisher` silently discards messages, so a broken `Publish` call would pass tests.

**Recommendation:** Add at least 3 more test cases. Replace `noopPublisher` with a recording publisher that captures published messages for assertions.

```go
type recordingPublisher struct {
    messages []*message.Message
    topics   []string
}
func (p *recordingPublisher) Publish(topic string, msgs ...*message.Message) error {
    p.topics = append(p.topics, topic)
    p.messages = append(p.messages, msgs...)
    return nil
}
func (p *recordingPublisher) Close() error { return nil }
```

### H-2: Audit subscriber handler duplication

`HandleUserCreated`, `HandleUserUpdated`, `HandleUserDeleted` in `audit/subscriber.go` are nearly identical (lines 51-129). The only differences are: (1) the event struct type, (2) the `action` string. This is 80 lines of copy-paste that will drift when new event fields are added.

**Recommendation:** Extract a generic handler:

```go
func (h *Handler) handleAuditEvent(msg *message.Message, action string) error {
    // unmarshal to a common base struct or use raw JSON
    var base struct {
        UserID    string `json:"user_id"`
        ActorID   string `json:"actor_id"`
        IPAddress string `json:"ip_address"`
    }
    if err := json.Unmarshal(msg.Payload, &base); err != nil {
        slog.Error("audit: unmarshal failed", "action", action, "err", err)
        return err
    }
    entityID, err := uuid.Parse(base.UserID)
    if err != nil {
        slog.Error("audit: invalid entity ID", "err", err)
        return nil
    }
    return h.queries.CreateAuditLog(msg.Context(), sqlcgen.CreateAuditLogParams{
        EntityType: "user",
        EntityID:   entityID,
        Action:     action,
        ActorID:    parseActorID(base.ActorID, entityID),
        Changes:    msg.Payload, // raw event JSON
        IpAddress:  parseIPAddress(base.IPAddress),
    })
}
```

This reduces `subscriber.go` from 130 to ~60 lines.

---

## Medium Priority

### M-1: No dead-letter queue configuration

`NewDurableQueueConfig` creates a basic durable queue but does not configure:
- Dead-letter exchange (DLX)
- Max retry count at the AMQP level (relies solely on Watermill's 3-retry middleware)
- Message TTL for poison messages

After 3 Watermill retries, a permanently failing message will be nacked and requeued by AMQP indefinitely, creating an infinite retry loop.

**Recommendation:** Configure AMQP dead-letter exchange in `NewSubscriber`:
```go
amqpCfg.Queue.Arguments = amqp091.Table{
    "x-dead-letter-exchange": "dlx",
    "x-dead-letter-routing-key": "dead-letter",
}
```

### M-2: `json.Marshal(event)` error silently ignored in audit handlers

Lines 65-66 in `subscriber.go`:
```go
changes, _ := json.Marshal(event)
```
The `_` discards a marshal error. While unlikely (the event was just unmarshalled from JSON), this violates defensive coding. Same pattern at lines 92 and 119.

**Recommendation:** Log or return the error.

### M-3: Audit module creates its own `sqlcgen.Queries` instance

In `audit/module.go` line 12-14:
```go
fx.Provide(func(pool *pgxpool.Pool) *sqlcgen.Queries {
    return sqlcgen.New(pool)
})
```

If another module also provides `*sqlcgen.Queries`, Fx will error on duplicate providers. This should use `fx.Private` or a named type.

**Recommendation:** Add `fx.Private` annotation or use a module-scoped type alias:
```go
fx.Provide(fx.Private, func(pool *pgxpool.Pool) *sqlcgen.Queries {
    return sqlcgen.New(pool)
})
```

### M-4: Cron scheduler starts with zero registered jobs

`cron/scheduler.go` is well-written with Redis distributed locking, but no module registers any jobs. The cron goroutine runs idle. This is by-design for a boilerplate, but should be documented with a comment.

### M-5: Event bus `Publish` failure is fire-and-forget

In `create_user.go` lines 67-78, if event publishing fails, the error is logged but the user creation is still returned as successful. This is the correct pattern (don't fail the user operation due to event infra), but it means events can be silently lost.

For a boilerplate this is acceptable. For production, consider an outbox pattern.

### M-6: No tests for event subscribers (audit, notification)

Zero test files exist for `audit/subscriber.go` or `notification/subscriber.go`. These are critical paths that persist audit logs and send emails.

**Recommendation:** Add unit tests with a mock `sqlcgen.Queries` for audit and a mock `Sender` for notification. The `Sender` interface is already clean for mocking.

---

## Low Priority

### L-1: `UserFixture` in `testutil/fixtures.go` uses `string` for Role

`testutil.UserFixture` has `Role string` instead of `domain.Role`. This forces callers to use raw strings and loses type safety.

### L-2: SMTP `Send` does not use context

`SMTPSender.Send` accepts `context.Context` but ignores it (line 29: `_ context.Context`). The `smtp.SendMail` stdlib function does not support context cancellation, so this is a stdlib limitation, but worth noting for future replacement with a context-aware SMTP library.

### L-3: `cmd/scaffold/main.go` validateIdentifier rejects digits

Line 171: `!unicode.IsLetter(r) && r != '_'` rejects digits, so module names like `oauth2` would fail. This is intentional (Go package names should be alpha), but could surprise users.

### L-4: Welcome email template is hardcoded

The HTML template in `notification/subscriber.go` lines 49-56 is embedded as a string constant. For a boilerplate this is fine, but production use would benefit from `embed.FS` templates.

### L-5: Cron `Stop()` waits correctly

I previously noted cron Stop() not waiting -- this was fixed. `<-s.cron.Stop().Done()` properly waits for running jobs. Good.

---

## Positive Observations

1. **Watermill retry middleware** with `MaxRetries: 3` and `InitialInterval: 1s` -- good defaults for transient failures.
2. **OTel trace propagation** in event bus `Publish` -- traces flow from HTTP through AMQP to subscribers. Excellent observability.
3. **Cancellable context** for Watermill router lifecycle -- `context.WithCancel` + `router.Close()` on stop is correct.
4. **AMQP shutdown** properly closes both publisher and subscriber on Fx stop.
5. **Testcontainers** for real Postgres/Redis/RabbitMQ -- no fake DB mocks for integration tests.
6. **Migration runner** in testutil extracts goose Up SQL correctly -- avoids goose dependency in tests.
7. **Domain model encapsulation** -- unexported fields, getters, `Reconstitute()` pattern, business rule validation in constructors.
8. **Sentinel errors** with DomainError type mapping to HTTP + gRPC codes -- clean error taxonomy.
9. **Scaffold generator** is comprehensive (20 files per module) with conflict checking and reserved-word validation.
10. **Seed script** is idempotent -- checks `GetByEmail` before creating.

---

## Edge Cases Found

1. **Audit ActorID fallback**: If `ActorID` is empty (system-triggered event), it falls back to `EntityID`. This means the audit log shows the user created themselves. Documented as by-design, but a `system` UUID constant would be clearer.

2. **Event publish after DB write without outbox**: If the app crashes between `repo.Create` and `bus.Publish`, the user exists in DB but no event fires. Audit log and welcome email are lost. Acceptable for boilerplate; outbox pattern needed for production.

3. **SMTP CRLF injection**: Properly mitigated with `sanitize()` function in `email.go`. The subject also uses `mime.QEncoding` which is correct.

4. **Cron lock expiry**: The 5-minute lock TTL means a job running >5 minutes will release its lock, allowing a second instance to start. Long-running jobs need their own TTL management.

5. **`NewDurableQueueConfig` shared between pub and sub**: Both publisher and subscriber use identical AMQP config. This works for simple fan-out but means each subscriber gets its own queue (not shared consumer group). For this boilerplate's scale, this is fine.

---

## Test Coverage Summary

| Layer | Files | Tests | Coverage |
|-------|-------|-------|----------|
| Domain (`user/domain/`) | 3 | 8 tests (user_test.go) | Good -- validates creation, mutation, validation errors |
| App (`user/app/`) | 1 test file | 2 tests | **Weak** -- only happy path + email-taken |
| Adapter (`postgres/`) | 1 test file (integration) | 5 tests | Good -- CRUD, pagination, soft-delete, not-found |
| Event subscribers | 0 test files | 0 tests | **Missing** |
| Notification | 0 test files | 0 tests | **Missing** |
| Cron | 0 test files | 0 tests | **Missing** |

**Estimated overall test coverage: ~40-50%** for hand-written code. Domain layer is well-tested; app and infra layers need work.

---

## Recommended Actions (Priority Order)

1. **Add error-path tests to `create_user_test.go`** -- invalid role, hasher failure, repo failure, event publish verification (H-1)
2. **Add unit tests for audit subscriber** with mock Queries (M-6)
3. **Add unit tests for notification subscriber** with mock Sender (M-6)
4. **Extract generic audit handler** to eliminate 80 lines of duplication (H-2)
5. **Configure dead-letter exchange** for poison messages (M-1)
6. **Add `fx.Private`** to audit module's Queries provider (M-3)
7. **Handle `json.Marshal` errors** in audit subscriber (M-2)

---

## Unresolved Questions

1. Should the outbox pattern be implemented now or deferred as a future enhancement?
2. Is the `testutil.UserFixture` type intentionally using `string` for Role to stay decoupled from domain?
3. Should notification subscriber retry on SMTP failure or dead-letter after 3 retries?
