---
phase: 03
status: complete
completed: 2026-03-05
priority: HIGH
effort: Medium
depends_on: []
---

# Phase 03: Rewrite Docs + Example Tests + Mockgen

## Overview

Three related items that make the boilerplate "followable" for new devs:
- A) Rewrite adding-a-module.md to match actual code patterns
- B) Add example test files (domain unit + repo integration)
- C) Setup mockgen with go:generate directive
- D) Standardize naming convention in docs

## Part A: Rewrite adding-a-module.md

**File:** `docs/adding-a-module.md` (268 lines → rewrite)

### Current Problems

1. **Entity pattern wrong** — doc shows exported fields:
   ```go
   type Product struct { ID uuid.UUID; Name string }
   ```
   Actual code uses unexported + getters + Reconstitute:
   ```go
   type User struct { id UserID; email string }
   func (u *User) ID() UserID { return u.id }
   func Reconstitute(...) *User { ... }
   ```

2. **Error API wrong** — doc shows `errors.NewNotFound("...")` which doesn't exist.
   Actual: `errors.New(errors.CodeNotFound, "...")`

3. **Repository error handling wrong** — doc shows bare `return nil, domain.ErrProductNotFound` without `pgx.ErrNoRows` check or `fmt.Errorf` wrapping.

4. **Naming inconsistent** — doc shows `type Repository struct` / `NewRepository()`.
   Actual: `type PgUserRepository struct` / `NewPgUserRepository()`

### Fix

Rewrite all code examples to match the actual user module patterns:
- Unexported fields + getters + Reconstitute
- `sharederr.New(sharederr.CodeNotFound, "product not found")`
- `pgx.ErrNoRows` check → domain error, else `fmt.Errorf("...: %w", err)`
- `PgProductRepository` / `NewPgProductRepository()`
- Add section on event publishing + subscriber registration

### Reference Files (copy patterns from)

| Pattern | Source file |
|---------|------------|
| Entity | `internal/modules/user/domain/user.go` |
| Errors | `internal/modules/user/domain/errors.go` |
| Repository interface | `internal/modules/user/domain/repository.go` |
| Repository impl | `internal/modules/user/adapters/postgres/repository.go` |
| App handler | `internal/modules/user/app/create_user.go` |
| gRPC handler | `internal/modules/user/adapters/grpc/handler.go` |
| Routes | `internal/modules/user/adapters/grpc/routes.go` |
| Module | `internal/modules/user/module.go` |
| Events | `internal/shared/events/topics.go` |

## Part B: Example Test Files

### B1: Domain Unit Test

**Create:** `internal/modules/user/domain/user_test.go`

Test cases:
- `TestNewUser_Success` — valid inputs → user created with correct fields
- `TestNewUser_InvalidEmail` — empty email → error
- `TestNewUser_InvalidRole` — unknown role → error
- `TestUser_ChangeName` — valid name → updated
- `TestUser_ChangeName_Empty` — empty → error
- `TestUser_ChangeRole` — valid role → updated

Pattern: table-driven tests, no mocks needed (pure domain logic).

### B2: Repository Integration Test

**Create:** `internal/modules/user/adapters/postgres/repository_test.go`

Test cases:
- `TestPgUserRepository_Create` — insert + verify by GetByID
- `TestPgUserRepository_Create_DuplicateEmail` — returns ErrEmailTaken
- `TestPgUserRepository_GetByID_NotFound` — returns ErrNotFound
- `TestPgUserRepository_SoftDelete` — delete + verify GetByID returns ErrNotFound
- `TestPgUserRepository_List_Pagination` — insert 3 users, list with limit=2, verify cursor

Uses: `testutil.NewTestPostgres(t)` for real DB via testcontainers.

**Prerequisite:** Need to run migrations on test DB. Add helper:

**Create:** `internal/shared/testutil/migrate.go`
```go
func RunMigrations(t *testing.T, pool *pgxpool.Pool) {
    // Read and execute migration files from db/migrations/
    // Or use goose programmatic API
}
```

### B3: App Handler Unit Test (with mockgen)

**Create:** `internal/modules/user/app/create_user_test.go`

Test cases:
- `TestCreateUserHandler_Success` — mock repo + mock bus → user created
- `TestCreateUserHandler_EmailTaken` — mock repo returns ErrEmailTaken → error

Uses: mockgen-generated mock for `domain.UserRepository`.

## Part C: Setup Mockgen

### C1: Install mockgen

Add to `go.mod`:
```bash
go install go.uber.org/mock/mockgen@latest
```

### C2: Add go:generate directive

**File:** `internal/modules/user/domain/repository.go`

Add before interface:
```go
//go:generate mockgen -source=repository.go -destination=../../../shared/mocks/mock_user_repository.go -package=mocks
```

### C3: Generate mocks

```bash
go generate ./internal/modules/user/domain/...
```

Creates: `internal/shared/mocks/mock_user_repository.go`

### C4: Add to Taskfile

Add mock generation to `task generate`:
```yaml
generate:
  cmds:
    - buf generate
    - sqlc generate
    - go generate ./...
```

## Part D: Naming Convention

Already consistent in code (`PgXxxRepository` / `NewPgXxxRepository`).
Fix only in docs (Part A covers this).

## Todo

- [x] Rewrite adding-a-module.md matching actual patterns (completely rewritten)
- [x] Create domain/user_test.go (9 unit tests - NewUser, ChangeName, ChangeRole, validation cases)
- [x] Create testutil/migrate.go helper (migration execution helper)
- [x] Create adapters/postgres/repository_test.go (5 integration tests - CRUD + pagination)
- [x] Install mockgen + add go:generate directive (installed, directive added to repository.go)
- [x] Generate mocks (mocks generated in internal/shared/mocks/)
- [x] Create app/create_user_test.go (2 unit tests - success + email taken)
- [x] Update Taskfile generate task (added generate:mocks task + updated generate task)
- [x] go test ./... passes (verified)
- [x] go build ./... passes (verified)

## Risk

- **MEDIUM**: testcontainers requires Docker running. CI already has Docker service. Local dev needs Docker Desktop.
- **LOW**: Migration runner in tests — may need goose programmatic API or raw SQL execution.
