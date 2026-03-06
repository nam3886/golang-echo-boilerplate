# Boilerplate DX Master Review

Date: 2026-03-06 | Reviewer: Master-level | Method: Evidence-based codebase audit

## Review Table

| # | Tieu chi | Tham khao (generic) | Thuc te (GNHA) | Verdict | Nhan dinh |
|---|----------|---------------------|-----------------|---------|-----------|
| 1 | Convention over Config | Structure + naming conventions, dev follows rules | fx DI, env struct tags (`caarlos0/env`), module pattern co dinh (`domain/app/adapters`), proto-first codegen | **PASS+** | Vuot: dev khong can quyet dinh gi — fx enforce dependency direction, buf enforce API contract, sqlc enforce query typing. Zero ambiguity. |
| 2 | Module Template / CLI | `make module name=X` | `task module:create name=X` — 19 templates, auto-run `buf generate` + `sqlc generate` sau scaffold | **PASS** | Co day du. 19 file scaffold (proto, migration, domain, app x5, adapters x4, module, test). Plural support: `task module:create name=category plural=categories`. |
| 3 | Structure ro rang | `cmd/`, `internal/`, `pkg/`, `config/`, `migrations/` | `cmd/{server,scaffold,seed}/`, `internal/{modules,shared}/`, `db/migrations/`, `proto/`, `gen/`, `deploy/` | **PASS+** | Vuot: tach rieng `gen/` (generated code), `deploy/` (Docker/CI), `proto/` (source of truth). Clean separation giua hand-written vs generated code. |
| 4 | Enforced Architecture | handler -> service -> repository | handler -> app handler -> domain <- repository (hexagonal, interface ports) | **PASS+** | Vuot: khong chi 3-layer ma hexagonal thuc su. `domain.UserRepository` interface, `postgres.NewPgUserRepository` implement, fx DI wire. Dependency inversion dung chuan. Go package boundaries enforce — adapter khong import adapter khac. |
| 5 | Standard Error System | Response loi thong nhat | `DomainError{Code, Message, Err}` + 8 ErrorCode constants + `codeToHTTP` mapping + sentinel errors + centralized `ErrorHandler` | **PASS** | Solid. `errors.As()` unwrap chain, Echo error fallback, unexpected error log + generic 500. Gom 67 LOC — gon. |
| 6 | Standard Response Format | `{data, error, meta}` | Protobuf message responses (Connect RPC). Error: `{code, message}`. Pagination: cursor in proto message. | **DIFFERS** | Khong dung REST wrapper — dung protobuf typed responses. Day la **thiet ke tot hon**: type-safe, versioned, generated. Frontend nhan TS types tu `gen/ts/`. Trade-off: khong co generic `meta` field — nhung proto message co the extend bat ky luc nao. |
| 7 | Middleware san | logger, request-id, recovery, auth, cors, rate-limit | 10 middleware ordered: recovery → request-id → logger → body-limit → gzip → security-headers → CORS → timeout → rate-limit → error-handler. Auth+RBAC at route group level. | **PASS+** | Vuot mong doi. Them: body-limit (10MB), gzip (level 5), security headers (6 headers: HSTS, X-Frame-Options, CSP via Permissions-Policy, etc.), timeout (30s). Auth+RBAC tach rieng theo route group — dung pattern. |
| 8 | Logging chuan | Structured logging | `log/slog` stdlib, JSON (prod) / text+source (dev), configurable level via env | **PASS** | Dung stdlib — khong vendor lock. JSON cho prod, text+source cho dev. Chua co: request-scoped context fields (trace_id, user_id inject vao logger). Minor gap. |
| 9 | Validation | Validator chuan | 3 tang: buf.validate (proto rules) → domain constructors → repository constraints (unique violation) | **PASS+** | Vuot: validation auto-generated tu proto annotations (`string.email`, `string.min_len`, `string.uuid`, `int32.gte/lte`). Zero boilerplate validation code. Buf interceptor reject invalid requests truoc khi toi handler. |
| 10 | Testing pattern | Template test co san | testcontainers (real PG/Redis/RabbitMQ), `_test.go` scaffold trong module templates, gomock infrastructure, race detection | **PASS** | Real infra > mocks. `testutil.NewTestPostgres()` spin up PG container, auto-cleanup. Mock repo generated via `mockgen`. Scaffold generates test boilerplate. Chua co: shared test fixtures cho integration tests. |
| 11 | Dev commands | make dev/test/lint/migrate | Taskfile: `dev:setup`, `dev`, `test`, `test:integration`, `test:coverage`, `lint`, `check`, `build`, `migrate:{up,down,status,create}`, `seed`, `module:create`, `generate`, `docker:{build,run}`, `monitor:{up,down}`, `clean` | **PASS+** | Vuot: 20+ commands. `task dev:setup` = one-command setup (tools + deps + migrate + seed). Taskfile > Makefile: cross-platform, YAML, source/generates declarations cho incremental builds. |
| 12 | Local development | Docker compose full stack | `docker-compose.dev.yml`: PG16, Redis7, RabbitMQ3 (management UI), ES8, MailHog. Hot-reload via Air. | **PASS+** | Vuot: 5 services voi healthchecks. MailHog cho email testing. RabbitMQ management UI (port 15672). ES cho full-text search. `task dev:setup` → `task dev` = 2 commands to running. |
| 13 | Code generation | make module / make migration | `task generate` = buf (proto→Go+TS+OpenAPI) + sqlc (SQL→Go) + mockgen. `task module:create` = scaffold 19 files. `task migrate:create` = goose migration. | **PASS+** | Vuot: triple codegen pipeline. Proto generate 3 outputs (Go, TypeScript, OpenAPI). CI `generated-check` verify gen files in sync. Breaking change detection via `buf breaking`. |
| 14 | Documentation | README, ARCHITECTURE, HOW_TO_ADD_FEATURE | `architecture.md`, `code-standards.md` (633L), `adding-a-module.md`, `error-codes.md`, `testing-strategy.md`, `project-changelog.md` | **PASS** | 6 docs. `adding-a-module.md` co Quick Start voi generator. `code-standards.md` comprehensive (633 lines). Thieu: README.md overview cho newcomers, API reference auto-gen. |
| 15 | Example module | `internal/example` | `internal/modules/user/` (16 files, full CRUD) + `audit/` (event subscriber) + `notification/` (event subscriber) | **PASS+** | Vuot: 3 modules demonstrate 2 patterns — CRUD (user) va event-driven subscriber (audit, notification). User module co 5 use cases, tests, mappers. Real implementation, khong fake. |
| 16 | Guardrails | lint, pre-commit, CI check | `golangci-lint`, lefthook (pre-commit/push), GitLab CI 4-stage pipeline (quality→test→build→deploy), `buf breaking` detection, `generated-check` job | **PASS+** | Vuot: CI co `generated-check` verify codegen sync + `buf breaking` detect breaking proto changes. 4-stage pipeline voi integration tests chay real PG/Redis/RabbitMQ. Auto deploy staging on main, manual deploy prod on tag. |
| 17 | Performance defaults | connection pool, timeout | PG: 25 max / 5 min conns, 1h lifetime, 30min idle. Redis rate-limit. HTTP: 30s timeout, 10MB body limit, gzip level 5. Retry: 10 attempts voi backoff. | **PASS** | Production-tuned. PG connection retry voi incremental backoff (1s, 2s, ..., 10s). Chua co: Redis pool config tuning, graceful degradation khi dependency down. |
| 18 | Observability | /health, /metrics | `/healthz` (liveness), `/readyz` (DB+Redis check), OpenTelemetry traces (OTLP gRPC) + metrics (OTLP gRPC), SigNoz via `docker-compose.monitor.yml` | **PASS+** | Vuot nhieu: distributed tracing + metrics + liveness/readiness probes. Proper k8s-ready health endpoints. SigNoz monitoring stack one-command setup (`task monitor:up`). |
| 19 | API versioning | `/api/v1/` | Proto package versioning (`user.v1`), Connect RPC path-based, `buf breaking` detect breaking changes tu proto schema | **PASS+** | Vuot: schema-level versioning > URL path versioning. Proto package `user.v1` → generated path `/user.v1.UserService/`. Breaking change detection automated trong CI. |
| 20 | Scalable module design | `internal/user`, `internal/order`, `internal/payment` | fx.Module doc lap, event-driven via Watermill/RabbitMQ, zero coupling giua modules, subscriber pattern cho cross-module communication | **PASS+** | Vuot: modular monolith thuc su. Modules giao tiep qua events, khong import truc tiep. fx.Module isolate dependency graph. Them module = them folder + register fx.Module trong `main.go`. |

