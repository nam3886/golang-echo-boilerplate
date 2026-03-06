# Code Review — Boilerplate 6-Fix Batch
**Date:** 2026-03-05
**Reviewer:** code-reviewer agent
**Build status:** PASS (go build ./... clean)

---

## Scope

| # | File | Change |
|---|------|--------|
| 1 | `internal/shared/events/topics.go` | Added `ActorID` to all three event structs |
| 2 | `internal/modules/user/app/create_user.go` | Extract actor, pass ActorID in event |
| 3 | `internal/modules/user/app/update_user.go` | Added EventBus dep, publish UserUpdatedEvent |
| 4 | `internal/modules/user/app/delete_user.go` | Added EventBus dep, publish UserDeletedEvent |
| 5 | `internal/modules/audit/subscriber.go` | Use `event.ActorID` via `parseActorID` helper |
| 6 | `internal/modules/user/domain/repository.go` | List signature returns `([]*User, string, bool, error)` |
| 7 | `internal/modules/user/adapters/postgres/repository.go` | Pagination probe, SoftDelete row-check, Create 23505 handling |
| 8 | `internal/modules/user/app/list_users.go` | Simplified to consume new List signature |
| 9 | `db/queries/user.sql` | SoftDeleteUser `:exec` → `:execrows` |
| 10 | `internal/modules/user/adapters/grpc/routes.go` | Added `validate.NewInterceptor()` |
| 11 | `go.mod` | Added `connectrpc.com/validate v0.6.0` |

---

## Overall Assessment

All 11 changes are **correct and consistent**. The build is clean with zero errors. The fixes address the three critical issues documented in agent memory (C-2 pagination skip bug, C-3 silent soft-delete, I-3 missing protovalidate) plus the audit ActorID regression (I-5). The implementation is lean and follows existing patterns throughout.

---

## Critical Issues

None. No regressions introduced.

---

## High Priority

### H-1: `connectrpc.com/validate` is marked `// indirect` in go.mod

**File:** `go.mod` line 34

```
connectrpc.com/validate v0.6.0 // indirect
```

It is directly imported in `internal/modules/user/adapters/grpc/routes.go`. The `// indirect` comment is incorrect — it should be a direct dependency in the `require` block at the top alongside `connectrpc.com/connect`.

**Impact:** Cosmetically wrong; `go mod tidy` will fix it automatically. If the project enforces `go mod tidy` in CI, the build would fail there. Low runtime risk.

**Fix:** Move to the direct-dependency block and remove `// indirect`:
```
connectrpc.com/validate v0.6.0
```

---

### H-2: `create_user.go` still has a TOCTOU race for email uniqueness

**File:** `internal/modules/user/app/create_user.go` lines 39–45

