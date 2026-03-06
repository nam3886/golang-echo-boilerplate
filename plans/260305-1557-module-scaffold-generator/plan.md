---
status: completed
created: 2026-03-05
completed: 2026-03-05
slug: module-scaffold-generator
---

# Plan: Module Scaffold Generator

## Summary

Implement `task module:create name=X` that scaffolds a complete CRUD module (17 files) following existing user module patterns exactly.

## Context

- Brainstorm: `plans/reports/brainstorm-260305-1557-module-scaffold-generator.md`
- Reference module: `internal/modules/user/`
- Gap: Only significant gap in boilerplate DX assessment (P0)

## Phases

| # | Phase | Status | Effort | Files |
|---|-------|--------|--------|-------|
| 1 | Scaffold CLI | completed | 2h | 1 Go file |
| 2 | Go templates | completed | 4h | 19 .tmpl files |
| 3 | Taskfile + docs | completed | 30min | 2 files |
| 4 | Verify + test | completed | 1h | compile check |

## Architecture

```
cmd/scaffold/
├── main.go              # CLI: parse flags, derive names, execute templates, write files
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

## Dependencies

- None (stdlib only: `text/template`, `embed`, `flag`, `os`, `strings`, `time`)

## Risk

- Template drift: mitigate with CI scaffold+compile test
- Plural edge cases: `-plural` flag override

## Completion Summary

All 4 phases completed successfully on 2026-03-05.

**Phase 1 - Scaffold CLI (cmd/scaffold/main.go)**
- Implements flag parsing (-name, -plural) with validation
- Derives 5 naming variants automatically
- Reads Go module from go.mod dynamically
- Embeds 19 templates via go:embed
- Generates 17 files to correct locations
- Includes conflict detection and next-steps output
- Code review feedback applied: safe UUID parsing, reserved word blocklist, underscore validation

**Phase 2 - Go Templates (19 .tmpl files in cmd/scaffold/templates/)**
- Proto + DB layer: proto.tmpl, migration.tmpl, queries.tmpl
- Domain layer: domain_entity.tmpl, domain_repository.tmpl, domain_errors.tmpl, domain_test.tmpl
- App layer: app_create.tmpl, app_create_test.tmpl, app_get.tmpl, app_list.tmpl, app_update.tmpl, app_delete.tmpl
- Adapter layer: adapter_postgres.tmpl, adapter_postgres_test.tmpl, adapter_grpc_handler.tmpl, adapter_grpc_mapper.tmpl, adapter_grpc_routes.tmpl
- Module registration: module.tmpl
- All templates compile, lint passes, tests pass
- Code review feedback applied: composite cursor pagination index (M-3)

**Phase 3 - Taskfile + Docs**
- Added task module:create to Taskfile.yml with proper chaining (scaffold → generate)
- Updated docs/adding-a-module.md with Quick Start section
- Task properly handles optional -plural flag override

**Phase 4 - Verification + Testing**
- All files generated without error
- Go compilation clean (go build + go vet)
- Proto validation passes (buf lint)
- Linting passes (golangci-lint)
- Tests compile and run successfully
- Conflict detection verified
- Test module cleaned up

**Code Review Fixes Applied**
- H-1: Replaced uuid.MustParse with safe parse helper
- M-1: Added plural flag validation
- M-2: Removed dead ErrInvalidName error
- M-3: Added composite cursor pagination index
- M-4: Read GoModule from go.mod dynamically
- Additional: Go reserved word blocklist, sentinel go.mod check, leading/trailing underscore validation

**Key Files Modified/Created**
- cmd/scaffold/main.go (~150 lines)
- cmd/scaffold/templates/*.tmpl (19 files)
- Taskfile.yml (added module:create task)
- docs/adding-a-module.md (added Quick Start section)
