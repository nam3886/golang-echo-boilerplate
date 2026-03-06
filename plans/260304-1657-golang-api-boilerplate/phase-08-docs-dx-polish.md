# Phase 8: Docs & DX Polish

**Priority:** P1 | **Effort:** M (2-4h) | **Status:** completed
**Depends on:** Phase 7
**Completed:** 2026-03-04

## Context

- [Brainstorm Report](../reports/brainstorm-260304-1636-golang-api-boilerplate.md) — Dev Workflow, Git Hooks, Conventional Commits

## Overview

Final polish: README onboarding guide, golangci-lint config, lefthook git hooks, Swagger UI serving, error codes registry doc, and verify full developer workflow end-to-end.

## Files to Create

```
README.md
.golangci.yml
.lefthook.yml
internal/shared/middleware/swagger.go      # Swagger UI mount (OpenAPI from proto)
docs/error-codes.md                        # Error codes registry for API consumers
docs/architecture.md                       # Architecture overview for developers
docs/adding-a-module.md                    # Step-by-step guide to add new module
```

## Implementation Steps

### 1. README.md
```markdown
# MyApp

Production-ready Go API boilerplate — modular monolith.

## Quick Start

\`\`\`bash
# 1. Clone & setup
git clone <repo> && cd myapp
cp .env.example .env
task dev:setup    # Install tools, start infra, migrate, seed

# 2. Run
task dev          # Hot reload on :8080

# 3. Test
task test                # Unit tests
task test:integration    # Integration (testcontainers)
task check               # Lint + test
\`\`\`

## Stack
Go 1.26 | Echo | Connect RPC | Watermill | PostgreSQL | Redis | RabbitMQ | Elasticsearch | SigNoz

## Architecture
Simplified Hexagonal — see [docs/architecture.md](docs/architecture.md)

## Code Gen
\`\`\`bash
task generate          # Proto (buf) + SQL (sqlc)
task generate:proto    # Protobuf only
task generate:sqlc     # SQL only
\`\`\`

## Adding a Module
See [docs/adding-a-module.md](docs/adding-a-module.md)

## API
- REST/gRPC: Connect RPC on :8080
- Swagger UI: http://localhost:8080/swagger/
- Proto definitions: proto/<module>/v1/*.proto

## Monitoring
\`\`\`bash
task monitor:up    # Start SigNoz → http://localhost:3301
\`\`\`

## Deploy
\`\`\`bash
task docker:build          # Build image
task deploy                # Push + deploy to production
\`\`\`
```

### 2. .golangci.yml
```yaml
run:
  timeout: 5m
  go: "1.26"

linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gocritic
    - gofumpt
    - misspell
    - prealloc
    - revive
    - unconvert
    - unparam

linters-settings:
  gocritic:
    enabled-tags: [diagnostic, style, performance]
  revive:
    rules:
      - name: unexported-return
        disabled: true  # Allow returning unexported types from exported functions
  gofumpt:
    extra-rules: true

issues:
  exclude-dirs: [gen, tmp, vendor]
  max-issues-per-linter: 50
  max-same-issues: 5
```

### 3. .lefthook.yml
```yaml
pre-commit:
  parallel: true
  commands:
    lint:
      glob: "*.go"
      run: golangci-lint run --fix {staged_files}
      stage_fixed: true
    generated:
      run: |
        buf generate 2>/dev/null
        sqlc generate 2>/dev/null
        git diff --exit-code gen/
      fail_text: "Generated code is stale. Run 'task generate' and commit."

pre-push:
  commands:
    test:
      run: go test -race -count=1 ./internal/...
      fail_text: "Tests failed. Fix before pushing."
```

### 4. Swagger UI mount
```go
// internal/shared/middleware/swagger.go
func MountSwagger(e *echo.Echo, cfg *config.Config) {
    if cfg.AppEnv == "production" { return } // Disable in prod

    // Serve OpenAPI spec from gen/openapi/
    e.Static("/swagger/spec", "gen/openapi")

    // Serve Swagger UI (embed or CDN)
    e.GET("/swagger/*", echo.WrapHandler(
        httpSwagger.Handler(
            httpSwagger.URL("/swagger/spec/user/v1/user.swagger.json"),
        ),
    ))
}
```

