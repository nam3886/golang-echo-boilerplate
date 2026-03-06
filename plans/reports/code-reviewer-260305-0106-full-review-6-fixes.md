# Code Review: 6 Boilerplate Fixes
Date: 2026-03-05
Build status at review: `go build ./...` PASS, `go vet ./...` PASS

---

## 1. Summary — PASS WITH NOTES

All 6 fixes are directionally correct and compile cleanly. No critical regressions introduced. Three medium-priority issues require attention before the code can be considered production-ready, and one low-priority inconsistency in the pagination logic is worth documenting.

---

## 2. Per-Fix Verdict

### Fix 1 — Pagination cursor skip fix
**Verdict: CORRECT, with one edge-case gap**

Files: `domain/repository.go`, `adapters/postgres/repository.go`, `app/list_users.go`

The probe-row pattern (fetch `limit+1`, truncate, build cursor from last kept row) is the correct solution to the "skip one record per page" bug. The implementation in `repository.go` lines 63–95 is logically sound:

- `params.Limit = int32(limit + 1)` — correct probe
- `hasMore := len(users) > limit` — correct detection
- `users = users[:limit]` — correct truncation
- Cursor built from `users[len(users)-1]` after truncation — correct: points at the last row the caller receives, not at the (limit+1)th row

The `list_users.go` simplification is clean; the handler properly caps `limit` at 100 and defaults to 20.

Gap (see edge case analysis, item 1).

---

### Fix 2 — SoftDelete no-op fix
**Verdict: CORRECT**

Files: `db/queries/user.sql`, `gen/sqlc/user.sql.go`, `adapters/postgres/repository.go`

- SQL changed from `:exec` to `:execrows` — correct directive
- Generated `SoftDeleteUser` now returns `(int64, error)` — confirmed at line 157
- Repository checks `rows == 0` and returns `sharederr.ErrNotFound` — correct

The `WHERE deleted_at IS NULL` clause means a second soft-delete on the same user correctly returns 0 rows, which is mapped to `ErrNotFound`. This is the expected idempotency behavior.

---

### Fix 3 — Unique violation TOCTOU fix
**Verdict: PARTIALLY CORRECT — medium-priority issue remains**

File: `adapters/postgres/repository.go` lines 107–109

```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "23505" {
    return domain.ErrEmailTaken
}
```

The pgconn 23505 catch is correct for eliminating the TOCTOU window. However, the pre-check in `create_user.go` lines 39–45 still performs `GetByEmail` before `Create`. This means:

1. The pre-check is now redundant — the DB constraint is the authoritative guard.
2. The pre-check adds a network round-trip on every CreateUser call.
3. More importantly: the pre-check only guards against non-deleted email matches (`WHERE deleted_at IS NULL`). If a soft-deleted user's email is being re-used, the pre-check passes, then the insert either succeeds (OK) or hits a unique constraint on the un-filtered email column (depends on schema).

**Overmatch concern (from review checklist):** `pgErr.Code == "23505"` will fire for ANY unique constraint violation on the `users` table, not only the email column. If a future unique index is added (e.g., on `name` or an external ID), it would be misreported as `ErrEmailTaken`. Mitigation: also check `pgErr.ConstraintName == "users_email_key"` (or whichever the actual constraint name is).

Severity: Medium. Wrong error message to the caller when a different constraint is violated; not a data integrity issue.

---

### Fix 4 — Events in Update/Delete
**Verdict: CORRECT**

Files: `app/update_user.go`, `app/delete_user.go`

Event publishing happens after the successful DB operation in both handlers. Publish failures are logged with `slog.ErrorContext` and do not block the response (fire-and-forget). This matches the `create_user.go` convention.

One observation: in `update_user.go`, the `updated` variable is set inside the closure at line 45 (`updated = user`). If the closure returns an error, `updated` remains `nil`. After the closure, line 48 checks `if err != nil { return nil, err }`. So `updated` is only non-nil when the closure succeeded — this is safe.

---

### Fix 5 — ActorID in events + audit
**Verdict: CORRECT**

