# Boilerplate DX Master Review

Date: 2026-03-06 | Reviewer: Master-level assessment | Codebase: gnha-services

## Bảng Nhận Định Tổng Quan

| # | Tiêu chí | Tham khảo | Thực tế GNHA | Verdict | Nhận định |
|---|----------|-----------|--------------|---------|-----------|
| 1 | Convention over Config | Dev không cần quyết định structure/naming | fx DI auto-wire, env struct tags (`caarlos0/env`), module pattern cố định `domain/app/adapters/module.go`, snake_case enforced trong scaffold | **PASS+** | Vượt: không chỉ convention mà còn **enforce** qua DI + codegen. Dev mới chỉ cần `task module:create` rồi customize fields |
| 2 | Module Template (CLI) | `make module name=user` | `task module:create name=X plural=Y` → scaffold 19 files (proto, migration, queries, domain, app, adapters, tests, module.go). Validate Go reserved words, conflict check trước khi ghi | **PASS+** | Vượt: 19 files > typical 5-8. Có plural override, reserved word guard, auto-run `task generate` sau scaffold |
| 3 | Structure rõ ràng | `cmd/`, `internal/`, `pkg/`, `config/`, `migrations/` | `cmd/{server,scaffold,seed}/`, `internal/{modules,shared}/`, `db/{migrations,queries}/`, `proto/`, `gen/`, `deploy/`, `docs/` | **PASS** | Chuẩn Go project layout. Tách `gen/` riêng (generated code không lẫn source). `deploy/` chứa Docker compose — gọn |
| 4 | Enforced Architecture | handler → service → repository | handler → app handler → domain repository (interface port). fx DI enforce qua `fx.Annotate` + `fx.As(new(domain.UserRepository))` | **PASS+** | Hexagonal thực sự, không chỉ 3-layer. Domain port interface → adapter implement. Dependency inversion đúng nghĩa |
| 5 | Standard Error System | Response lỗi thống nhất | `DomainError{Code, Message, Err}` + 8 ErrorCode constants + `codeToHTTP` mapping + sentinel errors + `Wrap()` for chaining + centralized `ErrorHandler` | **PASS** | Solid. Error code → HTTP status mapping tự động. Sentinel errors cho common cases. `Unwrap()` support error chain |
| 6 | Standard Response Format | `{data, error, meta}` | Protobuf-defined response per RPC: `CreateUserResponse{user}`, `ListUsersResponse{items, next_cursor, has_more}`. Error: Connect RPC error model | **DIFFERS** | Không dùng generic wrapper — dùng typed protobuf response. Trade-off hợp lý: type-safe, versioned, auto-generated. Frontend dùng generated TS client thay vì parse generic JSON |
| 7 | Middleware sẵn | logger, request-id, recovery, auth, cors, rate-limit | 10 middleware ordered: recovery → request-id → request-logger → body-limit(10M) → gzip(L5) → security-headers → CORS → timeout(30s) → rate-limit(100/min) → error-handler. Auth+RBAC ở route group | **PASS+** | Vượt: thêm body-limit, gzip, security-headers, RBAC interceptor. Order đúng (recovery first, error-handler last) |
| 8 | Logging chuẩn | Structured logging | `log/slog` stdlib. JSON(prod) / text+source(dev). Configurable level (debug/info/warn/error). Context-aware via slog groups | **PASS** | Zero-dependency logging (stdlib). Đủ cho production. Nếu cần: thiếu sensitive field redaction handler (có thể thêm custom slog.Handler) |
| 9 | Validation | Validator chuẩn | 3 tầng: (1) `buf.validate` declarative trên proto fields → auto-generated, (2) `connectrpc.com/validate` interceptor enforce runtime, (3) domain constructors validate business rules | **PASS+** | Vượt: proto-level validation = zero boilerplate per field. `buf.validate` rules: email, uuid, min_len, max_len, enum `in` — tất cả declared, không coded |
| 10 | Testing pattern | Template test có sẵn | Testcontainers (real PG/Redis/RabbitMQ), `testutil/` package (db, redis, rabbitmq, fixtures, migrate helpers), mockgen directives, scaffold generates `_test.go` files, race detection `-race` | **PASS+** | Vượt: real infra > mocks. Scaffold auto-generates domain_test + app_create_test + adapter_postgres_test. `go test -race` default |
| 11 | Dev commands | `make dev/test/lint/migrate` | Taskfile.yml: `dev`, `dev:setup`, `dev:tools`, `dev:deps`, `test`, `test:integration`, `test:coverage`, `lint`, `check`, `build`, `migrate:{up,down,status,create}`, `seed`, `generate:{proto,sqlc,mocks}`, `module:create`, `docker:{build,run}`, `monitor:{up,down}`, `clean` | **PASS+** | Vượt: 20+ tasks vs typical 4-6. `task dev:setup` = one-command from zero. Taskfile > Makefile (cross-platform, YAML, deps tracking) |
| 12 | Local development | Docker compose full stack | `docker-compose.dev.yml`: Postgres 16, Redis 7, RabbitMQ 3 (+ management UI), Elasticsearch 8.13, MailHog. All with healthchecks + persistent volumes. Air hot-reload via `.air.toml` | **PASS+** | Vượt: 5 services + healthchecks + MailHog (email testing). `task dev:setup` → tools + infra + migrate + seed = zero-to-running |
| 13 | Code generation | `make module`, `make migration` | `task generate` = buf(proto→Go+TS+OpenAPI) + sqlc(SQL→Go) + mockgen(interfaces→mocks). `task module:create` scaffold 19 files. `task migrate:create` via goose. CI `generated-check` verifies gen files in sync | **PASS+** | Vượt: 4 generators chained. CI checks generated code freshness — prevents drift. Proto generates Go + TypeScript + OpenAPI simultaneously |
| 14 | Documentation | README, ARCHITECTURE, HOW_TO_ADD_FEATURE | `docs/`: architecture.md, code-standards.md, adding-a-module.md (Quick Start with generator), error-codes.md, testing-strategy.md, project-changelog.md | **PASS** | 6 docs covering architecture → standards → onboarding → errors → testing → changelog. `adding-a-module.md` updated with scaffold CLI |
| 15 | Example module | `internal/example` | `internal/modules/user/` = full CRUD (5 app handlers, postgres adapter, grpc adapter, domain with events, tests). `audit/` = event subscriber. `notification/` = email sender + subscriber | **PASS+** | Vượt: 3 modules demonstrating different patterns — CRUD, event subscriber, notification sender. Not just "hello world" example |
| 16 | Guardrails | lint, pre-commit, CI check | golangci-lint, lefthook (pre-commit: lint, pre-push: test), GitLab CI 5-stage pipeline (quality → test → build → deploy-staging → deploy-prod). `buf breaking` detect proto breaking changes. `generated-check` job | **PASS+** | Vượt: CI has generated-code freshness check + proto breaking change detection + coverage reports. Deploy pipeline staging→prod(manual) |
| 17 | Performance defaults | connection pool, timeout | PG: 25 max / 5 min conns, 1h lifetime, 30min idle. HTTP: 30s timeout, 10MB body limit, gzip L5. Redis rate-limit 100/min. Retry with backoff (postgres connect). CORS MaxAge 3600 | **PASS** | Production-tuned. Retry logic on DB connect (10 attempts, linear backoff). Nếu cần: thiếu Redis pool config explicit (dùng go-redis defaults — OK cho most cases) |
| 18 | Observability | `/health`, `/metrics` | `/healthz` (liveness — process OK), `/readyz` (readiness — PG+Redis ping). OpenTelemetry traces+metrics via OTLP gRPC exporter. SigNoz stack via `docker-compose.monitor.yml`. Docker HEALTHCHECK | **PASS+** | Vượt: split liveness/readiness (K8s-native). Distributed tracing sẵn. `task monitor:up` = one-command SigNoz. Thiếu Prometheus `/metrics` endpoint (OTel collector handles) |
| 19 | API versioning | `/api/v1/` | Proto package `user.v1`, Connect RPC path `/user.v1.UserService/`. `buf breaking` detect breaking changes automatically. Schema-level versioning | **PASS** | Proto versioning > URL path versioning: type-safe, backward-compatible detection automated. Frontend gets versioned TS types |
| 20 | Scalable module design | `internal/user`, `internal/order` | fx.Module per domain (`user.Module`, `audit.Module`, `notification.Module`). Event-driven via Watermill/RabbitMQ. Zero coupling between modules — communicate via events. Cron module isolated | **PASS** | Modular monolith đúng chuẩn. Thêm module = thêm folder + register fx.Module trong main.go. Event bus cho async communication |