### 5. Error codes registry
```markdown
# docs/error-codes.md
# API Error Codes

| Code | HTTP | When |
|------|------|------|
| INVALID_ARGUMENT | 400 | Request validation failed |
| UNAUTHENTICATED | 401 | Missing/invalid token |
| PERMISSION_DENIED | 403 | Insufficient permissions |
| NOT_FOUND | 404 | Resource doesn't exist |
| ALREADY_EXISTS | 409 | Duplicate creation |
| FAILED_PRECONDITION | 412 | Business rule violation |
| INTERNAL | 500 | Unexpected server error |
| UNAVAILABLE | 503 | Dependency down |

## Error Response Format
\`\`\`json
{"code": "INVALID_ARGUMENT", "message": "email is required"}
\`\`\`
```

### 6. Adding a module guide
```markdown
# docs/adding-a-module.md
# Adding a New Module

## 1. Create proto
\`proto/<module>/v1/<module>.proto\`
Define service + messages with protovalidate rules.

## 2. Create SQL
\`db/migrations/NNNNN_<module>.sql\` — tables
\`db/queries/<module>.sql\` — sqlc queries

## 3. Generate code
\`task generate\`

## 4. Create module structure
\`\`\`
internal/modules/<module>/
  domain/         # Entity, repository interface, errors
  app/            # Command/query handlers
  adapters/
    postgres/     # Repository impl (sqlc)
    grpc/         # Connect RPC handler
  module.go       # Fx module
\`\`\`

## 5. Register in main.go
Add \`<module>.Module\` to \`fx.New()\` in \`cmd/server/main.go\`.

## 6. Verify
\`task generate && go build ./... && task test\`
```

### 7. Architecture doc
High-level overview: hexagonal architecture, module boundaries, data flow, event flow, middleware chain diagram.

### 8. End-to-end verification
Run through entire developer workflow:
```bash
# Fresh clone simulation
git clone <repo> && cd myapp
cp .env.example .env
task dev:setup          # ← must work first try
task dev                # ← app starts, logs structured
# In another terminal:
curl localhost:8080/healthz           # ← 200
curl localhost:8080/readyz            # ← 200
# Create user via Connect RPC
curl -X POST localhost:8080/user.v1.UserService/CreateUser \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","name":"Test","password":"12345678","role":"member"}'
# ← 200 with user JSON

task test               # ← all pass
task test:integration   # ← all pass
task check              # ← lint + test pass
task docker:build       # ← image builds
```

## Todo

- [x] README.md (quick start, stack, architecture, code gen, deploy)
- [x] .golangci.yml (sensible defaults, exclude gen/)
- [x] .lefthook.yml (pre-commit: lint + generated check, pre-push: test)
- [x] `lefthook install` in dev:setup task
- [x] Swagger UI mount (dev/staging only)
- [x] docs/error-codes.md (error code registry)
- [x] docs/architecture.md (hexagonal overview, diagrams)
- [x] docs/adding-a-module.md (step-by-step guide)
- [x] End-to-end verification (fresh clone → running API → tests pass)
- [x] Verify: `task dev:setup` works on clean machine
- [x] Verify: lefthook pre-commit runs lint on staged files
- [x] Verify: lefthook pre-push runs tests
- [x] Verify: Swagger UI accessible at /swagger/

## Success Criteria

- New developer: clone → `task dev:setup` → `task dev` → API running in <5 minutes
- `task check` passes (lint + tests)
- Pre-commit hook auto-fixes lint issues
- Pre-push hook blocks push if tests fail
- Swagger UI shows all endpoints with try-it-out
- docs/ contains actionable guides for common tasks
- Full workflow verified end-to-end

## Risk Assessment

- **Low risk:** Documentation and tooling config, no complex logic
- **Swagger UI with proto-generated spec:** May need manual aggregation if multiple proto services generate separate spec files

## Completion

This is the final phase. After completion:
- Boilerplate is production-ready
- Tag `v0.1.0` and push
- Use as template for new projects: `git clone <boilerplate> <new-project>`
