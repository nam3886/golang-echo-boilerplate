# Boilerplate DX Master Review

Date: 2026-03-06

## Review Summary

| # | Tiêu chí | Mong đợi (tham khảo) | Thực tế trong codebase | Verdict | Nhận định |
|---|----------|----------------------|------------------------|---------|-----------|
| 1 | Convention over Config | Dev không cần quyết định structure/naming | fx DI, module pattern cố định (domain/app/adapters), config via `env` struct tags, snake_case SQL, PascalCase Go | **PASS** | Convention rõ ràng. Dev follow folder structure là đủ |
| 2 | Module Template / CLI | `make module name=user` | `task module:create name=X` → scaffold 19 files (proto→migration→queries→domain→app→adapters→module.go→tests) | **PASS+** | Vượt mong đợi: 19 files vs typical 5-8. Auto-run codegen sau scaffold |
| 3 | Structure rõ ràng | `cmd/`, `internal/`, `pkg/`, `config/`, `migrations/` | `cmd/` (server, scaffold, seed), `internal/modules/` + `internal/shared/`, `db/migrations/`, `db/queries/`, `deploy/`, `gen/`, `proto/` | **PASS** | Chuẩn Go layout. Không có `pkg/` — đúng vì internal-only project |
| 4 | Enforced Architecture | handler → service → repository | handler → app handler → repository qua interface ports. fx DI enforce dependency direction | **PASS+** | Hexagonal > 3-layer. Interface ports enforce direction compile-time |
| 5 | Standard Error System | Response lỗi thống nhất | `DomainError{Code, Message, Err}` + 8 error codes → HTTP mapping + sentinels + Connect RPC mapping | **PASS** | Nhất quán cả REST lẫn gRPC. `errors.As` unwrap chain |
| 6 | Standard Response Format | `{data, error, meta}` | Protobuf JSON qua Connect RPC. Error: `{code, message}`. Health: `{status}` | **DIFFERS** | Protobuf approach type-safe hơn, versioned, generated. Không cần wrapper thủ công |
| 7 | Middleware sẵn | logger, request-id, recovery, auth, cors, rate-limit | 10 MW: recovery, request-id, logger (redaction), body-limit(10MB), gzip(lv5), security-headers(CSP/HSTS), CORS, timeout(30s), rate-limit(Redis), error-handler | **PASS+** | Thêm security-headers, gzip, body-limit. Rate-limit dùng Redis sliding window (scalable multi-instance) |
| 8 | Logging chuẩn | Structured logging | `log/slog` stdlib. JSON(prod)/text(dev). Sensitive redaction (Auth/Cookie→REDACTED). Context-aware. Configurable level | **PASS** | Zero-dependency logging. Redaction là điểm cộng security |
| 9 | Validation | Validator chuẩn | 3 tầng: buf.validate declarative (proto) → protovalidate interceptor (runtime) → domain constructors (business rules) | **PASS+** | Proto validation auto-generate, giảm boilerplate. Runtime interceptor reject trước handler |
| 10 | Testing pattern | Template test | 3 tầng: unit (gomock), integration (testcontainers: real PG/Redis/RabbitMQ), fixtures. Scaffold CLI generate test boilerplate | **PASS+** | Real infra > mocks. Testcontainers auto-cleanup. Race detection enabled |
| 11 | Dev commands | make dev/test/lint/migrate | Taskfile: dev, test, test:integration, test:coverage, lint, migrate:up/down/status/create, build, check, seed, module:create, generate, monitor:up, clean | **PASS** | 20+ tasks. Taskfile > Makefile (cross-platform, readable YAML, dependencies) |
| 12 | Local development | Docker compose full stack | docker-compose.dev.yml: PG16, Redis7, RabbitMQ3(+mgmt UI), ES8, MailHog. Hot-reload via Air. `task dev:setup` = one command | **PASS+** | Vượt mong đợi: MailHog (email preview), ES, RabbitMQ management UI |
| 13 | Code generation | make module, make migration | `task generate` = buf(proto→Go+OpenAPI) + sqlc(SQL→Go) + mockgen. `task module:create` scaffold. `task migrate:create` | **PASS** | Triple codegen pipeline. Lefthook pre-commit verify stale generated code |
| 14 | Documentation | README, ARCHITECTURE, HOW_TO_ADD_FEATURE | architecture.md, code-standards.md(633L), adding-a-module.md(manual+CLI), error-codes.md, project-changelog.md | **PASS** | `adding-a-module.md` cover cả manual lẫn CLI path. Code-standards chi tiết |
| 15 | Example module | `internal/example` | `internal/modules/user/` full CRUD (5 use cases) + `audit/` (event subscriber→DB) + `notification/` (event→email) | **PASS+** | 3 modules demonstrate 3 patterns: CRUD, event consumer→DB, event consumer→external |
| 16 | Guardrails | lint, pre-commit, CI | lefthook pre-commit(lint+generated code check), pre-push(test+race), golangci-lint(11 linters), buf breaking change detection | **PASS** | `buf breaking` detect proto breaking changes. Generated code staleness check |
| 17 | Performance defaults | connection pool, timeout | PG: 25/5 conns, 1h lifetime. Redis: 10×NumCPU pool. HTTP: 30s timeout, 10MB body. RabbitMQ retry: 3×backoff. Cron: Redis distributed lock | **PASS** | Production-tuned. Distributed cron lock prevent duplicate execution across replicas |
| 18 | Observability | /health, /metrics | `/healthz` + `/readyz`, OpenTelemetry traces+metrics, OTLP gRPC exporter, SigNoz, trace context propagation into RabbitMQ messages | **PARTIAL** | Traces/metrics vượt mong đợi. **Nhưng `/readyz` static — không check DB/Redis/RabbitMQ liveness** |
| 19 | API versioning | `/api/v1/` | Proto package `user.v1`, Connect RPC path `/user.v1.UserService/`. `buf breaking` detect changes. Directory structure support v2 alongside | **PASS** | Schema-level versioning > URL path. Breaking change detection automated |
| 20 | Scalable module design | Separate module folders | fx.Module độc lập, event-driven (Watermill/RabbitMQ), zero coupling. Thêm module = thêm folder + register fx.Module | **PASS** | Modular monolith ready for extraction to microservices nếu cần |

