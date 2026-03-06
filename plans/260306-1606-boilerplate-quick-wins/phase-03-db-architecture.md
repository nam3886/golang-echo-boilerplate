---
phase: 3
priority: high
status: completed
---

# Phase 3: Database + Architecture

## Items

### D1: Add pagination composite index
- File: New migration `db/migrations/00002_add_pagination_index.sql`
- SQL: `CREATE INDEX CONCURRENTLY idx_users_pagination ON users (created_at DESC, id DESC) WHERE deleted_at IS NULL;`

### D2: Add CHECK constraint on role
- File: Same migration `db/migrations/00002_add_pagination_index.sql`
- SQL: `ALTER TABLE users ADD CONSTRAINT chk_users_role CHECK (role IN ('admin', 'member'));`

### D3: Config-driven connection pool
- File: `internal/shared/database/postgres.go`, `internal/shared/config/config.go`
- Add config: `DBMaxConns`, `DBMinConns`, `DBMaxConnLifetime`
- Apply to pgxpool config

### A1: fx.Private for audit module
- File: `internal/modules/audit/module.go`
- Change: Wrap `sqlcgen.Queries` provider with `fx.Private`

### A2: Extract GetClientIP from middleware to shared util
- File: Create `internal/shared/httputil/client_ip.go`
- Move `GetClientIP` from middleware, update imports in `app/create_user.go`

### A3: Repository.List return result struct
- File: `internal/modules/user/domain/repository.go`
- Change: `List` returns `(ListResult, error)` with struct containing users, total, nextCursor, hasMore

## Success Criteria
- Migration applies cleanly
- `go build ./...` passes
- No fx provider collision risk
