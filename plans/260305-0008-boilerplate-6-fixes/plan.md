---
status: complete
created: 2026-03-05
slug: boilerplate-6-fixes
context: plans/reports/brainstorm-260305-0008-boilerplate-fix-scope.md
---

# Plan: Fix 6 Boilerplate Issues

## Scope
3 correctness bugs + 3 pattern consistency fixes from full codebase review.

## Phases

| # | Phase | Priority | Effort | Status |
|---|-------|----------|--------|--------|
| 1 | Fix pagination cursor skip | CRITICAL | ~20 min | complete |
| 2 | Fix SoftDelete no-op success | CRITICAL | ~15 min | complete |
| 3 | Map Postgres unique violation (TOCTOU) | CRITICAL | ~15 min | complete |
| 4 | Publish events in Update/Delete handlers | IMPORTANT | ~20 min | complete |
| 5 | Fix ActorID in audit + events | IMPORTANT | ~20 min | complete |
| 6 | Wire protovalidate interceptor | IMPORTANT | ~15 min | complete |

## Phase Details

### Phase 1: Fix Pagination Cursor Skip
**Bug:** `repo.List()` returns `limit+1` rows + cursor from last row. App truncates to `limit` but uses repo's cursor (pointing to row limit+1). Next page skips 1 record.

**Fix:** Build cursor in app layer from last *kept* row, not in repo.

**Files:**
- `internal/modules/user/adapters/postgres/repository.go` — `List()` returns rows only, no cursor
- `internal/modules/user/app/list_users.go` — Build cursor from `users[limit-1]`
- `internal/modules/user/domain/user.go` — Confirm `ID()` and `CreatedAt()` are exported

**Implementation:**
1. `repository.go:List()` — Remove cursor building (lines 80-86), return `[]*domain.User, error` only
2. `domain/user.go` — Add `UserRepository.List` signature change if needed
3. `list_users.go` — Build nextCursor from last kept user: `encodeCursor(last.CreatedAt(), last.ID())`
4. Problem: `encodeCursor` is in `postgres` package. Options:
   - **Option A (simple):** Keep cursor in repo but fix the logic — build from `rows[limit-1]` not `rows[len-1]`
   - **Option B (clean):** Move cursor encoding to shared/pagination package

**Decision: Option A** (KISS — minimal change, fix the index)
```go
// repository.go — fix cursor to use rows[limit-1] when hasMore
if len(users) > 0 {
    last := users[len(users)-1] // This is already correct IF caller doesn't truncate
}
```

Wait — re-reading: repo returns ALL `limit+1` rows + cursor from the last (limit+1th). App truncates to `limit`. Cursor points to limit+1th row. Next call skips from limit+1th, missing nothing... Actually let me verify:

- `List(ctx, limit+1, cursor)` → repo fetches `limit+1` rows
- Repo builds cursor from `users[len-1]` = row at position `limit+1`
- App: `users = users[:limit]` (keeps 1..limit), `nextCursor` from repo = position of row `limit+1`
- Next page: `WHERE (created_at, id) < (cursor)` → starts AFTER row `limit+1`, skipping row `limit+1`
- Row `limit+1` is the probe row — it SHOULD be the first of the next page, not skipped

**Real fix:** Cursor should point to the LAST KEPT row (position `limit`), not the probe row. Then next page starts after row `limit`, getting row `limit+1` first.

```go
// In repository.go List(), change cursor to use the limit-th row:
// After app truncates, cursor from last kept row works.
// Simplest: let app control cursor building.
```

**Simplest fix:** In `list_users.go`, override nextCursor after truncation:
```go
if hasMore {
    users = users[:limit]
    last := users[len(users)-1]
    nextCursor = buildCursor(last) // Need cursor encoding in app or expose from repo
}
```

**Cleanest approach:** Export `EncodeCursor`/`DecodeCursor` from repo (or move to shared). Or simpler: repo already builds cursor from `users[len(users)-1]`. If app passes `limit+1`, repo returns `limit+1` rows, cursor from row `limit+1`. After app truncates, cursor is wrong.

