# Phase 1: Scaffold CLI

**Priority:** High | **Status:** completed | **Effort:** 2h

## Overview

Create `cmd/scaffold/main.go` — the CLI entry point that parses flags, derives naming variants, executes templates, writes files, and prints next steps.

## Context Links

- Plan: `plan.md`
- Reference: `internal/modules/user/` (complete CRUD module)
- Brainstorm: `plans/reports/brainstorm-260305-1557-module-scaffold-generator.md`

## Requirements

### Functional
- Accept `-name` flag (required, singular lowercase: "product", "order")
- Accept `-plural` flag (optional override, default: name + "s")
- Derive naming variants: singular, plural, title, plural-title, snake_case
- Embed all `.tmpl` files via `//go:embed`
- Execute each template with `ModuleData` struct
- Write 17 output files to correct locations
- Check file conflicts before writing (abort if exists)
- Print colored next-steps instructions after scaffold

### Non-Functional
- Zero external dependencies (stdlib only)
- Exit code 1 on any error
- Idempotent: re-running with same name fails safely (conflict check)

## Template Data Structure

```go
type ModuleData struct {
    Name            string // "product"
    NameTitle       string // "Product"
    NamePlural      string // "products"
    NamePluralTitle string // "Products"
    NameSnake       string // "product" (same for single word, "order_item" for multi)
    NameID          string // "ProductID"
    Timestamp       string // "20260305153000" (for migration)
    GoModule        string // "github.com/gnha/gnha-services"
}
```

## Related Code Files

### Create
- `cmd/scaffold/main.go` (~150 lines)

### Read (reference)
- `go.mod` — extract Go module path
- `internal/modules/user/` — all files as pattern reference

## Implementation Steps

1. Create `cmd/scaffold/` directory
2. Implement `main.go`:
   - Parse `-name` and `-plural` flags
   - Validate: name must be lowercase, alphabetic, no spaces
   - Read `go.mod` first line to extract module path (or hardcode)
   - Derive all naming variants using `strings` and `unicode`
   - Generate timestamp for migration: `time.Now().Format("20060102150405")`
   - Define output file map: template name → output path
   - For each template:
     a. Check if output file exists → abort with error
     b. Create parent directories (`os.MkdirAll`)
     c. Parse and execute template
     d. Write to file with `0644` permissions
   - Print success message with next steps

3. Output file mapping:
```
proto.tmpl           → proto/{name}/v1/{name}.proto
migration.tmpl       → db/migrations/{timestamp}_create_{plural}.sql
queries.tmpl         → db/queries/{name}.sql
domain_entity.tmpl   → internal/modules/{name}/domain/{name}.go
domain_repository.tmpl → internal/modules/{name}/domain/repository.go
domain_errors.tmpl   → internal/modules/{name}/domain/errors.go
domain_test.tmpl     → internal/modules/{name}/domain/{name}_test.go
app_create.tmpl      → internal/modules/{name}/app/create_{name}.go
app_create_test.tmpl → internal/modules/{name}/app/create_{name}_test.go
app_get.tmpl         → internal/modules/{name}/app/get_{name}.go
app_list.tmpl        → internal/modules/{name}/app/list_{plural}.go
app_update.tmpl      → internal/modules/{name}/app/update_{name}.go
app_delete.tmpl      → internal/modules/{name}/app/delete_{name}.go
adapter_postgres.tmpl → internal/modules/{name}/adapters/postgres/repository.go
adapter_postgres_test.tmpl → internal/modules/{name}/adapters/postgres/repository_test.go
adapter_grpc_handler.tmpl → internal/modules/{name}/adapters/grpc/handler.go
adapter_grpc_mapper.tmpl  → internal/modules/{name}/adapters/grpc/mapper.go
adapter_grpc_routes.tmpl  → internal/modules/{name}/adapters/grpc/routes.go
module.tmpl          → internal/modules/{name}/module.go
```

## Success Criteria

- [x] `go run ./cmd/scaffold -name=product` creates all 17 files
- [x] `go run ./cmd/scaffold -name=product` fails on second run (conflict)
- [x] `go run ./cmd/scaffold -name=order -plural=orders` works with custom plural
- [x] `go build ./cmd/scaffold` compiles with zero errors
- [x] Next-steps output is clear and actionable

## Todo

- [x] Create cmd/scaffold directory
- [x] Implement main.go with flag parsing and name derivation
- [x] Implement template execution and file writing
- [x] Implement conflict detection
- [x] Implement next-steps output

## Implementation Notes

- Safe UUID parsing helper added for H-1 code review feedback
- Plural flag validation added for M-1 feedback
- Go reserved word blocklist implemented
- Leading/trailing underscore validation added
- Dynamic go.mod module reading via sentinel check
