# Boilerplate DX Master Review

Date: 2026-03-06 | Reviewer: code-reviewer (master-level)

## Assessment Table

| # | Tiêu chí | Tham khảo (generic) | Hiện trạng GNHA | Verdict | Nhận định |
|---|----------|---------------------|------------------|---------|-----------|
| 1 | Convention over Config | Structure/naming cố định | fx DI, module pattern (domain/app/adapters), env struct tags, snake_case Go, kebab-case files | **STRONG** | Dev mới chỉ cần copy pattern. fx.Module enforce dependency direction — convention embedded in code, không cần doc |
| 2 | Module Template / CLI | `make module name=X` | `task module:create name=X` — scaffold 19 files (proto, migration, sqlc, domain, app, adapters, tests, module.go) | **EXCELLENT** | Vượt mong đợi. 19 files vs typical 5-8. Có input validation, plural support, atomic creation (rollback on failure) |
| 3 | Structure rõ ràng | cmd/ internal/ pkg/ config/ migrations/ | `cmd/` (server, scaffold, seed), `internal/modules/`, `internal/shared/`, `db/`, `proto/`, `gen/`, `deploy/`, `docs/` | **STRONG** | Chuẩn Go project layout. Không dùng `pkg/` (đúng — `pkg/` là anti-pattern cho internal projects). `gen/` tách riêng generated code |
| 4 | Enforced Architecture | handler → service → repository | handler → app handler → domain → repository (hexagonal). fx DI enforce interface boundaries | **EXCELLENT** | Hexagonal > 3-layer. Domain encapsulation thực sự (private fields, constructor validation). Import rules enforced by Go packages |
| 5 | Standard Error System | Response lỗi thống nhất | `DomainError{Code, Message, Err}` + 8 ErrorCode enum + HTTP mapping + sentinel errors + centralized ErrorHandler middleware | **STRONG** | Nhất quán. Code→HTTP mapping rõ ràng. Sentinel errors cho common cases. ErrorHandler middleware xử lý tập trung |
| 6 | Standard Response Format | `{data, error, meta}` | Protobuf JSON (Connect RPC). Error: `{code, message}`. Pagination: cursor in proto message | **DIFFERS** | Protobuf approach **tốt hơn** reference. Type-safe, versioned, auto-generated. `{data,error,meta}` wrapper là REST pattern — không cần với RPC |
| 7 | Middleware sẵn | logger, request-id, recovery, auth, cors, rate-limit | 11 middleware: recovery, request-id, logger, body-limit, gzip, security-headers, CORS, timeout, rate-limit, error-handler, auth+RBAC | **EXCELLENT** | Vượt reference 5 items. Thêm: body-limit (10MB), gzip (level 5), security-headers (HSTS/CSP), context-timeout (30s), RBAC. Rate-limit dùng Redis sliding window — production-grade |
| 8 | Logging chuẩn | Structured logging | `log/slog` stdlib. JSON (prod) / text (dev). Configurable level. Context-aware. Request duration tracking | **STRONG** | slog là stdlib — zero dependency. Sensitive field redaction. Structured fields propagate qua context |
| 9 | Validation | Validator chuẩn | 3 tầng: buf.validate (proto rules) → domain constructors (business rules) → repository (uniqueness constraints) | **EXCELLENT** | Vượt reference. Proto validation auto-generate — dev chỉ cần declare rules trong .proto. Domain constructor validate business logic. DB enforce uniqueness |
| 10 | Testing pattern | Có template test | Unit: gomock + testify. Integration: testcontainers (real Postgres/Redis/RabbitMQ). Scaffold generates test boilerplate. Race detection enabled | **STRONG** | Real infra testing > mocks. Scaffold auto-gen test files. Thiếu: load testing setup, benchmark tests |
| 11 | Dev commands | make dev/test/lint/migrate | `task` (Taskfile.yml): 27 commands — dev, test, lint, migrate:*, build, check, dev:setup, generate, module:create, seed, monitor, clean | **EXCELLENT** | Task > Make (cross-platform, YAML, dotenv support). 27 commands vs reference 4. `task dev:setup` = one-command bootstrap (tools + infra + migrate + seed) |
| 12 | Local development | Docker compose stack | docker-compose.dev.yml: Postgres 16, Redis 7, RabbitMQ 3 (management UI), ES 8, MailHog. Hot-reload via Air (.air.toml) | **EXCELLENT** | 5 services với health checks. Air hot-reload watches .go/.sql/.proto. MailHog cho email testing. `task dev:setup` = zero-config start |
| 13 | Code generation | make module, make migration | `task generate` = buf (proto → Go + OpenAPI) + sqlc (SQL → Go) + mockgen. `task module:create` scaffold 19 files. CI verify generated code freshness | **EXCELLENT** | Triple codegen (proto + sqlc + mocks). CI check prevents stale generated code. Scaffold + codegen = complete automation |
| 14 | Documentation | README, ARCHITECTURE, HOW_TO_ADD | README, architecture.md (diagrams), code-standards.md (633 lines), adding-a-module.md (manual + scaffold), error-codes.md, project-changelog.md | **STRONG** | code-standards.md rất chi tiết. adding-a-module.md có cả manual + scaffold path. Thiếu: API docs cho frontend team (Swagger serving có nhưng chưa có descriptions) |
| 15 | Example module | internal/example | `internal/modules/user/` (full CRUD, 5 use cases) + `audit/` (event subscriber) + `notification/` (email subscriber) | **EXCELLENT** | 3 module examples vs reference 1. User = CRUD pattern. Audit = event subscriber pattern. Notification = async processing pattern. Cover 3 architectural patterns |
| 16 | Guardrails | lint, pre-commit, CI | lefthook (pre-commit: lint+fix+restage; pre-push: test). golangci-lint (11 linters). GitLab CI (lint → generated-check → test → build → deploy) | **STRONG** | Generated code freshness check trong CI là bonus lớn. `buf breaking` detect breaking proto changes. 11 linters vs typical 3-5 |
| 17 | Performance defaults | connection pool, timeout | PG: 25/5 conns, 1h lifetime, 30min idle. Redis: sliding window rate-limit. HTTP: 30s timeout, 10MB body, gzip level 5. RabbitMQ: 3 retries | **STRONG** | Tuned defaults. Rate-limit fail-open (Redis down → allow request). Connection pool sizes reasonable cho mid-scale |
| 18 | Observability | /health, /metrics | `/healthz` + `/readyz`, OpenTelemetry (traces + metrics), OTLP gRPC exporter, SigNoz integration, `task monitor:up`, Docker HEALTHCHECK | **EXCELLENT** | Vượt xa reference. Distributed tracing + metrics vs chỉ health check. SigNoz UI sẵn. Event trace propagation qua Watermill messages |
| 19 | API versioning | /api/v1/ | Proto package versioning (`user.v1`). `buf breaking` detect breaking changes. Connect RPC path versioning | **DIFFERS** | Schema-level versioning > URL path versioning. Proto versioning = type-safe, auto-enforced. `buf breaking` = CI-enforced backward compatibility |
| 20 | Scalable module design | internal/user, internal/order, internal/payment | fx.Module độc lập. Event-driven (Watermill/RabbitMQ). Zero coupling giữa modules. Thêm module = folder + fx.Module register | **STRONG** | Modular monolith ready to split. Event-driven = async workflows sẵn. fx.Module = explicit dependency declaration |

