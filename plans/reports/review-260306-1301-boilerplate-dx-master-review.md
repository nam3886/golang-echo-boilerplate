# Boilerplate DX Master Review

Date: 2026-03-06

## Bảng đánh giá tổng hợp

| # | Tiêu chí | Tham khảo (Generic) | Hiện trạng GNHA | Verdict | Nhận định |
|---|----------|---------------------|-----------------|---------|-----------|
| 1 | **Convention over Config** | Dev không cần quyết định structure/naming | fx DI, module pattern cố định (`domain/app/adapters`), config via env struct tags, snake_case Go convention | **PASS+** | Vượt mong đợi: fx enforce DI direction, `env` struct tags = zero manual config parsing, buf.validate = declarative validation. Dev mới chỉ cần follow folder structure |
| 2 | **Module Template (CLI)** | `make module name=user` | `task module:create name=X plural=Y` — scaffold 19 files (proto, migration, queries, domain, app, adapters, tests, module.go) + auto-run `task generate` | **PASS+** | Vượt mong đợi: 19 files vs typical 5-8. Có conflict detection, reserved word check, Go identifier validation. `<10 sec` vs `30+ min` manual |
| 3 | **Structure rõ ràng** | `cmd/`, `internal/`, `pkg/`, `config/`, `migrations/` | `cmd/server,scaffold,seed/`, `internal/modules/*/`, `internal/shared/`, `db/migrations,queries/`, `proto/`, `gen/`, `deploy/` | **PASS** | Chuẩn Go project layout. Tách `proto/` và `gen/` riêng = clean separation giữa schema definition và generated code |
| 4 | **Enforced Architecture** | handler → service → repository | handler → app handler → domain repository (interface ports). fx DI enforce dependency direction. `var _ Interface = (*Impl)(nil)` compile-time check | **PASS+** | Hexagonal architecture thực sự: domain layer zero dependencies, adapters implement ports qua interfaces. Strict hơn generic 3-layer |
| 5 | **Standard Error System** | Response lỗi thống nhất | `DomainError{Code, Message, Err}` + 8 ErrorCode constants + `codeToHTTP` mapping + sentinel errors + centralized `ErrorHandler` middleware | **PASS** | Clean: domain errors auto-map to HTTP status. Sentinel errors (`ErrNotFound`, `ErrForbidden`) reusable. `Wrap()` preserves error chain |
| 6 | **Standard Response Format** | `{data, error, meta}` | Protobuf-defined responses (Connect RPC). Error: `{code, message}`. Pagination: `{items, next_cursor, has_more}` | **DIFFERS** | Không dùng REST wrapper — dùng protobuf messages. Trade-off hợp lý: type-safe, versioned, auto-generated. Tốt hơn cho API-first approach. REST clients dùng Connect JSON mode vẫn nhận structured response |
| 7 | **Middleware sẵn** | logger, request-id, recovery, auth, cors, rate-limit | 10 middleware ordered: recovery → request-id → logger → body-limit(10M) → gzip → security-headers(6 headers) → CORS → timeout(30s) → rate-limit(100/min) → error-handler. Auth+RBAC at route group level | **PASS+** | Vượt mong đợi: thêm gzip, security-headers (HSTS, X-Frame-Options, Permissions-Policy), body-limit. Auth/RBAC tách riêng route group = không block health endpoints |
| 8 | **Logging chuẩn** | Structured logging | `log/slog` stdlib. JSON(prod)/Text(dev). Sensitive header redaction (`authorization`, `cookie`). Request logging: method, path, status, latency_ms, bytes, ip, request_id, user_agent. Log level by status code (500=Error, 400=Warn) | **PASS+** | Zero dependency (stdlib slog). Redaction tự động. Context-aware request_id. Log level phân loại theo HTTP status = dễ alert |
| 9 | **Validation** | Validator chuẩn | 3 tầng: (1) `buf.validate` proto rules (email, uuid, min/max_len, enum `in`) → (2) domain constructors (business rules) → (3) repository uniqueness (DB constraints) | **PASS+** | Vượt mong đợi: validation at schema level = auto-generated, zero manual wiring. `connectrpc.com/validate` interceptor = plug-and-play |
| 10 | **Testing pattern** | Template test sẵn | Unit tests: gomock + stub dependencies. Integration tests: testcontainers (real Postgres/Redis/RabbitMQ). Race detection. Coverage profiling. Scaffold generates test boilerplate | **PASS+** | Vượt mong đợi: real infra testing > mocks. Scaffold tạo sẵn test files cho mỗi module mới. `go test -race -count=1` default |
| 11 | **Dev commands** | `make dev/test/lint/migrate` | Taskfile.yml: `dev`, `dev:setup`, `dev:tools`, `test`, `test:integration`, `test:coverage`, `lint`, `check`, `build`, `migrate:up/down/status/create`, `seed`, `generate`, `module:create`, `docker:build/run`, `monitor:up/down`, `clean` | **PASS+** | 20+ commands vs typical 4-5. `task dev:setup` = one-command full setup. Task > Make: cross-platform, YAML readable, dependency tracking |
| 12 | **Local development** | Docker compose full stack | `docker-compose.dev.yml`: Postgres 16, Redis 7, RabbitMQ 3 (with management UI), Elasticsearch 8, MailHog. All with healthchecks. Hot-reload via Air (`.air.toml`) | **PASS+** | 5 services vs typical 2-3. Healthchecks on all containers. MailHog for email testing. RabbitMQ management UI on :15672 |
| 13 | **Code generation** | `make module`, `make migration` | `task generate` = buf (proto→Go+TS+OpenAPI) + sqlc (SQL→Go) + mockgen (interfaces→mocks). `task module:create` = scaffold 19 files. `task migrate:create` = goose migration | **PASS+** | Triple codegen pipeline: proto + sqlc + mocks. CI `generated-check` job verifies gen files are up-to-date. Scaffold includes auto-generate step |
| 14 | **Documentation** | README, ARCHITECTURE, HOW_TO_ADD_FEATURE | `architecture.md`, `code-standards.md`, `adding-a-module.md`, `error-codes.md`, `testing-strategy.md`, `project-changelog.md` | **PASS** | 6 docs vs typical 3. `adding-a-module.md` = critical onboarding doc. `code-standards.md` comprehensive. Thiếu README.md root (minor) |
| 15 | **Example module** | `internal/example` | `internal/modules/user/` = full CRUD (5 use cases). `audit/` = event subscriber pattern. `notification/` = email + subscriber pattern | **PASS+** | 3 reference modules vs typical 1. User = CRUD template. Audit + Notification = event-driven template. Covers cả sync và async patterns |
| 16 | **Guardrails** | lint, pre-commit, CI check | golangci-lint (11 linters: errcheck, govet, staticcheck, gocritic, revive...). Lefthook pre-commit/push. GitLab CI: lint → generated-check → unit-test → integration-test → build → deploy-staging → deploy-production | **PASS+** | 7-stage CI pipeline vs typical 3. `generated-check` = unique: verify proto/sqlc files match source. `buf breaking` detect breaking API changes. Coverage reporting to GitLab |
| 17 | **Performance defaults** | Connection pool, timeout | PG: 25/5 max/min conns, 1h lifetime, 30min idle. HTTP: 30s timeout, 10MB body limit, gzip level 5. Rate limit: 100 req/min via Redis. Connection retry with backoff (10 attempts) | **PASS** | Tuned cho production. Retry with incremental backoff. Pool sizing reasonable cho medium traffic |
| 18 | **Observability** | `/health`, `/metrics` | `/healthz` (liveness) + `/readyz` (DB+Redis check). OpenTelemetry: traces (OTLP gRPC) + metrics (periodic reader). SigNoz integration. Docker `monitor:up/down` commands | **PASS+** | Vượt mong đợi: distributed tracing + metrics vs typical `/health` endpoint. Liveness/readiness split = K8s-ready. SigNoz = full observability platform |
| 19 | **API versioning** | `/api/v1/` | Proto package versioning: `user.v1`. `buf breaking --against .git#branch=main` detect breaking changes. Connect RPC path: `/user.v1.UserService/` | **PASS** | Schema-level versioning > URL path versioning. Breaking change detection at CI level = prevent accidental API breaks |
| 20 | **Scalable module design** | `internal/user`, `internal/order`, `internal/payment` | fx.Module độc lập. Event-driven: Watermill + RabbitMQ. User/Audit/Notification demonstrate inter-module communication via events. Zero direct module coupling | **PASS+** | Modular monolith done right: modules communicate via event bus, not direct imports. Adding module = folder + fx.Module register. Ready for microservice extraction |

