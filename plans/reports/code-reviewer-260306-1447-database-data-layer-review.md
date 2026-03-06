# Database & Data Layer Review

**Date:** 2026-03-06
**Scope:** Schema, SQLC, repositories, connection management, Redis, test utilities
**Files reviewed:** 18 files across `db/`, `gen/sqlc/`, `internal/modules/user/`, `internal/shared/database/`, `internal/shared/testutil/`

## Overall Assessment

The data layer is well-structured and follows hexagonal architecture correctly. SQLC generation with pgx/v5 is a strong choice. Cursor-based pagination, transactional updates with `FOR UPDATE`, and soft-delete patterns are all implemented properly. A few medium-priority gaps exist around schema constraints, missing indexes, and test coverage breadth.

**Score: 8/10**

---

## Critical Issues

None.

---

## High Priority

### H-1: `ListUsers` returns password hashes in SQL result set

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/db/queries/user.sql` (line 11)

All queries use `SELECT *`, which includes the `password` column. While the proto mapper strips it before reaching clients, the password hash travels through every layer of the call chain (sqlcgen -> repository -> domain -> app service -> handler). This violates defense-in-depth.

**Fix:** Create explicit column lists for read queries, omitting `password` where it is not needed (ListUsers, GetUserByID). Only include `password` in `GetUserByEmail` (login use case) and `GetUserByIDForUpdate`.

```sql
-- name: ListUsers :many
SELECT id, email, name, role, created_at, updated_at, deleted_at FROM users
WHERE deleted_at IS NULL
  AND (sqlc.narg('cursor_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_created_at'), sqlc.narg('cursor_id')::uuid))
ORDER BY created_at DESC, id DESC
LIMIT $1;
```

This requires a separate SQLC row type (which sqlc generates automatically when columns differ).

### H-2: Audit module creates its own `sqlcgen.Queries` instance

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/audit/module.go` (line 12-14)

```go
fx.Provide(func(pool *pgxpool.Pool) *sqlcgen.Queries {
    return sqlcgen.New(pool)
}),
```

This provides `*sqlcgen.Queries` into the Fx container. If any other module also provides `*sqlcgen.Queries`, Fx will fail at startup with a duplicate provider error. The user module avoids this by creating `sqlcgen.New(r.pool)` inline, but the asymmetry is fragile.

**Fix:** Either make audit's Queries a named/tagged type, or use inline instantiation like the user module does:

```go
fx.Provide(func(pool *pgxpool.Pool) *Handler {
    return NewHandler(sqlcgen.New(pool))
}),
```

### H-3: No `CHECK` constraint on `role` column

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/db/migrations/00001_initial_schema.sql` (line 9)

The `role` column is `VARCHAR(50)` with no database-level constraint. While the domain validates roles, any direct SQL insert or migration script could insert invalid values.

**Fix:**
```sql
role VARCHAR(50) NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member', 'viewer')),
```

---

## Medium Priority

### M-1: `audit_logs` table has no partition strategy and no retention policy

The `audit_logs` table will grow unbounded. With high-write workloads, the `idx_audit_created` index helps queries but does not address storage growth.

**Recommendation:** Add a comment documenting the intended retention strategy. For production, consider:
- Range partitioning by `created_at` (monthly)
- A scheduled job to drop old partitions

### M-2: Connection pool settings are hardcoded

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/database/postgres.go` (lines 21-24)

```go
poolCfg.MaxConns = 25
poolCfg.MinConns = 5
poolCfg.MaxConnLifetime = 1 * time.Hour
poolCfg.MaxConnIdleTime = 30 * time.Minute
```

These should come from config for environment-specific tuning. Same applies to Redis pool sizing in `redis.go` line 21.

### M-3: Retry loops use linear backoff with `time.Sleep`

**Files:** `postgres.go` line 37, `redis.go` line 31

Both use `time.Sleep(time.Duration(i+1) * time.Second)` in a non-cancellable loop. If the startup context is cancelled (e.g., Fx shutdown timeout), the sleep continues.

**Fix:** Accept a `context.Context` parameter and use `select` with context cancellation:
```go
select {
case <-ctx.Done():
    return nil, ctx.Err()
case <-time.After(time.Duration(i+1) * time.Second):
}
```

### M-4: `decodeCursor` silently ignores invalid cursors

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` (lines 65-69)

```go
if cursor != "" {
    decoded, err := decodeCursor(cursor)
    if err == nil {
        // use decoded
    }
}
```

A malformed cursor is silently treated as "no cursor" (returns page 1). This could mask client bugs or allow cursor tampering to reset pagination.

**Fix:** Return an error to the caller when cursor is non-empty but invalid:
```go
if cursor != "" {
    decoded, err := decodeCursor(cursor)
    if err != nil {
        return nil, "", false, sharederr.New(sharederr.CodeInvalidArgument, "invalid pagination cursor")
    }
    // use decoded
}
```

### M-5: `UserFixture.Password` stores plaintext, not a hash

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/testutil/fixtures.go`

