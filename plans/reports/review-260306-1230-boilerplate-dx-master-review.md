# Boilerplate DX Master Review

Date: 2026-03-06 | Reviewer: Master-level | Method: Evidence-based codebase inspection

## Tổng quan

Review 20 tiêu chi DX boilerplate. Mỗi tiêu chí được đánh giá dựa trên code thực tế, không phải claim.

## Bảng đánh giá

| # | Tiêu chí | Mong đợi (generic) | Thực tế (gnha-services) | Verdict | Ghi chú |
|---|----------|---------------------|-------------------------|---------|---------|
| 1 | Convention over Config | Dev follow naming/structure convention | fx DI enforce dependency wiring, `env` struct tags cho config, module pattern cố định `domain/app/adapters/module.go`, snake_case Go convention | **STRONG** | Dev mới chỉ cần copy pattern từ `user` module. Config via struct tags = zero manual parsing |
| 2 | Module Template / CLI | `make module name=X` | `task module:create name=X` — scaffold 19 files (proto, migration, sqlc, domain, app, adapters, tests, module.go). Validate Go reserved words, check conflict before write | **EXCELLENT** | Vượt mong đợi: 19 templates, auto-gen code sau scaffold, plural override support, conflict detection |
| 3 | Structure rõ ràng | `cmd/ internal/ pkg/ config/ migrations/` | `cmd/{server,scaffold,seed}`, `internal/modules/{user,audit,notification}`, `internal/shared/{database,middleware,errors,auth,events,observability,testutil}`, `db/migrations/`, `proto/`, `gen/`, `deploy/` | **STRONG** | Chuẩn Go project layout. Tách `gen/` cho generated code = clean git diff. `deploy/` tách infra khỏi app code |
| 4 | Enforced Architecture | handler → service → repository | grpc.Handler → app.Handler → domain.Repository (interface). fx DI enforce direction qua `fx.Annotate + fx.As`. Package boundary = compile-time enforcement | **EXCELLENT** | Hexagonal thực sự enforce, không chỉ convention. Domain không import adapter — Go compiler sẽ reject circular import |
| 5 | Standard Error System | Response lỗi thống nhất | `DomainError{Code, Message, Err}` + 8 ErrorCode constants + sentinel errors + `HTTPStatus()` mapping + centralized `ErrorHandler` middleware | **STRONG** | Error flow clean: domain tạo error → middleware translate to HTTP. `errors.As` unwrap chain. Module errors extend shared sentinels |
| 6 | Standard Response Format | `{data, error, meta}` | Protobuf-defined response messages (`CreateUserResponse{user}`, `ListUsersResponse{items, next_cursor, has_more}`). Error: `{code, message}` | **DIFFERS** | Protobuf approach **tốt hơn** generic wrapper: type-safe, versioned, generated, no runtime reflection. Trade-off: không có unified meta wrapper — nhưng gRPC convention không cần |
| 7 | Middleware sẵn | logger, request-id, recovery, auth, cors, rate-limit | 10 middleware in fixed order: Recovery → RequestID → RequestLogger → BodyLimit(10M) → Gzip(5) → SecurityHeaders(6 headers) → CORS → Timeout(30s) → RateLimit(Redis, 100/min) → ErrorHandler. Auth+RBAC at route group level | **EXCELLENT** | Vượt mong đợi: thêm gzip, security headers (HSTS, X-Frame-Options, CSP), body-limit. Order đúng (recovery first, error handler last). Auth đúng vị trí (route group, không global) |
| 8 | Logging chuẩn | Structured logging | `log/slog` stdlib — JSON(prod) / Text+source(dev). Configurable level via env. Watermill events cũng dùng `slog.Default()` | **PASS** | Dùng stdlib, không vendor lock. Thiếu: sensitive field redaction (claim trước đó nhưng không thấy trong code hiện tại), request correlation ID injection vào logger context |
| 9 | Validation | Validator chuẩn | 3 tầng: (1) `buf.validate` rules trong proto — email, min_len, max_len, uuid, enum `in` (2) domain constructors validate business rules (3) Postgres constraints (unique violation → `ErrEmailTaken`) | **EXCELLENT** | Proto validation = zero boilerplate, auto-generated. Declarative rules in `.proto` file. Business validation in domain layer. DB as final guard |
| 10 | Testing pattern | Template test sẵn | Testcontainers (real Postgres 16), mockgen via `//go:generate`, scaffold generates test files (`domain_test.go`, `app_create_test.go`, `adapter_postgres_test.go`). `testutil/` package: db, redis, rabbitmq, fixtures, migrate helpers | **STRONG** | Real infra testing > mocks. Test scaffold sẵn. Thiếu: integration test example thực tế chạy full flow (hiện chỉ có structure) |
| 11 | Dev commands | `make dev/test/lint/migrate` | Taskfile.yml: `dev`, `dev:setup`, `dev:tools`, `dev:deps`, `test`, `test:integration`, `test:coverage`, `lint`, `check`, `build`, `generate`, `generate:{proto,sqlc,mocks}`, `module:create`, `migrate:{up,down,status,create}`, `seed`, `docker:{build,run}`, `monitor:{up,down}`, `clean` | **EXCELLENT** | 20+ tasks, grouped by concern. `dev:setup` = one command onboarding. `check` = lint+test combo. Task > Make (cross-platform, YAML, better dependency management) |
| 12 | Local development | Docker compose full stack | `docker-compose.dev.yml`: Postgres 16, Redis 7, RabbitMQ 3 (management), ES 8.13, MailHog. All have healthchecks. Hot-reload via Air (`.air.toml`). `task dev:setup` = install tools + start infra + migrate + seed | **EXCELLENT** | One-command setup. Healthchecks on all services. MailHog for email testing. ES cho search. 5 services sẵn |
| 13 | Code generation | `make module` + `make migration` | `task generate` = buf (proto→Go+TS+OpenAPI) + sqlc (SQL→Go) + mockgen. `task module:create` = 19-file scaffold. `task migrate:create` = new migration. Lefthook pre-commit checks generated code freshness | **EXCELLENT** | Triple codegen: proto + sqlc + mocks. Stale check via lefthook. Module scaffold = full CRUD in <10s |
| 14 | Documentation | README, ARCHITECTURE, HOW_TO_ADD_FEATURE | `docs/architecture.md`, `docs/code-standards.md`, `docs/adding-a-module.md`, `docs/error-codes.md`, `docs/testing-strategy.md`, `docs/project-changelog.md` | **STRONG** | 6 docs. `adding-a-module.md` = step-by-step onboarding. `code-standards.md` comprehensive. Thiếu: README.md ở root (hoặc chưa thấy), API reference/Swagger serving |
| 15 | Example module | `internal/example` | `internal/modules/user/` — full CRUD: 5 domain files, 5 app handlers, 5 adapter files, module.go. Plus `audit/` (event subscriber) + `notification/` (event subscriber) | **EXCELLENT** | 3 example modules demonstrating different patterns: CRUD (user), event consumer (audit), event consumer (notification). Better than single "example" module |
| 16 | Guardrails | lint, pre-commit, CI check | lefthook: pre-commit (lint+fix, generated code freshness check), pre-push (race-detected tests). `golangci-lint`. `buf lint` + `buf breaking` detect proto breaking changes | **STRONG** | Multi-layer: editor (lint), commit (format+gen check), push (tests), proto (breaking detection). Thiếu: CI pipeline config chưa verify (GitLab CI claimed nhưng file chưa thấy) |
| 17 | Performance defaults | connection pool, timeout | PG: 25 max / 5 min conns, 1h lifetime, 30min idle. HTTP: 30s timeout, 10MB body limit, Gzip level 5. Redis rate-limit: 100 req/min. Connection retry: 10 attempts with linear backoff | **STRONG** | Tuned cho production. Retry logic built-in. Thiếu: Redis pool config (dùng default), graceful degradation khi Redis down (rate-limit sẽ fail?) |
| 18 | Observability | `/health` + `/metrics` | `/healthz` (liveness) + `/readyz` (DB+Redis check). OpenTelemetry: OTLP trace exporter (gRPC), resource attributes (service name, version, env). Watermill events propagate trace context. SigNoz via docker-compose | **EXCELLENT** | Vượt xa `/health` + `/metrics`: distributed tracing, trace propagation qua event bus, dual health endpoints (K8s pattern). Thiếu: Prometheus metrics endpoint (chỉ có OTel metrics) |
| 19 | API versioning | `/api/v1/` | Proto package versioning: `package user.v1`, `go_package = ".../gen/proto/user/v1;userv1"`. `buf breaking` detect breaking changes. Connect RPC path = `/user.v1.UserService/GetUser` | **STRONG** | Schema-level versioning > URL path versioning. Breaking change detection tự động. Type-safe across versions |
| 20 | Scalable module design | Separate `internal/` modules | fx.Module isolated: mỗi module register riêng trong `cmd/server/main.go`. Event-driven: Watermill + RabbitMQ cho async. Zero coupling giữa modules (audit/notification subscribe events, không import user domain) | **EXCELLENT** | Modular monolith pattern đúng: independent modules, event-based communication, zero circular dependencies. Scale = thêm folder + register fx.Module |