**Final decision:** Fix in repo — don't build cursor from all returned rows. Instead, app passes actual `limit` (not `limit+1`), and repo handles the +1 probe internally.

Actually simplest: **Fix repo to accept `limit` and internally probe `limit+1`**, returning cursor from row at position `limit` (last kept row):

```go
func (r *PgUserRepository) List(ctx context.Context, limit int, cursor string) ([]*domain.User, string, bool, error) {
    // Fetch limit+1 internally
    params := sqlcgen.ListUsersParams{Limit: int32(limit + 1)}
    // ...
    hasMore := len(users) > limit
    if hasMore {
        users = users[:limit]
    }
    var nextCursor string
    if hasMore && len(users) > 0 {
        last := users[len(users)-1] // last KEPT row
        nextCursor = encodeCursor(...)
    }
    return users, nextCursor, hasMore, nil
}
```

Then `list_users.go` becomes trivial:
```go
users, nextCursor, hasMore, err := h.repo.List(ctx, limit, cursor)
```

This moves pagination logic to repo where cursor encoding already lives. Clean.

---

### Phase 2: Fix SoftDelete No-Op Success
**Bug:** `:exec` doesn't return affected rows. Deleting non-existent user returns nil.

**Fix:** Change SQL to `:execrows` and check count.

**Files:**
- `db/queries/user.sql` — Change `SoftDeleteUser` from `:exec` to `:execrows`
- `gen/sqlc/` — Regenerate
- `internal/modules/user/adapters/postgres/repository.go` — Check returned count

```sql
-- name: SoftDeleteUser :execrows
UPDATE users SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL;
```

```go
func (r *PgUserRepository) SoftDelete(ctx context.Context, id domain.UserID) error {
    uid, err := parseUserID(id)
    if err != nil { return err }
    q := sqlcgen.New(r.pool)
    rows, err := q.SoftDeleteUser(ctx, uid)
    if err != nil { return fmt.Errorf("soft deleting user: %w", err) }
    if rows == 0 { return sharederr.ErrNotFound }
    return nil
}
```

---

### Phase 3: Map Postgres Unique Violation (TOCTOU)
**Bug:** `Create()` does app-level uniqueness check (GetByEmail → Create) which races under concurrency. If two requests pass the check simultaneously, one gets a Postgres unique violation error that surfaces as internal error instead of `ErrEmailTaken`.

**Fix:** Catch Postgres error code `23505` (unique_violation) in `Create()`.

**Files:**
- `internal/modules/user/adapters/postgres/repository.go` — Add pgconn error handling in `Create()`

```go
import "github.com/jackc/pgx/v5/pgconn"

func (r *PgUserRepository) Create(ctx context.Context, user *domain.User) error {
    q := sqlcgen.New(r.pool)
    _, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{...})
    if err != nil {
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) && pgErr.Code == "23505" {
            return domain.ErrEmailTaken
        }
        return fmt.Errorf("inserting user: %w", err)
    }
    return nil
}
```

Note: Keep the app-level check in `create_user.go` as a fast path — it avoids unnecessary password hashing. The repo-level catch is the safety net.

---

### Phase 4: Publish Events in Update/Delete
**Bug:** `create_user.go` publishes events but `update_user.go` and `delete_user.go` don't.

**Fix:** Add EventBus to both handlers, publish after success.

**Files:**
- `internal/modules/user/app/update_user.go` — Add bus, publish UserUpdatedEvent
- `internal/modules/user/app/delete_user.go` — Add bus, publish UserDeletedEvent
- `internal/modules/user/module.go` — No change needed (Fx auto-injects)

**Events already defined:** `UserUpdatedEvent{UserID, At}`, `UserDeletedEvent{UserID, At}`