## Score

| Metric | Count |
|--------|-------|
| **PASS+** (vượt mong đợi) | **11/20** (#1, #2, #4, #7, #9, #10, #11, #12, #13, #15, #16) |
| **PASS** (đạt chuẩn) | **8/20** (#3, #5, #8, #14, #17, #18, #19, #20) |
| **DIFFERS** (khác approach, hợp lý) | **1/20** (#6) |
| **FAIL** | **0/20** |

**Overall: 19/20 đạt/vượt, 1/20 khác approach có lý do**

## So sánh với Review trước (2026-03-05)

| Thay đổi | Trước | Sau |
|----------|-------|-----|
| Module scaffold | Partial (thiếu CLI) | **PASS+** — 19-file scaffold, reserved word guard |
| Test template | Partial | **PASS+** — scaffold generates 3 test files |
| Pass+ count | 3/20 | **11/20** |
| Overall | 17 pass + 2 partial + 1 differs | **19 pass + 1 differs** |

## Gaps còn lại (xếp theo priority)

| Priority | Item | Effort | Impact |
|----------|------|--------|--------|
| P2 | OpenAPI/Swagger serving cho frontend team (buf generates OpenAPI nhưng chưa serve) | Vài giờ | Frontend DX |
| P3 | Sensitive field redaction trong slog (password, token fields) | Vài giờ | Security logging |
| P3 | Migration template kèm common patterns (soft delete, audit columns, indexes) | Vài giờ | Convenience |
| P4 | Redis pool config explicit (hiện dùng go-redis defaults) | 30 phút | Fine-tuning |

## Điểm mạnh nổi bật

1. **Protobuf-first**: API → validation → Go types → TS types → OpenAPI, tất cả từ 1 source of truth
2. **Real infra testing**: testcontainers chạy PG/Redis/RabbitMQ thật, không mock — confidence cao
3. **19-file scaffold**: `task module:create name=X` → full hexagonal module trong <10 giây
4. **Event-driven sẵn sàng**: Watermill + RabbitMQ, audit/notification demonstrate subscriber pattern
5. **CI pipeline mature**: lint → generated-check → unit-test → integration-test → build → deploy(staging/prod)
6. **Distributed tracing**: OpenTelemetry + SigNoz one-command setup
7. **Security defaults**: Argon2id password hashing, JWT with configurable TTL, RBAC interceptor, security headers, rate-limit

## Kết luận

Boilerplate **production-grade, vượt mong đợi ở 11/20 tiêu chí**. So với template "tối ưu DX" ban đầu, GNHA services không chỉ đạt mà còn nâng cấp approach: protobuf thay REST wrapper, hexagonal thay 3-layer, testcontainers thay mocks, 19-file scaffold thay basic template.

Gap duy nhất đáng kể đã fix (module scaffold). Gaps còn lại đều P2-P4, không block development.
