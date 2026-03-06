# Boilerplate DX Master Review

Date: 2026-03-05 | Reviewer: Claude (Master-level)

## Assessment Table

| # | Tiêu chí | Tham khảo (generic) | Thực tế GNHA | Verdict | Nhận định |
|---|----------|---------------------|--------------|---------|-----------|
| 1 | Convention over Config | Structure + naming chuẩn, dev follow là đủ | fx DI, module pattern cố định (domain/app/adapters), snake_case, proto-first, sqlc codegen | **STRONG** | Vượt chuẩn. fx enforce dependency direction at compile-time. Dev không cần quyết định gì ngoài business logic |
| 2 | Module Template / CLI | `make module name=X` scaffold | Chỉ có doc `adding-a-module.md` (7 bước manual). Không có CLI/script | **MISSING** | Gap lớn nhất. 7 bước manual = error-prone, slow onboarding. Cần `task module:create name=X` |
| 3 | Structure rõ ràng | cmd/ internal/ pkg/ config/ migrations/ | `cmd/`, `internal/modules/`, `internal/shared/`, `db/migrations/`, `deploy/`, `gen/`, `proto/` | **STRONG+** | Tốt hơn tham khảo. Tách `gen/` (generated code), `proto/` (source of truth), `deploy/` (infra). Không có `pkg/` — đúng vì monolith |
| 4 | Enforced Architecture | handler → service → repository | handler → app handler → repository qua interface ports. fx DI enforce. Compile-time interface check (`var _ Interface = (*Impl)(nil)`) | **STRONG+** | Hexagonal thực sự, không chỉ convention. Compile-time enforcement > runtime. fx `fx.As()` bind interface tại module level |
| 5 | Standard Error System | Response lỗi thống nhất | `DomainError{Code, Message, Err}` + 8 error codes + HTTP mapping + Connect mapping + centralized ErrorHandler middleware | **STRONG** | Dual mapping (HTTP + Connect RPC) là điểm cộng. Sentinel errors cho common cases. Error wrapping preserve cause chain |
| 6 | Standard Response Format | `{data, error, meta}` wrapper | Protobuf typed responses (Connect RPC). Error: `{code, message}`. Pagination: cursor-based trong message | **DIFFERS** | Không theo pattern REST wrapper — tốt hơn. Proto response = type-safe, versioned, auto-generated. REST wrapper là anti-pattern khi dùng RPC |
| 7 | Middleware sẵn | logger, request-id, recovery, auth, cors, rate-limit (6) | 10 middleware: recovery, request-id, logger, body-limit, gzip, security-headers, CORS, timeout, rate-limit, error-handler. Plus: auth, RBAC (per-route) | **EXCELLENT** | 10 global + 2 per-route > 6 tham khảo. Security headers (HSTS, X-Frame-Options, CSP) và gzip compression thường bị bỏ qua ở boilerplate |
| 8 | Logging chuẩn | Structured logging | `log/slog` stdlib, JSON(prod)/text+source(dev), sensitive header redaction, status-based level routing (5xx→Error, 4xx→Warn) | **STRONG** | stdlib slog = zero dependency. Redaction cho authorization/cookie headers. Status-based routing giúp alert chính xác |
| 9 | Validation | Validator chuẩn | 3 tầng: buf.validate (proto) → domain constructors → repository constraints (unique). Connect interceptor auto-validate | **STRONG+** | 3 tầng > 1 tầng validator. Proto validation = declarative, auto-generated, zero boilerplate. Domain constructors catch business rules |
| 10 | Testing pattern | Có template test | Unit: gomock + table-driven. Integration: testcontainers (real Postgres/Redis/RabbitMQ). Race detection. Coverage report. Build tags tách unit/integration | **STRONG** | Real infra testing > mocks. Testcontainers = test exactly what runs in prod. Thiếu: test template cho module mới |
| 11 | Dev commands | make dev/test/lint/migrate | Taskfile: 20+ tasks. `task dev:setup` = one-command bootstrap. `task check` = lint+test. `task monitor:up` = observability stack | **STRONG+** | Taskfile > Makefile (cross-platform, YAML, dependency-aware). `dev:setup` = install tools + start infra + migrate + seed. Zero friction |
| 12 | Local development | Docker compose full stack | docker-compose.dev.yml: Postgres 16, Redis 7, RabbitMQ 3 (management UI), Elasticsearch 8, MailHog. Named volumes, healthchecks | **EXCELLENT** | 5 services > typical 2-3. MailHog cho email testing. RabbitMQ management UI. ES cho future search. Tất cả có healthcheck |
| 13 | Code generation | make module / make migration | `task generate` = buf (proto→Go+Connect+OpenAPI) + sqlc (SQL→Go) + mockgen (interface→mocks). `task migrate:create` | **STRONG** | 3 generators chạy parallel. Thiếu module scaffold (same gap #2). Có OpenAPI generation — bonus cho frontend team |
| 14 | Documentation | README, ARCHITECTURE, HOW_TO_ADD_FEATURE | README, architecture.md, adding-a-module.md (7 bước chi tiết), code-standards.md, error-codes.md, project-changelog.md | **STRONG** | 6 docs > 3 tham khảo. `adding-a-module.md` chi tiết từng bước với code examples. `code-standards.md` enforce conventions |
| 15 | Example module | internal/example (basic) | `internal/modules/user/` full CRUD + audit subscriber + notification subscriber. Domain: entity+repo interface+errors. App: 5 use cases. Adapters: postgres+grpc | **EXCELLENT** | 3 modules (user+audit+notification) demonstrate: CRUD, event publishing, event subscribing, cross-module communication. Reference implementation hoàn chỉnh |
| 16 | Guardrails | lint, pre-commit, CI check | golangci-lint (11 linters), lefthook installed nhưng **thiếu config file**, **không có CI pipeline file** | **PARTIAL** | Linter config tốt (11 linters). Nhưng: lefthook.yml missing = pre-commit hooks không hoạt động. Không tìm thấy CI config (.github/workflows/ hay .gitlab-ci.yml). Gap nghiêm trọng |
| 17 | Performance defaults | connection pool, timeout | PG: 25 max/5 min conns, 1h lifetime, 30m idle. HTTP: 30s timeout, 10MB body. Rate-limit: 100/min. Gzip level 5. JWT: 15m access, 7d refresh | **STRONG** | Tuned cho production. Exponential backoff retry (10 attempts) cho PG connection. Thiếu: Redis pool config explicit, per-endpoint rate-limit override |
| 18 | Observability | /health + /metrics | `/healthz` + `/readyz`, OpenTelemetry (traces+metrics), OTLP exporter, SigNoz stack, Swagger UI (non-prod), Docker HEALTHCHECK | **EXCELLENT** | Vượt xa tham khảo. Distributed tracing + metrics > basic /health. SigNoz = full observability platform. Swagger UI cho API exploration |
| 19 | API versioning | /api/v1/ URL path | Proto package versioning (`user.v1`), buf breaking change detection (FILE mode), Connect RPC path includes version | **STRONG+** | Schema-level versioning > URL path. buf breaking = automated backward-compat check tại generate time. Prevents accidental breaking changes |
| 20 | Scalable module design | internal/user, internal/order... (folder-based) | fx.Module per module, event-driven (Watermill/RabbitMQ), fx group injection cho fan-out. Add module = add folder + register 1 line in main.go | **STRONG** | fx.Module isolation > plain folders. Event-driven sẵn sàng cho async workflows. Group injection cho fan-out subscribers. Modular monolith → microservices path clear |

## Score Summary

| Metric | Count |
|--------|-------|
| EXCELLENT (vượt xa tham khảo) | 4/20 (#7, #12, #15, #18) |
| STRONG+ (tốt hơn tham khảo) | 4/20 (#3, #4, #9, #11, #19) — thực tế 5 |
| STRONG (đạt chuẩn) | 7/20 (#1, #5, #8, #10, #13, #14, #17, #20) — thực tế 8 |
| DIFFERS (khác nhưng hợp lý) | 1/20 (#6) |
| PARTIAL (đạt một phần) | 1/20 (#16) |
| MISSING (thiếu) | 1/20 (#2) |

**Overall: 17/20 đạt hoặc vượt | 1 partial | 1 missing | 1 differs (tốt hơn)**

## Critical Gaps (Priority Order)

| Priority | Gap | Impact | Effort |
|----------|-----|--------|--------|
| **P0** | `task module:create name=X` — scaffold domain/app/adapters/module.go + proto + sqlc queries | Onboarding speed, error reduction | 1-2 ngày |
| **P0** | Guardrails incomplete — lefthook.yml missing, CI pipeline config missing | Code quality gate không enforce | Vài giờ |
| **P1** | Test template cho module mới | Dev phải copy-paste từ user module | Vài giờ |
| **P2** | Redis pool config explicit (pool size, dial timeout, max retries) | Production tuning | 1 giờ |
| **P2** | Service version từ build flags thay vì hardcoded "0.1.0" | Tracing/metrics accuracy | 30 phút |
| **P3** | Per-endpoint rate-limit override | Fine-grained control cho sensitive endpoints | Vài giờ |

## Hidden Issues Found

| Issue | Location | Severity |
|-------|----------|----------|
| lefthook.yml không tồn tại | Project root | **High** — pre-commit hooks installed but no config = silently doing nothing |
| CI pipeline config missing | `.github/workflows/` hoặc `.gitlab-ci.yml` | **High** — no automated quality gate |
| Elasticsearch in docker-compose nhưng không có client code | `internal/shared/` | **Low** — unused resource consuming RAM |
| JWT blacklist có thể thiếu TTL | Redis `blacklist:` key | **Medium** — potential memory leak nếu TTL không set lúc write |
| Hardcoded service version "0.1.0" | `observability/tracer.go`, `metrics.go` | **Low** — traces/metrics không reflect actual version |
| `go.mod` tidy warning | `go.uber.org/mock` should be direct | **Low** — `go mod tidy` cần chạy |
| Compile errors in `create_user_test.go` | `timeNow` undefined, `watermillMsg` undefined | **Medium** — tests sẽ fail |

## So sánh với Tham Khảo Generic

| Aspect | Tham khảo generic | GNHA thực tế | Winner |
|--------|-------------------|--------------|--------|
| API paradigm | REST + JSON wrapper | Connect RPC + Protobuf | **GNHA** — type-safe, versioned, auto-gen |
| Validation | 1 layer (validator lib) | 3 layers (proto+domain+DB) | **GNHA** — defense in depth |
| Architecture enforcement | Convention only | fx DI + compile-time checks | **GNHA** — machine-enforced |
| Testing | Mocks | Real infra (testcontainers) | **GNHA** — test what you ship |
| Observability | /health + /metrics | OTel traces+metrics + SigNoz | **GNHA** — distributed tracing |
| Dev setup | `make dev` multi-step | `task dev:setup` one-command | **GNHA** — zero friction |
| Module scaffold | `make module name=X` | Manual 7-step process | **Tham khảo** — automation > documentation |
| CI/CD guardrails | pre-commit + CI | Linter only (hooks broken) | **Tham khảo** — working pipeline > installed tools |

## Kết luận

Boilerplate **production-grade, vượt chuẩn generic ở hầu hết tiêu chí**. Kiến trúc hexagonal enforce bằng compiler, không chỉ convention. Proto-first approach cho type safety end-to-end.

**2 gaps nghiêm trọng cần fix ngay:**
1. Module scaffold generator — gap DX lớn nhất, ảnh hưởng trực tiếp đến onboarding speed
2. Guardrails thực sự hoạt động — lefthook config + CI pipeline. Linter không có nghĩa gì nếu không ai bắt buộc chạy

**Mọi thứ khác đều ở mức strong hoặc excellent.**