Add `ActorID` field to events (ties into Phase 5):
```go
type UserUpdatedEvent struct {
    UserID  string    `json:"user_id"`
    ActorID string    `json:"actor_id"`
    At      time.Time `json:"at"`
}
type UserDeletedEvent struct {
    UserID  string    `json:"user_id"`
    ActorID string    `json:"actor_id"`
    At      time.Time `json:"at"`
}
type UserCreatedEvent struct {
    UserID  string    `json:"user_id"`
    ActorID string    `json:"actor_id"` // add
    Email   string    `json:"email"`
    Name    string    `json:"name"`
    Role    string    `json:"role"`
    At      time.Time `json:"at"`
}
```

---

### Phase 5: Fix ActorID in Audit + Events
**Bug:** Audit subscriber sets `ActorID = entityID` (the user being acted upon). Should be the authenticated admin performing the action.

**Fix:**
1. Add `ActorID` field to all event structs (Phase 4)
2. Extract actor from `auth.UserFromContext(ctx)` in app handlers
3. Audit subscriber uses `event.ActorID` instead of `entityID`

**Files:**
- `internal/shared/events/topics.go` — Add ActorID to all event structs
- `internal/modules/user/app/create_user.go` — Extract actor, set in event
- `internal/modules/user/app/update_user.go` — Same
- `internal/modules/user/app/delete_user.go` — Same
- `internal/modules/audit/subscriber.go` — Parse ActorID from event, use for audit log

```go
// In any handler:
actor := auth.UserFromContext(ctx)
actorID := ""
if actor != nil {
    actorID = actor.UserID
}
// Pass actorID in event
```

```go
// In audit subscriber:
actorID := entityID // fallback
if event.ActorID != "" {
    if parsed, err := uuid.Parse(event.ActorID); err == nil {
        actorID = parsed
    }
}
```

---

### Phase 6: Wire Protovalidate Interceptor
**Bug:** Proto files have `buf.validate` annotations but no runtime enforcement. Any invalid data passes through.

**Fix:** Add `bufbuild/protovalidate-go` interceptor to Connect RPC handler.

**Files:**
- `go.mod` — Add `github.com/bufbuild/protovalidate-go`
- `internal/modules/user/adapters/grpc/routes.go` — Add interceptor

```go
import (
    "github.com/bufbuild/protovalidate-go/interceptor"
)

func RegisterRoutes(e *echo.Echo, handler *UserServiceHandler, cfg *config.Config, rdb *redis.Client) {
    path, h := userv1connect.NewUserServiceHandler(handler,
        connect.WithInterceptors(interceptor.NewUnaryInterceptor()),
    )
    g := e.Group(path, appmw.Auth(cfg, rdb))
    g.Any("*", echo.WrapHandler(http.StripPrefix(path, h)))
}
```

Need to verify exact import path for protovalidate-go v2 interceptor.

## Dependencies
- Phase 4 depends on Phase 5 (add ActorID to events first)
- Phase 5 depends on nothing
- All others independent

## Execution Order
5 → 4 → 1 → 2 → 3 → 6 → build+vet → review

## Risk
- Phase 1 changes `List()` signature → need to update `domain.UserRepository` interface
- Phase 6 depends on protovalidate-go library compatibility with Go 1.26

## Completion Summary

All 6 phases completed successfully. Full test suite passed. Code review approved.

**Build & Vet:** All phases passed `go build` and `go vet` checks.

**Code Review:** All phases passed code review. Post-review fixes applied:

- **M-2 (Phase 4):** Added `slog.Error` logging to Updated/Deleted unmarshal errors in audit subscriber
- **M-3 (Phase 4):** Added `changes` payload to `HandleUserDeleted` audit log for completeness
- **M-4 (Phase 5):** Added `slog.Warn` logging for invalid `actor_id` in `parseActorID` function
- **H-1 (Phase 6):** Ran `go mod tidy` to fix direct/indirect dependency markers

## Next Steps
Ready for merge to main branch.