## Score

| Metric | Count |
|--------|-------|
| **PASS+** (vượt mong đợi) | **14/20** |
| **PASS** | **5/20** |
| **DIFFERS** (approach khác, hợp lý) | **1/20** (#6) |
| **Partial / Fail** | **0/20** |

## So sánh với review trước (2026-03-05)

| Metric | Review trước | Review này |
|--------|-------------|------------|
| Pass+ | 3/20 | 14/20 |
| Pass | 14/20 | 5/20 |
| Partial | 2/20 | 0/20 |
| Differs | 1/20 | 1/20 |

**Lý do thay đổi:** Review trước đánh giá conservative. Review này evidence-based, so sánh trực tiếp với generic boilerplate criteria — nhiều tiêu chí thực tế vượt xa mức "đạt" thông thường.

## Gaps còn lại

| Priority | Item | Status |
|----------|------|--------|
| **Low** | README.md root file | Missing — minor, có `architecture.md` thay thế |
| **Low** | OpenAPI/Swagger serving cho frontend | Có `swagger.go` middleware, cần verify hoạt động |
| **Low** | Migration template common patterns (soft delete, audit columns) | Scaffold tạo basic migration, chưa có advanced patterns |

## Điểm mạnh nổi bật (Master perspective)

1. **Protobuf-first pipeline**: Proto → Go + TypeScript + OpenAPI + validation rules. Single source of truth cho toàn bộ API contract
2. **Real infra testing**: Testcontainers cho mọi integration test. Zero mocking ở adapter layer
3. **Event-driven architecture**: Watermill + RabbitMQ sẵn sàng. Audit + Notification modules demonstrate pattern
4. **Security by default**: 6 security headers, Argon2id password hashing, JWT with Redis blacklist, RBAC at route level, sensitive log redaction
5. **Distributed tracing**: OpenTelemetry traces + metrics → SigNoz. Production observability từ day 1
6. **Module scaffold generator**: 19 files, `<10 sec`, conflict detection, reserved word validation — giải quyết hoàn toàn gap lớn nhất
7. **CI/CD maturity**: 7-stage pipeline, generated code verification, breaking change detection, coverage reporting

## Kết luận

Boilerplate **production-grade, DX-optimized**. 14/20 tiêu chí vượt mong đợi so với generic backend boilerplate. Gap duy nhất đáng kể từ review trước (module scaffold) đã được giải quyết hoàn toàn. Gaps còn lại đều low priority.

So với tiêu chuẩn generic: boilerplate này ở mức **top-tier** cho Go backend — hexagonal architecture thực sự enforce, type-safe toàn bộ stack, event-driven sẵn sàng, observability production-ready.