Files: `events/topics.go`, `app/create_user.go`, `app/update_user.go`, `app/delete_user.go`, `audit/subscriber.go`

- `ActorID` field added to all three event structs — consistent
- All three handlers extract `ActorID` from `auth.UserFromContext(ctx)` using the same pattern
- `parseActorID` in audit subscriber: falls back to `entityID` when `actorID` is empty string or unparseable, logs a `slog.Warn` on parse failure — correct

The remaining semantic limitation (actorID == entityID when no auth context, e.g., system actions) is now made explicit by the fallback + warning. That is an acceptable trade-off for a boilerplate and is tracked in memory as I-5.

---

### Fix 6 — Protovalidate interceptor
**Verdict: CORRECT**

File: `adapters/grpc/routes.go` lines 17–19

```go
path, h := userv1connect.NewUserServiceHandler(handler,
    connect.WithInterceptors(validate.NewInterceptor()),
)
```

`connectrpc.com/validate` v0.6.0 is present in `go.mod` at line 8. The interceptor is registered at the handler level (not global), which is consistent with the existing auth middleware approach.

`validate.NewInterceptor()` returns an error in some versions of the library. Check the actual `validate` package signature — if `NewInterceptor()` has an `(Interceptor, error)` signature, this line silently drops the error. At v0.6.0 the constructor returns only the interceptor (no error), so this is currently safe. Pin the version to avoid a silent break on upgrade.

---

## 3. Issues Found

### Medium

**M-1: `pgErr.Code == "23505"` overmatch**
- File: `internal/modules/user/adapters/postgres/repository.go:108`
- Problem: Any unique constraint violation on the users table is returned as `domain.ErrEmailTaken`. If a second unique index is added later (e.g., on phone number), the error message will be wrong.
- Fix: Add `&& pgErr.ConstraintName == "users_email_key"` (verify actual constraint name from migration). Fall through to generic error for other 23505 violations.

**M-2: Redundant pre-check + TOCTOU not fully eliminated**
- File: `internal/modules/user/app/create_user.go:39–45`
- Problem: `GetByEmail` check before `Create` is now redundant since the DB constraint + 23505 handler is the authoritative guard. The pre-check adds an extra round-trip and still has the TOCTOU race window (two requests can both pass the check concurrently before either inserts). The 23505 handler in the repo IS the correct fix; the pre-check should be removed.
- Fix: Delete lines 39–45 of `create_user.go` entirely. The 23505 catch in the repository is sufficient.

**M-3: `validate.NewInterceptor()` error not checked (future-safety)**
- File: `internal/modules/user/adapters/grpc/routes.go:18`
- Problem: If the library ever changes its constructor signature to return `(Interceptor, error)`, the error is silently dropped. Currently safe at v0.6.0 but fragile.
- Fix: No action required now, but add a comment pinning the assumption: `// validate.NewInterceptor() returns no error at v0.6.0`.

### Low

**L-1: `int32` overflow in List pagination**
- File: `internal/modules/user/adapters/postgres/repository.go:63`
- `int32(limit + 1)` — `limit` is capped at 100 in the handler, so max value is 101. No practical overflow risk. But the cast is implicit and unchecked. If the cap is ever raised past `math.MaxInt32`, this silently wraps. Low risk given current bounds.

**L-2: `encodeCursor` ignores marshal error**
- File: `internal/modules/user/adapters/postgres/repository.go:201–204`
```go
func encodeCursor(t time.Time, id uuid.UUID) string {
    data, _ := json.Marshal(cursorPayload{T: t, U: id})
    return base64.URLEncoding.EncodeToString(data)
}
```
`json.Marshal` on a struct with only `time.Time` and `uuid.UUID` fields cannot actually fail, so `_` is acceptable here. Worth a comment: `// Marshal of fixed types cannot fail`.

**L-3: Audit `changes` field contains full event JSON including ActorID**
- File: `internal/modules/audit/subscriber.go:52,78,104`
- `changes, _ := json.Marshal(event)` — The audit log `changes` column stores the full event struct, which includes `ActorID`. This means `ActorID` is stored twice (once in the dedicated `ActorID` column, once inside `changes`). Not a bug, just redundant data. Low priority.