The fix in this batch adds 23505 handling at the postgres layer (fix #7), which is the correct safety net. However, the app layer `GetByEmail` check at lines 39–44 remains and creates a check-then-act window. The two layers now both guard against duplicate email, but the order creates a subtle question:

- If two concurrent requests pass the `GetByEmail` check simultaneously, both will attempt `repo.Create`, the second will get `domain.ErrEmailTaken` from the 23505 handler — which is correct.
- The `GetByEmail` check is now redundant. It adds a round-trip and the TOCTOU window without providing extra safety.

**Recommendation:** Remove the `GetByEmail` pre-check from `create_user.go` entirely and rely solely on the postgres 23505 handler. This simplifies the code and eliminates the race.

```go
// Handle creates a new user.
func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCmd) (*domain.User, error) {
    hashedPwd, err := h.hasher.Hash(cmd.Password)
    if err != nil {
        return nil, fmt.Errorf("hashing password: %w", err)
    }

    user, err := domain.NewUser(cmd.Email, cmd.Name, hashedPwd, domain.Role(cmd.Role))
    if err != nil {
        return nil, err
    }

    if err := h.repo.Create(ctx, user); err != nil {
        return nil, fmt.Errorf("creating user: %w", err) // ErrEmailTaken surfaces here
    }
    // ... publish event
}
```

Note: this is a pre-existing issue that the 23505 fix in this batch partially addresses. The redundancy introduced by having both guards is the new concern.

---

## Medium Priority

### M-1: Pagination cursor correctness — confirm row ordering matches SQL

**File:** `internal/modules/user/adapters/postgres/repository.go` lines 82–93

The SQL orders `created_at DESC, id DESC`. The cursor is built from `last.CreatedAt()` and `last.ID()` (the last kept row after truncation). The SQL keyset predicate is:

```sql
(created_at, id) < (cursor_created_at, cursor_id)
```

This is correct for DESC ordering: the next page starts after (i.e., strictly less than) the cursor values. The fix of building the cursor from the last **kept** row (index `limit-1`) rather than the probe row (index `limit`) is correct — the probe row is discarded; using it as a cursor would skip a record.

**Verification:** The logic is:
1. Fetch `limit+1` rows
2. `hasMore = len(rows) > limit` (correct, uses raw row count before conversion)
3. `users = users[:limit]` (truncate)
4. Cursor from `users[len(users)-1]` (last kept) — correct

No issues found here. Documented as confirmed correct.

---

### M-2: `HandleUserUpdated` and `HandleUserDeleted` suppress unmarshal errors differently

**File:** `internal/modules/audit/subscriber.go`

`HandleUserCreated` (line 36–38) logs the error before returning it:
```go
slog.Error("audit: failed to unmarshal user created event", "err", err)
return err
```

`HandleUserUpdated` (line 62–63) silently returns the error without logging:
```go
if err := json.Unmarshal(msg.Payload, &event); err != nil {
    return err
}
```

`HandleUserDeleted` has the same silent pattern (line 87–88).

**Impact:** Unmarshal errors on updated/deleted events will cause Watermill to nack/retry the message without any log trace. Debugging poison messages becomes harder.

**Fix:** Add `slog.Error(...)` before `return err` in `HandleUserUpdated` and `HandleUserDeleted`, matching the pattern in `HandleUserCreated`.

---

### M-3: `HandleUserDeleted` does not record changes payload

**File:** `internal/modules/audit/subscriber.go` lines 98–103

`HandleUserCreated` and `HandleUserUpdated` both marshal the full event as `changes`:
```go
changes, _ := json.Marshal(event)
```

`HandleUserDeleted` does not:
```go
return h.queries.CreateAuditLog(ctx, sqlcgen.CreateAuditLogParams{
    EntityType: "user",
    EntityID:   entityID,
    Action:     "deleted",
    ActorID:    parseActorID(event.ActorID, entityID),
    // changes omitted
})
```

**Impact:** The audit schema likely has a nullable `changes` column, so this will not crash. But the audit record for deletions carries no payload, making forensic queries inconsistent. At minimum, recording `{"user_id": ..., "actor_id": ..., "at": ...}` helps.

**Fix:**
```go
changes, _ := json.Marshal(event)
return h.queries.CreateAuditLog(ctx, sqlcgen.CreateAuditLogParams{
    EntityType: "user",
    EntityID:   entityID,
    Action:     "deleted",
    ActorID:    parseActorID(event.ActorID, entityID),
    Changes:    changes,
})
```

---

### M-4: `parseActorID` fallback hides invalid ActorID strings silently

**File:** `internal/modules/audit/subscriber.go` lines 24–31

```go
func parseActorID(actorIDStr string, entityID uuid.UUID) uuid.UUID {
    if actorIDStr != "" {
        if parsed, err := uuid.Parse(actorIDStr); err == nil {
            return parsed
        }
    }
    return entityID
}
```

If `actorIDStr` is non-empty but not a valid UUID (e.g., a corrupt event payload), it silently falls back to `entityID`. The audit record then records the wrong actor with no indication of the parse failure.

**Fix:** Log a warning when the fallback fires:
```go
func parseActorID(actorIDStr string, entityID uuid.UUID) uuid.UUID {
    if actorIDStr != "" {
        if parsed, err := uuid.Parse(actorIDStr); err == nil {
            return parsed
        }
        slog.Warn("audit: invalid actor_id in event, falling back to entity_id",
            "actor_id", actorIDStr, "entity_id", entityID)
    }
    return entityID
}
```

---

## Low Priority

### L-1: `create_user.go` ActorID block is empty string when no auth context

The fallback `actorID = ""` (zero value) is passed to the event when the creator has no auth context (e.g., system-bootstrap or unauthenticated route). The `parseActorID` helper then falls back to `entityID`. This is intentional per the design, but it means bootstrap-created users appear as "self-created" in audit. Acceptable for a boilerplate, worth noting in docs.

---

### L-2: `decodeCursor` ignores malformed cursor silently

**File:** `internal/modules/user/adapters/postgres/repository.go` lines 66–69

```go
if cursor != "" {
    decoded, err := decodeCursor(cursor)
    if err == nil {
        // apply cursor
    }
}
```

A malformed cursor simply results in a first-page query without error. This is defensively correct (no panic), but clients that pass a corrupted cursor will get page 1 silently. A logged warning would help diagnose client bugs:

```go
if decoded, err := decodeCursor(cursor); err == nil {
    // apply
} else {
    slog.Warn("list users: ignoring malformed cursor", "err", err)
}
```

---

### L-3: `validate.NewInterceptor()` has no error return — but returns `(*Interceptor, error)` in older versions

Confirmed from docs: v0.6.0 signature is `NewInterceptor(opts ...Option) *Interceptor` with no error. The current usage `validate.NewInterceptor()` is correct. No issue.

---

### L-4: `go.mod` missing `go.sum` verification note

The `connectrpc.com/validate v0.6.0` addition should be accompanied by an updated `go.sum`. Assuming `go mod tidy` was run, this is handled automatically. Not a code issue, but worth verifying in CI that `go.sum` is committed.

---

## Positive Observations

- Pagination fix is architecturally correct: probe row at repo layer, cursor from last kept row. The app layer is now cleanly decoupled from pagination mechanics.
- `SoftDelete` returning `sharederr.ErrNotFound` for zero affected rows is the correct idempotency boundary — the error propagates cleanly through `domainErrorToConnect` to `CodeNotFound`.
- 23505 unique violation mapping at the postgres layer is the right place (not app layer). `errors.As(err, &pgErr)` is the correct pgx v5 pattern.
- `parseActorID` fallback-to-entityID logic is a good defensive design that prevents nil-UUID panics in the audit DB write.
- ActorID extraction pattern (`auth.UserFromContext(ctx)`) is consistent across all three handlers (create, update, delete).
- `validate.NewInterceptor()` placement as a Connect interceptor means validation runs before handler code — correct layer.
- Event publish errors are logged and non-fatal (fire-and-forget pattern) consistently across all three app handlers.
- `toDomain` uses `row.CreatedAt` (from DB) for the cursor, which is the authoritative timestamp. `user.CreatedAt()` in the cursor builder reflects this correctly since `Reconstitute` sets it from the DB value.

---

## Recommended Actions (Prioritized)

1. **[H-1]** Run `go mod tidy` and commit updated `go.mod`/`go.sum` to move `connectrpc.com/validate` to the direct-dependency block.
2. **[H-2]** Remove the `GetByEmail` pre-check from `create_user.go`; rely solely on the 23505 handler in the postgres adapter.
3. **[M-2]** Add `slog.Error` logging for unmarshal failures in `HandleUserUpdated` and `HandleUserDeleted`.
4. **[M-3]** Record `changes` payload in `HandleUserDeleted` audit log entry.
5. **[M-4]** Add a `slog.Warn` in `parseActorID` when the non-empty string is not a valid UUID.
6. **[L-2]** Log a warning in `List` when a non-empty cursor fails to decode.

---

## Metrics

| Metric | Value |
|--------|-------|
| Files reviewed | 11 changed + 6 context |
| Build status | PASS |
| Compile errors | 0 |
| Critical issues | 0 |
| High priority | 2 |
| Medium priority | 4 |
| Low priority | 4 |

---

## Unresolved Questions

1. Does the audit `changes` column have a NOT NULL constraint? If so, M-3 is a latent data error for deletions, not just cosmetic.
2. Will CI enforce `go mod tidy --check`? If yes, H-1 will fail the build.
3. Is the `GetByEmail` pre-check in `create_user.go` intentional for returning a more user-friendly error message before attempting the insert? If so, the TOCTOU race is accepted and H-2 can be closed.
