# Boilerplate DX Master Review

Date: 2026-03-06

## Bảng đánh giá tổng hợp

| # | Tiêu chí | Tham khảo (Generic) | Thực tế GNHA | Status | Nhận định Master |
|---|----------|---------------------|--------------|--------|------------------|
| 1 | **Convention over Config** | Dev không cần quyết định structure/naming | fx DI enforce module pattern cố định (`domain/app/adapters`), config via env struct tags (`caarlos0/env`), snake_case package names | **STRONG** | Vượt chuẩn. Dev mới chỉ cần follow folder structure + đọc `adding-a-module.md`. Không có magic, không hidden config files. fx.Module pattern loại bỏ hoàn toàn service locator anti-pattern |
| 2 | **Module Template** | `make module name=user` | `task module:create name=X plural=Y` — scaffold 19 files (proto→migration→sqlc→domain→app→adapters→tests→module.go). Go reserved word validation, conflict detection, auto-run `task generate` | **EXCELLENT** | Vượt xa kỳ vọng. 19 files vs thông thường 5-8 files. Có `embed.FS` templates, validation input, print next-steps guide. Thiếu: interactive mode (chọn có CRUD operations nào) |
| 3 | **Structure rõ ràng** | `cmd/ internal/ pkg/ config/ migrations/` | `cmd/{server,scaffold,seed}/`, `internal/modules/{user,audit,notification}/`, `internal/shared/{database,middleware,events,auth,config,testutil,observability,cron}/`, `db/{migrations,queries}/`, `proto/`, `gen/`, `deploy/` | **STRONG** | Chuẩn Go project layout. Không có `pkg/` (đúng — avoid pkg antipattern). `gen/` tách generated code khỏi hand-written code. `deploy/` chứa docker-compose + traefik — production-ready |
| 4 | **Enforced Architecture** | `handler -> service -> repository` | `grpc handler -> app handler -> domain repository (interface)`. fx DI enforce dependency direction. Domain layer zero imports from infrastructure | **EXCELLENT** | Hexagonal thật sự, không chỉ folder structure. `domain/repository.go` define interface (port), `adapters/postgres/` implement (adapter). fx.Annotate + fx.As enforce tại compile time. Không thể bypass |
| 5 | **Standard Error System** | Response lỗi thống nhất | `DomainError{Code, Message, Err}` + 8 ErrorCode constants + `codeToHTTP` mapping + sentinel errors (`ErrNotFound`, `ErrAlreadyExists`, etc.) + centralized `ErrorHandler` middleware | **STRONG** | Clean implementation. `errors.As()` cho type assertion, `Unwrap()` cho error chain. ErrorHandler xử lý cả DomainError, echo.HTTPError, và unexpected errors. Ghi log `slog.Error` cho 500s |
| 6 | **Standard Response Format** | `{data, error, meta}` | Protobuf messages define response schema. Error response: `{code, message}`. Pagination: `{items, next_cursor, has_more}` | **DIFFERS** | **Tốt hơn generic approach.** Protobuf = type-safe, versioned, auto-generated client code. `{data,error,meta}` wrapper là REST convention — Connect RPC không cần vì protocol đã handle framing. Trade-off đúng |
| 7 | **Middleware sẵn** | logger, request-id, recovery, auth, cors, rate-limit | 10 middleware theo thứ tự: Recovery → RequestID → RequestLogger (sanitized) → BodyLimit(10M) → Gzip(level5) → SecurityHeaders(6 headers) → CORS → Timeout(30s) → RateLimit(Redis-backed, 100/min) → ErrorHandler. Auth+RBAC at route group level | **EXCELLENT** | Vượt kỳ vọng. SecurityHeaders (HSTS, X-Frame-Options, Permissions-Policy, CSP-adjacent). Rate-limit backed by Redis (distributed). Sensitive header redaction in logs. Auth/RBAC tách riêng route group — đúng pattern |
| 8 | **Logging chuẩn** | Structured logging | `log/slog` stdlib — JSON(prod)/text(dev), configurable level, AddSource in dev, sensitive header redaction, request-id correlation, log level by HTTP status (500=Error, 400=Warn, 200=Info) | **STRONG** | Zero-dependency logging (stdlib slog). Context-aware. Redaction là security bonus hiếm thấy ở boilerplate. Thiếu: sampling cho high-throughput (nhưng YAGNI cho hầu hết projects) |
| 9 | **Validation** | Dùng validator chuẩn | 3 tầng: (1) `buf.validate` rules trong proto (email, min_len, max_len, uuid, enum in) → auto-generated; (2) Domain constructors validate business rules; (3) Repository-level uniqueness (Postgres 23505 → `ErrEmailTaken`) | **STRONG** | Vượt chuẩn. Proto validation = zero-boilerplate, declarative, auto-enforced via Connect interceptor. 3-layer validation đúng DDD. Thiếu: custom business validator framework (nhưng domain constructors đủ cho hầu hết cases) |
| 10 | **Testing pattern** | Có template test | Testcontainers (real Postgres/Redis/RabbitMQ), `testutil/` package (NewTestPostgres, fixtures, migrate helper), mockgen directives, race detection (`-race`), integration tags, scaffold generates test boilerplate | **EXCELLENT** | Real infra testing > mock everything. `t.Cleanup` auto-terminate containers. Scaffold tạo sẵn test files. Cả unit (mock repo) lẫn integration (testcontainers). Coverage reporting trong CI |
| 11 | **Dev commands** | `make dev/test/lint/migrate` | Taskfile.yml: `dev:setup`, `dev`, `test`, `test:integration`, `test:coverage`, `lint`, `check`, `build`, `migrate:{up,down,status,create}`, `seed`, `generate:{proto,sqlc,mocks}`, `module:create`, `docker:{build,run}`, `monitor:{up,down}`, `clean` | **STRONG** | 20+ tasks vs typical 5-6. Taskfile > Makefile (cross-platform, YAML, task dependencies, variable support). `dev:setup` = one-command từ zero → running. `check` = lint + test combo |
| 12 | **Local development** | Docker compose chạy toàn bộ stack | `docker-compose.dev.yml`: Postgres 16, Redis 7, RabbitMQ 3 (with management UI), Elasticsearch 8, MailHog. All have healthchecks. Hot-reload via Air. `task dev:setup` = install tools + start infra + migrate + seed | **EXCELLENT** | 5 services với healthcheck. MailHog cho email testing. RabbitMQ management UI (port 15672). ES cho search. `dev:setup` từ clone → running trong 1 command. Thiếu: Adminer/pgAdmin (minor) |
| 13 | **Code generation** | `make module` + `make migration` | `task generate` = buf (proto→Go+TS+OpenAPI) + sqlc (SQL→Go) + mockgen. `task module:create` = 19-file scaffold. `task migrate:create` = goose migration. CI `generated-check` job verify generated code up-to-date | **EXCELLENT** | Triple codegen (proto + sqlc + mocks). CI enforce generated code freshness — prevents "forgot to regenerate" bugs. TypeScript types auto-generated từ proto → frontend team gets type-safe client |
| 14 | **Documentation** | README, ARCHITECTURE, HOW_TO_ADD_FEATURE | `architecture.md`, `code-standards.md` (633 lines), `adding-a-module.md`, `error-codes.md`, `testing-strategy.md`, `project-changelog.md` | **STRONG** | 6 docs covering architecture→standards→onboarding→errors→testing→changelog. `adding-a-module.md` = step-by-step onboarding guide. `code-standards.md` 633 lines = comprehensive. Thiếu: API docs serving (OpenAPI/Swagger endpoint) |
| 15 | **Example module** | `internal/example` | `internal/modules/user/` — full CRUD (5 use cases), hexagonal layers, pagination, events, soft delete. `audit/` — event subscriber pattern. `notification/` — email sender + subscriber | **EXCELLENT** | 3 example modules demonstrating khác nhau patterns: CRUD (user), event consumer (audit), side-effect handler (notification). Tốt hơn 1 generic example vì dev thấy được multiple patterns |
| 16 | **Guardrails** | lint, pre-commit, CI check | golangci-lint, lefthook (pre-commit: lint, pre-push: test), GitLab CI 5-stage pipeline (quality→test→build→deploy-staging→deploy-production). `generated-check` job. `buf breaking` detect proto breaking changes | **STRONG** | Multi-layer: local (lefthook) → CI (lint+test+generated-check+build). Proto breaking change detection là rare bonus. CI integration-test với real services (PG+Redis+RabbitMQ). Deploy pipeline sẵn (staging auto, prod manual) |
| 17 | **Performance defaults** | connection pool, timeout | PG: 25 max / 5 min conns, 1h lifetime, 30min idle. HTTP: 30s timeout, 10MB body limit, Gzip level 5. Redis-backed rate limit. Retry with linear backoff (DB connection). CGO_ENABLED=0 production build | **STRONG** | Tuned values, không phải defaults. Connection pool sizing hợp lý. Retry logic cho DB connection (10 attempts, linear backoff). Static binary build (`-ldflags="-s -w"`). Thiếu: graceful shutdown timeout config (có fx lifecycle nhưng hardcoded) |
| 18 | **Observability** | `/health` + `/metrics` | `/healthz` (liveness) + `/readyz` (checks PG+Redis). OpenTelemetry traces (OTLP/gRPC exporter) + metrics. SigNoz integration. Docker HEALTHCHECK. Trace propagation qua events (Watermill metadata) | **EXCELLENT** | Vượt xa `/health` + `/metrics`. Phân biệt liveness/readiness (K8s-ready). Distributed tracing propagated across async events (OTel → Watermill metadata). SigNoz = full observability platform. Service resource attributes (name, version, environment) |
| 19 | **API versioning** | `/api/v1/` | Proto package versioning (`user.v1`), Connect RPC path = `/user.v1.UserService/GetUser`. `buf breaking` detect breaking changes against main branch | **STRONG** | Schema-level versioning > URL path versioning. Proto package = version baked into type system. `buf breaking` = automated breaking change detection trong CI. Thiếu: multi-version serving (nhưng YAGNI cho v1) |
| 20 | **Scalable module design** | `internal/user` + `internal/order` + `internal/payment` | fx.Module per domain. Zero coupling between modules — communicate via EventBus (Watermill/RabbitMQ). User→publishes events→Audit+Notification subscribe independently | **STRONG** | Modular monolith done right. Event-driven decoupling. Thêm module = thêm folder + register fx.Module trong `main.go`. Audit/Notification demonstrate subscriber pattern. Cron module sẵn cho scheduled tasks. Path to microservices clear nếu cần |