---

## 4. Edge Case Analysis

**1. Pagination: limit=0, malformed cursor, exactly limit rows returned**

- `limit=0`: Handler clamps to 20 at `list_users.go:28`. Safe. Repository never sees 0.
- Malformed cursor: `decodeCursor` returns an error; the `if err == nil` guard at line 66 means the cursor fields are simply not set — the query runs from the beginning. Silent fail, no error returned to the caller. This is a defensible design for pagination (treat bad cursor as "start from beginning") but the caller gets no indication the cursor was invalid. If strict cursor validation is desired, return an error. Current behavior is acceptable for a boilerplate.
- Exactly `limit` rows: `len(rows) == limit`, so `hasMore = false`, `nextCursor = ""`. Correct — no more pages.
- Exactly `limit+1` rows: `hasMore = true`, truncated to `limit`. Correct.

**2. SoftDelete: already soft-deleted, ID doesn't exist**

Both cases return 0 affected rows from `WHERE deleted_at IS NULL`. Both correctly return `sharederr.ErrNotFound`. The caller cannot distinguish between "never existed" and "already deleted" — this is standard soft-delete semantics.

**3. 23505 catch: wrong column**

Addressed in M-1 above. The current implementation overmatch on any unique constraint. The fix is to add a constraint name check.

**4. Event publishing: bus.Publish fails**

Publish is fire-and-forget: errors are logged, not returned. This means a publish failure does not cause the HTTP/RPC response to fail. The operation (create/update/delete) is already committed to the DB at this point, so this is the correct behavior — event bus unavailability should not roll back a successful DB write. The audit trail will have a gap, which is logged. Acceptable for non-critical audit.

**5. ActorID: no auth context**

When `auth.UserFromContext(ctx)` returns `nil`, `actorID` remains `""` (empty string). The audit `parseActorID` falls back to `entityID` and emits a `slog.Warn`. This means system/cron actions appear to have self as actor in the audit log. Tracked in memory as I-5. The `slog.Warn` makes the gap visible in logs.

**6. Protovalidate: proto message with no validation annotations**

The `connectrpc.com/validate` interceptor runs CEL validation only if the proto message has `(buf.validate.field)` options. If a message has none, the interceptor is a no-op pass-through. No rejection, no panic. Safe.

---

## 5. DI Verification

`module.go` provides:
```go
fx.Provide(app.NewUpdateUserHandler),
fx.Provide(app.NewDeleteUserHandler),
```

`NewUpdateUserHandler` signature: `func(repo domain.UserRepository, bus *events.EventBus) *UpdateUserHandler`
`NewDeleteUserHandler` signature: `func(repo domain.UserRepository, bus *events.EventBus) *DeleteUserHandler`

`domain.UserRepository` is provided via `fx.Annotate(postgres.NewPgUserRepository, fx.As(new(domain.UserRepository)))`.
`*events.EventBus` is provided by `events.NewEventBus(publisher)` which is registered in the events module (confirmed from `bus.go`).

Fx will resolve both dependencies automatically. No extra wiring needed. DI is correct.

---

## 6. Recommendation

**Ship — after addressing M-1 and M-2.**

The two medium issues are both in `create_user.go` / `repository.go` and are small, contained fixes:

1. Remove the redundant `GetByEmail` pre-check in `create_user.go` (lines 39–45). The 23505 handler is the correct guard.
2. Add constraint name check to the 23505 handler: `&& pgErr.ConstraintName == "<actual_constraint_name>"`.

All other issues are low priority and do not block shipment. The 6 fixes are a net improvement over the previous state.

---

## Unresolved Questions

- What is the exact PG unique constraint name on `users.email`? Needed to make M-1 fix precise. Check migration file or run `\d users` in psql.
- Does the `users` table have a soft-delete-aware unique index on email (i.e., partial index `WHERE deleted_at IS NULL`)? If yes, the constraint name check in M-1 must target that index specifically.
- Is there an intent to support system/cron actions creating audit entries with a meaningful actor? If yes, I-5 needs a dedicated service-account ActorID rather than the entity-fallback.