## Score Summary

| Metric | Count |
|--------|-------|
| **PASS+** (vuot mong doi) | 12/20 (#1,3,4,7,9,11,12,13,15,16,18,19,20) |
| **PASS** | 6/20 (#2,5,8,10,14,17) |
| **DIFFERS** (khac nhung hop ly) | 1/20 (#6) |
| **FAIL** | 0/20 |

**Overall: 19/20 dat hoac vuot. 0 fail.**

## So sanh voi reference boilerplate

| Aspect | Reference (generic) | GNHA (thuc te) | Ai hon? |
|--------|---------------------|-----------------|---------|
| Architecture | 3-layer (handler→service→repo) | Hexagonal (ports+adapters) voi fx DI | GNHA |
| API protocol | REST + JSON wrapper | Connect RPC (gRPC + HTTP) + Protobuf | GNHA — type-safe, generated |
| Validation | Runtime validator (go-playground) | Compile-time proto rules (buf.validate) | GNHA — zero boilerplate |
| Code generation | Module + migration | Proto + SQL + mocks + module scaffold | GNHA — triple pipeline |
| Testing | Mock-based | Real infra (testcontainers) | GNHA — higher confidence |
| Observability | /health + /metrics | /healthz + /readyz + OTel traces + metrics | GNHA — distributed tracing |
| CI/CD | lint + test + build | lint + generated-check + breaking-detect + test + integration-test + build + deploy | GNHA — comprehensive |
| Module scaffold | Basic make command | 19-template CLI voi auto-codegen | GNHA |

## Gaps con lai (honest assessment)

| Priority | Gap | Impact | Effort |
|----------|-----|--------|--------|
| P1 | Logger thieu request-scoped fields (trace_id, user_id) — slog co WithGroup/WithAttrs nhung chua wire vao middleware | Debug production harder khi khong correlate log voi trace | 2-4h |
| P2 | Thieu README.md tong quan cho newcomer (setup, tech stack, architecture overview) | Onboarding friction cho dev chua doc docs/ | 1h |
| P2 | Swagger UI dang serve tu CDN (unpkg) — production security concern nho | CSP violation neu bat strict CSP | 1h |
| P3 | Redis pool config chua tune (dang dung default go-redis) | Khong anh huong nhieu o scale nho | 30min |
| P3 | Integration test fixtures/helpers chua du cho multi-module scenarios | Test coverage gap khi them modules | Half day |

## Diem manh noi bat (top 5)

1. **Protobuf-first approach** — Single source of truth: proto → Go types + TS types + OpenAPI + validation rules. Thay doi 1 file, generate het.
2. **Real infrastructure testing** — testcontainers chay PG/Redis/RabbitMQ that, khong mock. Test result = production behavior.
3. **Hexagonal architecture thuc su enforce** — fx DI + Go package boundaries + interface ports. Khong the vi pham dependency direction.
4. **CI pipeline comprehensive** — Generated code sync check + proto breaking change detection + real infra integration tests. Catch loi truoc khi merge.
5. **One-command everything** — `task dev:setup` (tools+infra+migrate+seed), `task module:create` (19 files), `task monitor:up` (observability stack). DX tot.

## Ket luan

Boilerplate **production-grade, vuot mong doi o 12/20 tieu chi**. So voi reference generic boilerplate, GNHA approach vuot tren moi aspect: type-safety (protobuf), testing (real infra), architecture (hexagonal), observability (distributed tracing), CI (breaking change detection).

Gap duy nhat dang luu y: request-scoped logging (P1). Phan con lai la nice-to-have.

**Rating: 9.2/10** — Mot trong nhung Go boilerplate tot nhat ma toi review.