The fixture stores `"password123"` but `domain.NewUser` expects `hashedPassword`. The test in `repository_test.go` passes `"hashed_pwd"` directly. The fixture is unused in repository tests, which means it may mislead future test authors.

**Recommendation:** Either pre-hash the fixture passwords or rename the field to `PlaintextPassword` with a `HashedPassword()` helper.

### M-6: Missing composite index for cursor-based pagination query

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/db/migrations/00001_initial_schema.sql`

The `ListUsers` query orders by `(created_at DESC, id DESC)` and filters `(created_at, id) < (cursor)`. The existing `idx_users_active` only covers `(id) WHERE deleted_at IS NULL`. There is no composite index on `(created_at DESC, id DESC)` with the `deleted_at IS NULL` partial condition.

**Fix:**
```sql
CREATE INDEX idx_users_list ON users (created_at DESC, id DESC) WHERE deleted_at IS NULL;
```

This directly supports the keyset pagination query and avoids a sequential scan on large tables.

### M-7: `UpdateUser` always sends both `name` and `role` even if unchanged

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` (lines 147-153)

The `Update` method always sets both `Name` and `Role` as valid `pgtype.Text`, even if the caller's `fn` only modified one field. Because `COALESCE` is used, this is functionally correct but generates unnecessary column writes and WAL entries.

**Impact:** Low in practice for a boilerplate, but worth noting for high-write scenarios.

---

## Low Priority

### L-1: `uuid-ossp` extension created but `gen_random_uuid()` used instead

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/db/migrations/00001_initial_schema.sql` (line 2)

`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"` is loaded, but the table uses `gen_random_uuid()` which is built into PostgreSQL 13+. The extension is unnecessary.

### L-2: `encodeCursor` silently discards marshal error

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` (line 207)

```go
data, _ := json.Marshal(cursorPayload{T: t, U: id})
```

The error is discarded. While `json.Marshal` on this struct will never fail in practice (time.Time and uuid.UUID both marshal deterministically), it is a minor hygiene issue.

### L-3: `repository.go` is exactly 222 lines

Per project guidelines, files should be under 200 lines. The cursor helpers (lines 200-221) could be extracted to a `cursor.go` file.

### L-4: No `updated_at` trigger

The `updated_at` column relies on the application always setting `NOW()` in UPDATE queries. A database trigger would provide defense-in-depth for any direct SQL operations.

---

## Positive Observations

1. **Hexagonal separation** - Clean boundary between `domain.UserRepository` (port) and `postgres.PgUserRepository` (adapter). Domain has zero database imports.
2. **Transactional update pattern** - `Update(ctx, id, fn)` with `FOR UPDATE` locking is the correct pattern for optimistic-concurrency-free updates.
3. **Keyset pagination** - Properly implemented with `(created_at, id)` composite cursor, `LIMIT+1` for `hasMore` detection, and base64-encoded cursor tokens. This avoids OFFSET performance degradation.
4. **Soft delete** - Consistent `deleted_at IS NULL` filtering in all queries and partial indexes.
5. **Testcontainers** - Real Postgres in tests (not mocks) with proper cleanup via `t.Cleanup`.
6. **SQLC code generation** - Type-safe queries with pgx/v5, proper type overrides for UUID and JSONB.
7. **Duplicate email handling** - Maps PG constraint violation `23505` to domain error `ErrEmailTaken`.
8. **Error wrapping** - `fmt.Errorf("context: %w", err)` throughout for debuggable error chains.

---

## Test Coverage Assessment

| Area | Covered | Missing |
|------|---------|---------|
| Create + read-back | Yes | - |
| Duplicate email | Yes | - |
| GetByID not found | Yes | - |
| Soft delete + verify | Yes | - |
| List pagination (2 pages) | Yes | - |
| Update (transactional) | No | No test for `Update` method |
| GetByEmail | No | No test for `GetByEmail` |
| Invalid cursor handling | No | No test for malformed cursor |
| Concurrent update (race) | No | Would validate `FOR UPDATE` |
| SoftDelete already-deleted | No | Double-delete should return `ErrNotFound` |

**Estimated test coverage for repository: ~60%** -- the happy paths are covered but update and several edge cases are not.

---

## Recommended Actions (priority order)

1. Add composite index `idx_users_list (created_at DESC, id DESC) WHERE deleted_at IS NULL`
2. Add `CHECK` constraint on `role` column
3. Return error on invalid cursor instead of silently resetting to page 1
4. Add integration tests for `Update`, `GetByEmail`, invalid cursor, and double-delete
5. Make connection pool settings configurable via `Config`
6. Make retry loops context-aware
7. Remove unused `uuid-ossp` extension
8. Extract cursor helpers to separate file to stay under 200 lines
9. Consider column-explicit `SELECT` to avoid passing password hashes through list operations

---

## Unresolved Questions

- Is there a planned retention policy for `audit_logs`? If the table will grow large, partitioning should be decided before production.
- Should the audit module share a `*sqlcgen.Queries` from a shared provider, or is per-module instantiation intentional?
- Will `GetUserByEmail` be used for login? If so, it correctly includes `password`, but there is no login endpoint yet to validate the flow end-to-end.