## Score

| Metric | Count |
|--------|-------|
| PASS+ (vượt mong đợi) | 7/20 (#2, #4, #7, #9, #10, #12, #15) |
| PASS | 11/20 |
| PARTIAL | 1/20 (#18) |
| DIFFERS (hợp lý) | 1/20 (#6) |

**Overall: 18/20 đạt/vượt. 1 differs (hợp lý). 1 partial.**

## Điểm mạnh nổi bật (so với boilerplate thông thường)

| Điểm mạnh | Chi tiết | So sánh |
|-----------|----------|---------|
| Protobuf-first | Type-safe API→DB, auto-gen Go/OpenAPI/TS, breaking change detection | Hầu hết boilerplate dùng hand-written REST |
| Real infra testing | Testcontainers (PG/Redis/RabbitMQ) thay vì mock | Đa số boilerplate chỉ có unit test + mock |
| Event-driven sẵn | Watermill + RabbitMQ + OTel trace propagation | Thường phải tự setup hoàn toàn |
| Security-first | Argon2id hashing, JWT blacklist (Redis), RBAC, security headers, sensitive log redaction | Hầu hết boilerplate bỏ qua security |
| Module scaffold 19 files | Proto→Migration→Queries→Domain→App→Adapters→Tests trong 1 command | Typical scaffold chỉ 3-5 files |
| Distributed tracing | OpenTelemetry traces across HTTP→RabbitMQ→consumer | Hiếm boilerplate có sẵn |

## Gaps cần bổ sung

| Priority | Item | Chi tiết | Effort |
|----------|------|----------|--------|
| **P0** | `/readyz` dependency check | Hiện tại static `{status: ready}`. Cần ping DB pool, Redis, RabbitMQ trước khi report ready | 2-3h |
| **P1** | CI pipeline | Không có `.github/` hay `.gitlab-ci.yml` trong repo. Lefthook chỉ chạy local | 4-8h |
| **P2** | Service version injection | Hardcoded `0.1.0` trong OTel. Nên inject từ build flags (`-ldflags`) | 1h |
| **P2** | Swagger serving production | Hiện chỉ serve non-prod. Frontend team cần OpenAPI spec access | 2h |
| **P3** | ES usage | Elasticsearch trong docker-compose nhưng không có client code trong `internal/` | Remove hoặc implement |

## So sánh với mong đợi ban đầu

| Khác biệt | Mong đợi | Thực tế | Đánh giá |
|-----------|----------|---------|----------|
| Response format | `{data, error, meta}` | Protobuf JSON/binary | **Tốt hơn** — type-safe, versioned, no manual wrapper |
| Architecture | handler→service→repository | handler→app handler→repository (hexagonal) | **Tốt hơn** — interface ports, compile-time enforcement |
| Dev commands | Makefile | Taskfile | **Tốt hơn** — cross-platform, YAML, dependency graph |
| Validation | Single validator | 3-layer (proto→interceptor→domain) | **Tốt hơn** — defense in depth |
| Testing | Template tests | Real infra + fixtures + gomock + scaffold | **Tốt hơn** — comprehensive |
| Observability | /health + /metrics | /healthz + /readyz + OTel traces+metrics + SigNoz | **Tốt hơn** — nhưng /readyz cần fix |

## Kết luận

Boilerplate **production-grade**, vượt tiêu chuẩn thông thường ở 7/20 tiêu chí. Gap nghiêm trọng duy nhất: `/readyz` không check dependency health. CI pipeline cũng nên có (dù lefthook cover local). ES trong compose mà chưa dùng nên remove để tránh confusion.