## Scoring

| Verdict | Count | Chi tiết |
|---------|-------|----------|
| EXCELLENT | 10/20 | #2, #4, #7, #9, #11, #12, #13, #15, #18, #20 |
| STRONG | 8/20 | #1, #3, #5, #10, #14, #16, #17, #19 |
| PASS | 1/20 | #8 |
| DIFFERS (hợp lý) | 1/20 | #6 |
| FAIL | 0/20 | — |

**Overall: 18/20 đạt hoặc vượt mong đợi. 10/20 vượt mong đợi.**

## So sánh vs Review trước (260305)

| Thay đổi | Trước | Sau |
|----------|-------|-----|
| Module scaffold | Partial (thiếu CLI) | EXCELLENT (19 templates, conflict detection, validation) |
| Code generation | Partial | EXCELLENT (triple codegen + stale check) |
| Logging | Pass (claim redaction) | Pass (không verify redaction trong code) |
| Overall score | 17 Pass + 3 Pass+ | 18 Pass+ (10 Excellent) |

## Gaps còn lại (ưu tiên)

| Priority | Gap | Impact | Effort |
|----------|-----|--------|--------|
| **P1** | Logger: sensitive field redaction chưa thấy implement | Security — password/token leak trong logs | 2-4h |
| **P1** | CI pipeline config (`.gitlab-ci.yml`) chưa verify exist | No automated gate on merge | 1h |
| **P2** | Redis pool sizing (dùng go-redis default) | Performance under load | 30min |
| **P2** | Rate-limit graceful degradation khi Redis down | Availability — 500 errors nếu Redis fail | 1-2h |
| **P3** | Prometheus `/metrics` endpoint (chỉ có OTel push) | Monitoring gap nếu không dùng SigNoz | 1h |
| **P3** | OpenAPI/Swagger serving cho external consumers | Frontend/mobile team integration | 2h (buf generate đã có, chỉ cần serve) |
| **P3** | Integration test example chạy full flow | Onboarding — dev mới không biết test integration thế nào | 2h |

## Nhận định tổng thể

Boilerplate **production-grade, vượt chuẩn DX cho Go backend**. Điểm mạnh nổi bật:

1. **Protobuf-first**: type-safe từ API → DB, auto-gen, breaking change detection — approach modern hơn REST + JSON schema
2. **Real enforcement**: hexagonal architecture không chỉ convention mà enforce bởi Go compiler (package boundary) + fx DI
3. **Event-driven sẵn sàng**: Watermill + RabbitMQ + trace propagation — async workflows ready day 1
4. **Module scaffold**: 19-file CRUD trong <10s — DX killer feature
5. **Testing infra**: testcontainers real database — tests reflect production behavior, không mock illusion

Gap chính: logging security (P1) và CI pipeline verification (P1). Còn lại là polish.
