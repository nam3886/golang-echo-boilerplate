# Boilerplate Gap Fixes Plan

status: pending
created: 2026-03-06
estimated: 1-2 hours

## Context

From master DX review (260305), 5 gaps identified. Gap #4 (Swagger serving) already implemented — `MountSwagger` exists at `cmd/server/main.go:47`. Actual remaining: 4 items.

## Phases

| Phase | Description | Effort | Status |
|-------|-------------|--------|--------|
| 1 | Quick fixes (go mod tidy, stale cleanup) | 5 min | pending |
| 2 | Connect-ES TypeScript codegen | 30 min | pending |
| 3 | Testing strategy doc | 1h | pending |

## Phase 1: Quick Fixes

### 1a. `go mod tidy`
- Run `go mod tidy` to fix `go.uber.org/mock` indirect→direct warning
- Verify: `go build ./...` succeeds

### 1b. Delete stale `gen/openapi/auth/`
- `rm -rf gen/openapi/auth/` — artifact from removed auth proto
- Verify: no auth references remain in gen/

### Files Modified
- `go.mod`, `go.sum`
- Delete: `gen/openapi/auth/v1/auth.swagger.json`

---

## Phase 2: Connect-ES TypeScript Codegen

### Problem
`gen/ts/` exists as empty placeholder. Frontend team (TypeScript) has no auto-generated client.

### Solution
Add `protoc-gen-es` plugin to `buf.gen.yaml` for TypeScript generation.

### Approach: Remote Plugin (Preferred)

No Node.js required in backend repo. Uses buf BSR remote plugin.

#### 2a. Update `buf.gen.yaml`

Add after existing plugins:

```yaml
  # TypeScript protobuf types + service descriptors (Connect-ES v2)
  - remote: buf.build/bufbuild/es
    out: gen/ts
    opt: target=ts
```

#### 2b. Run generation
```bash
task generate:proto
```

Verify: `gen/ts/` contains TypeScript files for user/v1 service.

#### 2c. Update Taskfile.yml `generate:proto`

Add `gen/ts/**/*.ts` to `generates:` list:
```yaml
  generate:proto:
    generates:
      - gen/proto/**/*.go
      - gen/ts/**/*.ts    # ADD
```

#### 2d. Update `.lefthook.yml`

No change needed — `buf generate` already runs in pre-commit hook, covers TS gen automatically.

#### 2e. Update `.gitlab-ci.yml` `generated-check`

Add TS gen check:
```yaml
  - git diff --exit-code gen/ts || (echo "TS generated files out of date" && exit 1)
```

#### Frontend Usage

Frontend team installs:
```bash
npm install @connectrpc/connect @connectrpc/connect-web @bufbuild/protobuf
```

Then imports generated types:
```typescript
import { UserService } from "./gen/ts/user/v1/user_pb"
import { createClient } from "@connectrpc/connect"
import { createConnectTransport } from "@connectrpc/connect-web"

const transport = createConnectTransport({ baseUrl: "http://localhost:8080" })
const client = createClient(UserService, transport)
const user = await client.getUser({ id: "uuid" }) // fully typed
```

### Files Modified
- `buf.gen.yaml` — add ES plugin
- `Taskfile.yml` — add ts generates
- `.gitlab-ci.yml` — add ts gen check
- New: `gen/ts/**/*.ts` (auto-generated)

### Risk
- Remote plugin `buf.build/bufbuild/es` may not support `target=ts` option. Fallback: use local plugin with `@bufbuild/protoc-gen-es` npm package.
- If remote fails, need Node.js in dev env. Acceptable trade-off.

---

## Phase 3: Testing Strategy Doc

### Problem
code-standards.md covers test patterns but lacks dedicated doc on when to use unit vs integration, test organization strategy.

### Solution
Create `docs/testing-strategy.md` (concise, <200 lines).

### Content Outline

1. **Test Types** — unit (no infra), integration (testcontainers), e2e (future)
2. **When Unit** — domain logic, app handlers with mocks, validation
3. **When Integration** — repository layer, SQL queries, event publishing
4. **File Naming** — `*_test.go`, `//go:build integration` tag
5. **Running** — `task test` (unit), `task test:integration` (integration)
6. **Mocking** — gomock, `//go:generate mockgen` directives, stub patterns
7. **Fixtures** — `testutil/fixtures.go`, predefined data
8. **Infrastructure** — testcontainers setup (Postgres, Redis, RabbitMQ)
9. **CI** — unit in MR, integration on main+tags
10. **Coverage** — Cobertura, `task test:coverage`

### Files Created
- `docs/testing-strategy.md`

---

## Updated Gap Status

| # | Item | Status After Plan |
|---|------|-------------------|
| 1 | `go mod tidy` | Phase 1a |
| 2 | Stale `gen/openapi/auth/` | Phase 1b |
| 3 | Connect-ES TS codegen | Phase 2 |
| 4 | Swagger/OpenAPI serving | **ALREADY DONE** (MountSwagger) |
| 5 | Testing strategy doc | Phase 3 |
| 6 | Test coverage expansion | Ongoing (out of scope) |

## Success Criteria

- [ ] `go mod tidy` — no diagnostic warnings
- [ ] `gen/openapi/auth/` removed
- [ ] `buf generate` produces TypeScript in `gen/ts/`
- [ ] CI `generated-check` includes TS files
- [ ] `docs/testing-strategy.md` exists and covers all 10 topics
- [ ] `task check` passes (lint + test)
