# Phase 2: Go Templates

**Priority:** High | **Status:** completed | **Effort:** 4h

## Overview

Create 19 `.tmpl` files in `cmd/scaffold/templates/` that generate a complete CRUD module matching the user module patterns exactly.

## Context Links

- Plan: `plan.md`
- Phase 1: `phase-01-scaffold-cli.md` (CLI that executes these templates)
- Reference module: `internal/modules/user/` (exact patterns to replicate)

## Requirements

### Functional
- Each template produces compilable Go code (or valid proto/SQL)
- Templates use `{{.Name}}`, `{{.NameTitle}}`, `{{.NamePlural}}`, etc. from `ModuleData`
- Generated code follows exact same patterns as user module
- Import paths use `{{.GoModule}}` for portability
- Proto validation rules use generic placeholders (dev customizes after)
- SQL schema uses generic columns: id, name, created_at, updated_at, deleted_at

### Non-Functional
- Templates must be readable — use Go template syntax, not string concatenation
- Each template is self-contained (no template inheritance)

## Template Variable Reference

```
{{.Name}}            → "product"
{{.NameTitle}}       → "Product"
{{.NamePlural}}      → "products"
{{.NamePluralTitle}} → "Products"
{{.NameID}}          → "ProductID"
{{.Timestamp}}       → "20260305153000"
{{.GoModule}}        → "github.com/gnha/gnha-services"
```

## Related Code Files

### Create (19 templates)
- `cmd/scaffold/templates/proto.tmpl`
- `cmd/scaffold/templates/migration.tmpl`
- `cmd/scaffold/templates/queries.tmpl`
- `cmd/scaffold/templates/domain_entity.tmpl`
- `cmd/scaffold/templates/domain_repository.tmpl`
- `cmd/scaffold/templates/domain_errors.tmpl`
- `cmd/scaffold/templates/domain_test.tmpl`
- `cmd/scaffold/templates/app_create.tmpl`
- `cmd/scaffold/templates/app_create_test.tmpl`
- `cmd/scaffold/templates/app_get.tmpl`
- `cmd/scaffold/templates/app_list.tmpl`
- `cmd/scaffold/templates/app_update.tmpl`
- `cmd/scaffold/templates/app_delete.tmpl`
- `cmd/scaffold/templates/adapter_postgres.tmpl`
- `cmd/scaffold/templates/adapter_postgres_test.tmpl`
- `cmd/scaffold/templates/adapter_grpc_handler.tmpl`
- `cmd/scaffold/templates/adapter_grpc_mapper.tmpl`
- `cmd/scaffold/templates/adapter_grpc_routes.tmpl`
- `cmd/scaffold/templates/module.tmpl`

### Read (reference patterns — copy structure exactly)
- `proto/user/v1/user.proto`
- `db/migrations/00001_initial_schema.sql`
- `db/queries/user.sql`
- `internal/modules/user/domain/user.go`
- `internal/modules/user/domain/repository.go`
- `internal/modules/user/domain/errors.go`
- `internal/modules/user/domain/user_test.go`
- `internal/modules/user/app/create_user.go`
- `internal/modules/user/app/create_user_test.go`
- `internal/modules/user/app/get_user.go`
- `internal/modules/user/app/list_users.go`
- `internal/modules/user/app/update_user.go`
- `internal/modules/user/app/delete_user.go`
- `internal/modules/user/adapters/postgres/repository.go`
- `internal/modules/user/adapters/postgres/repository_test.go`
- `internal/modules/user/adapters/grpc/handler.go`
- `internal/modules/user/adapters/grpc/mapper.go`
- `internal/modules/user/adapters/grpc/routes.go`
- `internal/modules/user/module.go`

## Implementation Steps

### Group A: Proto + Database (3 templates)

1. **proto.tmpl** — Generic CRUD service definition
   - Service: `{{.NameTitle}}Service` with 5 RPCs
   - Message: `{{.NameTitle}}` with id, name, created_at, updated_at
   - CreateRequest: name field with `buf.validate` (min_len: 1, max_len: 255)
   - GetRequest/DeleteRequest: id with uuid validation
   - ListRequest: limit (1-100) + cursor
   - UpdateRequest: id + optional name
   - Package: `{{.Name}}.v1`
   - go_package: `{{.GoModule}}/gen/proto/{{.Name}}/v1;{{.Name}}v1`

2. **migration.tmpl** — Table creation with soft delete
   - Table: `{{.NamePlural}}` with id (UUID PK), name (VARCHAR 255), timestamps, deleted_at
   - Indexes: name partial index (WHERE deleted_at IS NULL), active index
   - goose Up/Down format

3. **queries.tmpl** — sqlc CRUD queries
   - `Get{{.NameTitle}}ByID :one` — WHERE id = $1 AND deleted_at IS NULL
   - `Get{{.NameTitle}}ByIDForUpdate :one` — same + FOR UPDATE
   - `List{{.NamePluralTitle}} :many` — Keyset pagination with cursor
   - `Create{{.NameTitle}} :one` — INSERT RETURNING *
   - `Update{{.NameTitle}} :one` — COALESCE update pattern
   - `SoftDelete{{.NameTitle}} :execrows` — SET deleted_at = NOW()

### Group B: Domain Layer (4 templates)