## Score Tổng Kết

| Metric | Count | Chi tiết |
|--------|-------|----------|
| **EXCELLENT** (vượt kỳ vọng) | 8/20 | #2, #4, #7, #10, #12, #13, #15, #18 |
| **STRONG** (đạt chuẩn tốt) | 11/20 | #1, #3, #5, #8, #9, #11, #14, #16, #17, #19, #20 |
| **DIFFERS** (khác approach, hợp lý) | 1/20 | #6 |
| **PARTIAL / FAIL** | 0/20 | — |

**Overall: 19/20 đạt/vượt, 1/20 differs (justified)**

## So sánh với Review trước (2026-03-05)

| Thay đổi | Trước | Sau |
|----------|-------|-----|
| Module Template (#2) | Partial | **EXCELLENT** — `task module:create` đã implement |
| Code generation (#13) | Partial | **EXCELLENT** — scaffold + CI generated-check |
| Overall Pass+ count | 3 | 8 |
| Gaps | 2 (scaffold + migration template) | 0 critical |

## Điểm mạnh nổi bật (Master Perspective)

1. **Hexagonal Architecture thật sự enforce** — Không chỉ folder convention mà fx DI + Go interfaces enforce dependency direction tại compile time
2. **Triple Codegen Pipeline** — Proto (API+types) + SQLC (queries) + Mockgen (tests) = minimize hand-written boilerplate, maximize type safety
3. **Real Infra Testing** — Testcontainers > mocks. Test trên real Postgres/Redis/RabbitMQ = confidence cao
4. **Distributed Tracing across Events** — OTel trace context propagated qua Watermill message metadata = end-to-end visibility
5. **Security-first defaults** — 6 security headers, sensitive log redaction, Argon2id passwords, JWT blacklist, Redis rate-limit, RBAC

## Gaps còn lại (Low Priority)

| Priority | Item | Effort | Cần không? |
|----------|------|--------|------------|
| P2 | OpenAPI/Swagger serving cho frontend team | 2-4h | Có nếu có REST consumers |
| P3 | Module scaffold interactive mode (chọn CRUD operations) | 4-8h | Nice-to-have, current scaffold đủ xài |
| P3 | Graceful shutdown timeout configurable | 30min | fx default 15s đủ cho hầu hết cases |
| P4 | Migration template patterns (soft delete, audit columns) | 2h | Scaffold đã cover basic, refer user module |

## Kết luận

Boilerplate **production-grade, vượt chuẩn industry** cho Go backend. So với 20 tiêu chí DX tham khảo:

- **8 tiêu chí vượt kỳ vọng** (module scaffold 19 files, real infra testing, distributed tracing, 10 middleware, 3 example modules)
- **11 tiêu chí đạt chuẩn tốt** (mỗi tiêu chí đều có implementation chất lượng, không phải placeholder)
- **1 tiêu chí khác approach** nhưng justified (Protobuf response > REST wrapper)
- **0 gaps nghiêm trọng** sau khi implement module scaffold generator

Gap duy nhất đáng cân nhắc: OpenAPI serving nếu có frontend team consume REST. Còn lại là nice-to-have.
