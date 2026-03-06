---
phase: 02
status: complete
completed: 2026-03-05
priority: CRITICAL
effort: Small
depends_on: [phase-01]
---

# Phase 02: Fix CreateUser UUID Mismatch

## Overview

Domain generates UUID via `uuid.NewString()` but DB generates different UUID via `gen_random_uuid()`. Fix: pass domain UUID to INSERT.

## Current Flow (broken)

```
NewUser() → uuid.NewString() = "abc-123"
  ↓
repo.Create() → INSERT (email, name, password, role) — no ID param
  ↓
DB → gen_random_uuid() = "xyz-789" — different UUID
  ↓
API returns "abc-123" — WRONG
```

## Target Flow

```
NewUser() → uuid.NewString() = "abc-123"
  ↓
repo.Create() → INSERT (id, email, name, password, role) — domain ID passed
  ↓
DB → stores "abc-123" — same UUID
  ↓
API returns "abc-123" — CORRECT
```

## Implementation Steps

### Step 1: Update SQL query

**File:** `db/queries/user.sql`

Change CreateUser from:
```sql
-- name: CreateUser :one
INSERT INTO users (email, name, password, role)
VALUES ($1, $2, $3, $4)
RETURNING *;
```

To:
```sql
-- name: CreateUser :one
INSERT INTO users (id, email, name, password, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
```

### Step 2: Regenerate sqlc

```bash
sqlc generate
```

This will update `gen/sqlc/user.sql.go`:
- `CreateUserParams` will gain `ID uuid.UUID` field
- Param order shifts: $1=id, $2=email, $3=name, $4=password, $5=role

### Step 3: Update repository Create method

**File:** `internal/modules/user/adapters/postgres/repository.go`

Update `Create()` to pass domain UUID:
```go
func (r *PgUserRepository) Create(ctx context.Context, user *domain.User) error {
    q := sqlcgen.New(r.pool)
    _, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
        ID:       uuid.MustParse(string(user.ID())), // pass domain UUID
        Email:    user.Email(),
        Name:     user.Name(),
        Password: user.Password(),
        Role:     string(user.Role()),
    })
    // ... error handling unchanged ...
}
```

### Step 4: Verify

```bash
go build ./...
go vet ./...
```

## Related Files

| File | Change |
|------|--------|
| `db/queries/user.sql` | Add `id` to INSERT params |
| `gen/sqlc/user.sql.go` | Regenerated — CreateUserParams gains ID field |
| `internal/modules/user/adapters/postgres/repository.go` | Pass `user.ID()` to CreateUserParams |

## Todo

- [x] Update SQL query (db/queries/user.sql - added id to INSERT)
- [x] sqlc generate (completed - CreateUserParams now includes ID field)
- [x] Update repository Create method (repository.go - passing uuid.MustParse(string(user.ID())))
- [x] go build passes (verified)
- [x] go vet passes (verified)

## Risk

- **LOW**: DB schema still has `DEFAULT gen_random_uuid()` — this is fine, default only applies when ID not provided.
