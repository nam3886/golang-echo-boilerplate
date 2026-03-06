# Brainstorm: Module Scaffold Generator

Date: 2026-03-05

## Problem

Tạo module mới yêu cầu manual scaffold **16+ Go files** + proto + migration + SQL queries. Copy-paste từ user module → error-prone, tốn 30-45 phút, inconsistent naming.

## Decisions (Agreed)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Scope | Full CRUD mặc định | User module = reference pattern, scaffold giống hệt |
| Approach | Go `text/template` trong `cmd/scaffold/` | Zero external deps, type-safe, consistent với codebase |
| Tests | Có scaffold tests | domain_test.go, handler_test.go, repository_test.go với gomock |
| Registration | Manual — print hướng dẫn | An toàn, không risk sửa nhầm main.go |
| Migration | Timestamp-based | Không conflict khi nhiều dev làm song song |

## Solution Design

### Command

```bash
task module:create name=product
```

### Generated Files (17 files)

```
proto/product/v1/product.proto          # Service + messages + validation
db/migrations/{timestamp}_create_products.sql  # Table + indexes
db/queries/product.sql                  # CRUD queries (sqlc)

internal/modules/product/
├── domain/
│   ├── product.go                      # Entity + constructors + getters
│   ├── repository.go                   # Interface + mockgen directive
│   ├── errors.go                       # Module-specific errors
│   └── product_test.go                 # Entity validation tests
├── app/
│   ├── create_product.go              # Command handler + event publish
│   ├── create_product_test.go         # Handler test with gomock
│   ├── get_product.go                 # Query handler
│   ├── list_products.go              # Pagination handler
│   ├── update_product.go             # Update handler
│   └── delete_product.go             # Soft delete handler
├── adapters/
│   ├── postgres/
│   │   ├── repository.go             # sqlc-backed implementation
│   │   └── repository_test.go        # Integration test scaffold
│   └── grpc/
│       ├── handler.go                # Connect RPC handler
│       ├── mapper.go                 # Domain ↔ proto conversion
│       └── routes.go                 # Route registration
└── module.go                          # fx.Module definition
```

### Template Variables

```go
type ModuleData struct {
    Name          string // "product"
    NameTitle     string // "Product"
    NamePlural    string // "products"
    NamePluralTitle string // "Products"
    Timestamp     string // "20260305153000"
    GoPackage     string // "github.com/gnha/gnha-services"
}
```

### Implementation Approach

**Location:** `cmd/scaffold/main.go` (~200 lines)

1. Parse `name` flag
2. Derive naming variants (singular, plural, title case)
3. Execute templates → write files
4. Run `task generate` (proto + sqlc + mocks)
5. Print next steps (add to main.go, customize fields)

**Templates:** Embedded via `//go:embed templates/*.tmpl` in `cmd/scaffold/`

```
cmd/scaffold/
├── main.go                    # CLI entry point
└── templates/
    ├── proto.tmpl
    ├── migration.tmpl
    ├── queries.tmpl
    ├── domain_entity.tmpl
    ├── domain_repository.tmpl
    ├── domain_errors.tmpl
    ├── domain_test.tmpl
    ├── app_create.tmpl
    ├── app_create_test.tmpl
    ├── app_get.tmpl
    ├── app_list.tmpl
    ├── app_update.tmpl
    ├── app_delete.tmpl
    ├── adapter_postgres.tmpl
    ├── adapter_postgres_test.tmpl
    ├── adapter_grpc_handler.tmpl
    ├── adapter_grpc_mapper.tmpl
    ├── adapter_grpc_routes.tmpl
    └── module.tmpl
```

### Post-Scaffold Output

```
✓ Created proto/product/v1/product.proto
✓ Created db/migrations/20260305153000_create_products.sql
✓ Created db/queries/product.sql
✓ Created internal/modules/product/ (17 files)
✓ Generated code (proto + sqlc + mocks)

Next steps:
  1. Customize proto/product/v1/product.proto fields
  2. Customize db/migrations/20260305153000_create_products.sql columns
  3. Customize db/queries/product.sql queries
  4. Run: task generate
  5. Add to cmd/server/main.go:
     import "github.com/gnha/gnha-services/internal/modules/product"
     // In fx.New(): product.Module,
  6. Run: task migrate:up
  7. Run: task check
```

### Taskfile Integration

```yaml
module:create:
  desc: Scaffold a new module
  cmds:
    - go run ./cmd/scaffold -name={{.name}}
    - task: generate
  requires:
    vars: [name]
```

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Template drift vs actual patterns | CI test: scaffold + compile |
| Plural naming edge cases (e.g. "category" → "categories") | Simple `s` suffix by default, allow `-plural` flag override |
| Generated code conflicts | Check file exists before writing, abort if conflict |

## Effort Estimate

| Component | LOC | Effort |
|-----------|-----|--------|
| `cmd/scaffold/main.go` | ~150 | 2h |
| 19 `.tmpl` templates | ~800 total | 4h |
| Taskfile integration | ~5 | 15min |
| Docs update (`adding-a-module.md`) | ~20 | 30min |
| CI scaffold-compile test | ~20 | 30min |
| **Total** | **~1000** | **~1 day** |

## Success Criteria

- [ ] `task module:create name=product` generates 17 files + runs codegen
- [ ] Generated code compiles (`go build ./...`)
- [ ] Generated tests pass (`task test`)
- [ ] Proto validates (`buf lint`)
- [ ] Migration runs (`task migrate:up`)
- [ ] Pattern matches user module exactly (minus field-specific logic)

## Unresolved Questions

1. Plural naming: dùng simple `s` suffix hay integrate inflection library? (Recommend: simple `s` + `-plural` flag override cho edge cases)
2. Event topics: auto-add vào `topics.go` hay manual? (Recommend: auto-add, file structure predictable)