## Score Summary

| Metric | Count |
|--------|-------|
| EXCELLENT (vượt mong đợi) | 9/20 (#2, #4, #7, #9, #11, #12, #13, #15, #18) |
| STRONG (đạt chuẩn) | 9/20 (#1, #3, #5, #8, #10, #14, #16, #17, #20) |
| DIFFERS (khác approach, hợp lý hơn) | 2/20 (#6, #19) |
| PARTIAL / FAIL | 0/20 |

**Overall: 20/20 đạt hoặc vượt. 0 gaps critical.**

## So sánh với Review trước (2026-03-05)

| Thay đổi | Trước | Sau |
|----------|-------|------|
| Module scaffold | **Partial** (thiếu CLI) | **EXCELLENT** (19 files, validation, atomic) |
| Code generation | **Partial** | **EXCELLENT** (triple codegen + CI verify) |
| Overall Pass+ | 3/20 | 9/20 |
| Gaps critical | 1 (scaffold) | 0 |

## Điểm mạnh nổi bật (so với reference boilerplate)

1. **Protobuf-first stack**: Type-safe từ API → DB. Auto-gen Go code, OpenAPI, validation. Breaking change detection trong CI
2. **Real infra testing**: Testcontainers thay vì mocks — test trên Postgres/Redis/RabbitMQ thật
3. **Event-driven sẵn**: Watermill + RabbitMQ. Trace propagation qua messages. Audit + Notification subscribers as examples
4. **3-layer validation**: Proto rules (declarative) → Domain constructors (business) → DB constraints (uniqueness)
5. **Security-first defaults**: HSTS, CSP, rate-limit (Redis sliding window), JWT blacklist, bcrypt cost 12, RBAC
6. **Module scaffold**: 19 files trong <10s. Vượt xa typical `make module` (5-8 files)
7. **Observability stack**: OpenTelemetry traces + metrics → SigNoz. Không chỉ health check
8. **CI guardrails**: Generated code freshness check + proto breaking change detection

## Gaps còn lại (non-critical)

| Priority | Item | Effort | Impact |
|----------|------|--------|--------|
| P2 | OpenAPI/Swagger descriptions cho frontend team | 1 ngày | Frontend DX |
| P3 | OTLP TLS cho production (hiện dùng WithInsecure) | Vài giờ | Security |
| P3 | Per-endpoint rate limiting (hiện chỉ global 100 req/min) | 1 ngày | Fine-grained control |
| P3 | Load/benchmark test template | 1 ngày | Performance validation |
| P4 | Custom business metrics (domain-specific counters) | Vài giờ | Business observability |

## Kết luận

Boilerplate **production-grade, vượt chuẩn DX reference**. 9/20 tiêu chí vượt mong đợi. Không còn gap critical nào sau khi implement module scaffold. Approach khác reference ở 2 điểm (proto response format, schema-level versioning) — cả 2 đều **tốt hơn** reference.

Dev mới onboard: `task dev:setup` → đọc `adding-a-module.md` → `task module:create name=X` → customize → `task generate` → `task test` → done.