4. **domain_entity.tmpl** — Entity with typed ID, private fields, getters
   - Type `{{.NameID}} string`
   - Struct with: id, name, createdAt, updatedAt, deletedAt
   - `New{{.NameTitle}}(name string) (*{{.NameTitle}}, error)` — validates + UUID
   - `Reconstitute(...)` — rebuilds from DB
   - `ChangeName(name string) error` — domain method
   - Getters for all fields

5. **domain_repository.tmpl** — Interface + mockgen directive
   - `//go:generate mockgen` directive with correct destination path
   - `{{.NameTitle}}Repository` interface: GetByID, List, Create, Update, SoftDelete
   - Closure-based Update: `func(*{{.NameTitle}}) error`

6. **domain_errors.tmpl** — Module-specific errors
   - `Err{{.NameTitle}}NotFound` (CodeNotFound)
   - `ErrNameRequired` (CodeInvalidArgument)
   - `ErrInvalidName` (CodeInvalidArgument)

7. **domain_test.tmpl** — Entity constructor + validation tests
   - `TestNew{{.NameTitle}}_Success`
   - `TestNew{{.NameTitle}}_EmptyName`
   - Table-driven pattern for validation edge cases

### Group C: Application Layer (6 templates)

8. **app_create.tmpl** — Create command handler
   - `Create{{.NameTitle}}Cmd` struct
   - `Create{{.NameTitle}}Handler` with repo + bus dependencies
   - Handle: validate → create entity → persist → publish event

9. **app_create_test.tmpl** — Handler test with gomock
   - gomock Controller + MockRepository
   - noopPublisher stub for EventBus
   - Test success case + duplicate error case

10. **app_get.tmpl** — Get query handler
    - `Get{{.NameTitle}}Handler` with repo dependency
    - Simple: parse ID → repo.GetByID

11. **app_list.tmpl** — List with pagination
    - `List{{.NamePluralTitle}}Result` struct (Items, NextCursor, HasMore)
    - `List{{.NamePluralTitle}}Handler`
    - Limit bounds enforcement (default 20, max 100)

12. **app_update.tmpl** — Update with closure UoW
    - `Update{{.NameTitle}}Cmd` with optional fields (*string)
    - Closure-based repo.Update pattern
    - Publish event after transaction

13. **app_delete.tmpl** — Soft delete handler
    - `Delete{{.NameTitle}}Handler` with repo + bus
    - repo.SoftDelete → publish event

### Group D: Adapter Layer (5 templates)

14. **adapter_postgres.tmpl** — Repository implementation
    - `Pg{{.NameTitle}}Repository` with pgxpool.Pool
    - All 6 methods implementing domain interface
    - `toDomain()` helper for sqlc row → domain entity
    - `parse{{.NameTitle}}ID()` for UUID validation
    - Cursor encoding/decoding (base64 JSON)
    - PG error mapping (23505 unique violation)
    - Transaction management in Update

15. **adapter_postgres_test.tmpl** — Integration test scaffold
    - `//go:build integration` tag
    - `setupRepo(t)` with testutil.NewTestPostgres
    - Tests: Create, Create_Duplicate, GetByID, List_Pagination, SoftDelete

16. **adapter_grpc_handler.tmpl** — Connect RPC handler
    - `{{.NameTitle}}ServiceHandler` implementing generated interface
    - `var _ {{.Name}}v1connect.{{.NameTitle}}ServiceHandler = (*{{.NameTitle}}ServiceHandler)(nil)`
    - All 5 RPC methods mapping proto → app commands → proto responses

17. **adapter_grpc_mapper.tmpl** — Domain ↔ proto conversion
    - `toProto(*domain.{{.NameTitle}}) *{{.Name}}v1.{{.NameTitle}}`
    - `domainErrorToConnect(error) error` with code mapping

18. **adapter_grpc_routes.tmpl** — Echo route registration
    - `RegisterRoutes(e *echo.Echo, handler, cfg, rdb)`
    - Connect handler mount with validation interceptor + auth middleware

### Group E: Module Registration (1 template)

19. **module.tmpl** — fx.Module definition
    - Provide repository (annotated as interface)
    - Provide all 5 handlers
    - Provide gRPC service handler
    - Invoke RegisterRoutes

## Template Conventions

- Use `{{` and `}}` delimiters (Go default)
- First line of every .go template: `package {{.Name}}` (or appropriate subpackage)
- All imports use full paths with `{{.GoModule}}` prefix
- No trailing whitespace in templates
- Use `{{- -}}` trim markers where needed to avoid blank lines

## Success Criteria

- [x] All 19 templates created in cmd/scaffold/templates/
- [x] `task module:create name=product` generates compilable code
- [x] Generated proto passes `buf lint`
- [x] Generated SQL is valid for sqlc
- [x] Generated Go compiles: `go build ./internal/modules/product/...`
- [x] Generated tests compile: `go test -run=^$ ./internal/modules/product/...`
- [x] Patterns match user module exactly (verified by diff)

## Todo

- [x] Create templates directory
- [x] Write Group A templates (proto, migration, queries)
- [x] Write Group B templates (domain layer)
- [x] Write Group C templates (application layer)
- [x] Write Group D templates (adapter layer)
- [x] Write Group E template (module registration)
- [x] Verify all templates produce valid output

## Implementation Notes

- All 19 templates created with proper formatting and variable references
- Composite cursor pagination index added (M-3 feedback)
- All templates compile and generate valid code
- User module patterns replicated exactly
