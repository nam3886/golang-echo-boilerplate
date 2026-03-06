# Boilerplate Master Review — GNHA Services

Date: 2026-03-05 | Reviewer: Claude (Master-level DX Assessment)

## Assessment Table

| # | Tiêu chi | Tham khao | Thuc te GNHA | Verdict | Nhan dinh |
|---|----------|-----------|--------------|---------|-----------|
| 1 | Convention over Config | Dev khong can quyet dinh nhieu ve structure/naming | fx DI, module pattern co dinh (domain/app/adapters), env struct tags (`caarlos0/env`), typed IDs (`UserID string`), verb-first setters (`ChangeName`), Go naming enforced via golangci-lint | **STRONG** | Vuot mong doi. Convention doc hoa trong `code-standards.md` (633L). Dev moi chi can doc 1 file la biet naming + structure. |
| 2 | Module Template / CLI | `make module name=user` | `task module:create name=X plural=Y` — generates 19 files (proto, migration, sqlc, domain, app, adapters, tests, module.go). Auto-runs `task generate` after scaffold. Safety: validates identifier, checks no overwrite | **EXCELLENT** | Vuot xa "make module". Co validation, plural support, auto-codegen. Gap cu (#2 Partial) da close hoan toan. |
| 3 | Structure ro rang | cmd/, internal/, pkg/, config/, migrations/ | `cmd/{server,scaffold,seed}/`, `internal/{modules,shared}/`, `db/{migrations,queries}/`, `proto/`, `gen/{proto,sqlc,openapi}/`, `deploy/`, `docs/` | **STRONG** | Chuan Go project layout. Khac biet: dung `internal/shared/` thay `pkg/` (strict encapsulation), tach `proto/` va `gen/` rieng — tot hon tham khao. |
| 4 | Enforced Architecture | handler -> service -> repository | handler -> app handler -> repository (via interface ports). fx DI enforce dependency direction. Domain layer zero imports from infra | **STRONG** | Hexagonal dung chuan. Interface ports o domain/, adapters implement. fx.Module enforce wiring. Strict hon "handler->service->repo" — co domain layer trung gian. |
| 5 | Standard Error System | Response loi thong nhat | `DomainError{Code, Message, Err}` + 8 ErrorCode enum + HTTPStatus() mapping + codeToConnect mapping + per-module sentinels + centralized ErrorHandler middleware | **EXCELLENT** | Dual mapping (HTTP + Connect RPC). Per-module sentinels (`ErrEmailTaken`, `ErrUserNotFound`). Wrappable via `errors.As`. Production-grade error chain. |
| 6 | Standard Response Format | `{data, error, meta}` wrapper | Protobuf JSON (Connect RPC). Success: `{user: {...}}` or `{items: [...], nextCursor, hasMore}`. Error: `{code, message}` | **DIFFERS (tot hon)** | Khong dung REST wrapper `{data,error,meta}` — dung Connect RPC protobuf serialization. Type-safe, versioned, auto-generated. Trade-off: frontend can proto-aware client. |
| 7 | Middleware san | logger, request-id, recovery, auth, cors, rate-limit (6) | 10 middleware: recovery, request-id, logger, body-limit(10MB), gzip(L5), security-headers(6 headers), CORS, timeout(30s), rate-limit(Redis sliding window 100/min), error-handler. Auth+RBAC at route level | **EXCELLENT** | Vuot 67% so voi tham khao (10 vs 6). Bonus: gzip, security-headers (HSTS, CSP, X-Frame), body-limit, context-timeout. Rate-limit dung Redis sliding window (production-grade). |
| 8 | Logging chuan | Structured logging | `log/slog` stdlib. JSON(prod)/text+source(dev). Sensitive redaction (Authorization, Cookie, Set-Cookie). Level configurable via env. Request log: method, path, status, latency_ms, bytes, ip, request_id | **STRONG** | Zero dependency logging (stdlib slog). Redaction la diem cong lon — nhieu boilerplate bo qua. Watermill cung dung slog. |
| 9 | Validation | Dung validator chuan | 3 tang: (1) Protobuf rules (`buf.validate`) auto-gen — email, min/max len, enum in, uuid. (2) Domain constructors (`NewUser` validates). (3) DB constraints (unique violation -> domain error) | **EXCELLENT** | 3-layer validation vuot xa "dung validator chuan". Proto validation = zero boilerplate, auto-gen. Domain validation = business rules. DB = last line of defense. |
| 10 | Testing pattern | Co template test | 3-tier: domain unit (stdlib), app unit (gomock), integration (testcontainers real PG/Redis/RabbitMQ). `//go:generate mockgen` directives. Test fixtures. Build tag `integration`. Coverage report task | **EXCELLENT** | Khong mock DB — test tren real Postgres via testcontainers. Race detection enabled. Scaffold auto-gen test boilerplate. Fixtures cho common entities. |
| 11 | Dev commands | make dev/test/lint/migrate | Taskfile: `dev`, `dev:setup`, `dev:tools`, `test`, `test:integration`, `test:coverage`, `lint`, `check`, `build`, `migrate:{up,down,status,create}`, `seed`, `generate`, `module:create`, `docker:build`, `monitor:{up,down}`, `clean` | **EXCELLENT** | 20+ tasks vs 4 "make" commands. Taskfile > Makefile (cross-platform, YAML, deps). `dev:setup` = one-command full environment. `monitor:up` cho SigNoz la bonus. |
| 12 | Local development | Docker compose chay toan bo stack | docker-compose.dev.yml: Postgres 16, Redis 7, RabbitMQ 3 (management), Elasticsearch 8, MailHog. All with healthchecks. Hot-reload via Air. `task dev:setup` = install tools + start infra + migrate + seed | **EXCELLENT** | 5 services vs "toan bo stack". MailHog cho email testing. ES cho search (du chua co client). Production compose co Traefik + replicas. |
| 13 | Code generation | make module / make migration | `task generate` = buf (proto -> Go + Connect + OpenAPI) + sqlc (SQL -> Go) + mockgen. `task module:create` = scaffold 19 files + auto generate. `task migrate:create` = goose migration. Buf breaking change detection | **EXCELLENT** | Triple codegen (proto + sql + mocks) + module scaffold. Breaking change detection la dac biet — bao ve API contract. OpenAPI auto-gen tu proto. |
| 14 | Documentation | README, ARCHITECTURE, HOW_TO_ADD_FEATURE | `architecture.md` (system design + request/event flow), `code-standards.md` (633L — naming, patterns, testing), `adding-a-module.md` (scaffold + manual reference), `error-codes.md`, `project-changelog.md` | **STRONG** | 5 docs, `code-standards.md` rat chi tiet. Gap: thieu `development-roadmap.md` va `system-architecture.md` (referenced in rules nhung chua tao). |
| 15 | Example module | internal/example | `internal/modules/user/` — full CRUD (5 use cases), hexagonal layers, events, pagination. `audit/` — event subscriber pattern. `notification/` — email subscriber pattern | **EXCELLENT** | 3 example modules thay vi 1. User = CRUD reference. Audit = event consumer. Notification = async side-effect. Developers co 3 patterns de follow. |
| 16 | Guardrails | lint, pre-commit, CI check | lefthook: pre-commit (golangci-lint --fix + buf/sqlc staleness check), pre-push (go test -race). golangci-lint: 11 linters (errcheck, govet, gocritic, revive...). `task check` = lint + test | **STRONG** | Pre-commit auto-fix + gen staleness check la bonus. Gap: **khong co CI pipeline** (`.github/workflows/` khong ton tai). Chi enforce local via lefthook. |
| 17 | Performance defaults | connection pool, timeout | PG: 25 max/5 min conns, 1h lifetime. Redis: 10*NumCPU pool. HTTP: 30s timeout, 10MB body. Rate-limit: 100/min Redis sliding window. Gzip L5. JWT: 15m access/7d refresh. Argon2id: 64MB mem. Cursor pagination (O(1) any depth). Distributed cron lock (Redis SETNX + Lua) | **EXCELLENT** | Vuot xa "connection pool, timeout". Cursor pagination, distributed cron lock, Argon2id tuned, exponential retry — production-tuned. |
| 18 | Observability | /health, /metrics | `/healthz` (liveness) + `/readyz` (readiness). OpenTelemetry traces + metrics via OTLP. SigNoz integration. Trace propagation qua RabbitMQ messages. `task monitor:up` = SigNoz stack | **STRONG** | Distributed tracing across async events la diem manh. Gap: `/readyz` tra static response — khong check DB/Redis/RabbitMQ health. |
| 19 | API versioning | /api/v1/ (URL path) | Proto package versioning (`user.v1`), Connect RPC path = `/user.v1.UserService/...`. `buf breaking` detect breaking changes against main branch | **STRONG** | Schema-level versioning > URL path versioning. Breaking change detection automated. Trade-off: khong co URL path versioning — frontend dung Connect client. |
| 20 | Scalable module design | internal/user, internal/order, internal/payment (folder-based) | fx.Module doc lap, zero coupling. Event-driven (Watermill/RabbitMQ). Durable queues, retry middleware. Module scaffold dam bao consistency. 3 modules demonstrate: CRUD, event consumer, async side-effect | **EXCELLENT** | Modular monolith san sang tach microservice. Event-driven = loose coupling. fx.Module = plug-and-play. Scaffold = consistency guarantee. |

## Score Summary

| Verdict | Count | Items |
|---------|-------|-------|
| EXCELLENT (vuot mong doi) | 11 | #2, #5, #7, #9, #10, #11, #12, #13, #15, #17, #20 |
| STRONG (dat chuan) | 7 | #1, #3, #4, #8, #14, #16, #18 |
| STRONG (dat, co gap nho) | 1 | #19 |
| DIFFERS (khac nhung tot hon) | 1 | #6 |
| FAIL | 0 | — |

**Overall: 20/20 criteria met. 11 vuot mong doi.**

## Gaps Con Lai

| Priority | Gap | Impact | Effort |
|----------|-----|--------|--------|
| **P1** | Khong co CI/CD pipeline (`.github/workflows/` missing) | Guardrails chi enforce local. PR khong co automated check | 2-4h |
| **P2** | `/readyz` tra static — khong check DB/Redis/RabbitMQ | K8s readiness probe vo nghia neu service up nhung DB down | 1-2h |
| **P2** | Auth module chua implement (JWT utils co, endpoints chua) | Login/logout/refresh chua dung duoc | 1-2 ngay |
| **P3** | `error-codes.md` outdated (6 codes vs 8 trong code) | Doc khong match implementation | 30 phut |
| **P3** | `development-roadmap.md` + `system-architecture.md` chua tao | Referenced trong rules nhung chua co | 2-3h |
| **P3** | Elasticsearch client chua implement (service chay trong Docker) | Infra co nhung chua dung | Khi can |

## So Sanh Voi Tham Khao

| Khia canh | Tham khao (generic) | GNHA (thuc te) | Nhan dinh |
|-----------|---------------------|----------------|-----------|
| Architecture | handler->service->repo | Hexagonal + domain layer + interface ports | Strict hon, enforce tot hon |
| API | REST `{data,error,meta}` | Connect RPC + Protobuf | Type-safe, versioned, nhung can proto-aware client |
| Validation | 1 validator layer | 3 layers (proto + domain + DB) | Defense in depth |
| Testing | "co template test" | Real infra (testcontainers) + mocks + fixtures | Production-grade testing |
| Error handling | "response loi thong nhat" | Typed errors + dual mapping (HTTP + gRPC) + module sentinels | Enterprise-grade |
| Commands | 4 make commands | 20+ Taskfile tasks | Comprehensive DX |
| Observability | /health, /metrics | Health + OTel traces + metrics + SigNoz + event trace propagation | Distributed tracing across async flows |
| Security | (not mentioned) | Argon2id, JWT blacklist, RBAC, security headers, rate-limit, CRLF protection, sensitive redaction | Security-first design |

## Ket Luan

Boilerplate **production-grade, vuot muc tham khao o 11/20 tieu chi**. Architecture thuc su enforce (khong chi convention). Security-first (Argon2id, JWT blacklist, RBAC, header hardening). Testing tren real infra.

**Gap duy nhat nghiem trong: thieu CI/CD pipeline.** Moi thu khac hoac dat hoac vuot.
