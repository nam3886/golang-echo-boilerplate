# Boilerplate DX Assessment Report

Date: 2026-03-05

## Review Summary

| # | Tiêu chí | Status | Hiện trạng | Nhận định |
|---|----------|--------|------------|-----------|
| 1 | Convention over Config | **Pass** | fx DI, module pattern cố định (domain/app/adapters), config via env struct tags, snake_case enforced | Dev mới follow structure là đủ |
| 2 | Module Template / CLI | **Partial** | Có `task generate:proto` + `task generate:sqlc`, có doc `adding-a-module.md` | **Thiếu `task module:create name=X`** scaffold tự động |
| 3 | Structure rõ ràng | **Pass** | `cmd/`, `internal/modules/`, `internal/shared/`, `db/migrations/`, `deploy/`, `gen/`, `proto/` | Chuẩn Go project layout |
| 4 | Enforced Architecture | **Pass** | handler → app handler → repository qua interface ports. fx DI enforce dependency direction | Hexagonal đúng chuẩn |
| 5 | Standard Error System | **Pass** | `DomainError{Code, Message, Err}` + code→HTTP mapping + sentinels + centralized ErrorHandler | Nhất quán, dễ extend |
| 6 | Standard Response Format | **Differs** | Protobuf JSON thay vì `{data, error, meta}`. Error: `{code, message}`. Pagination: cursor in message | Protobuf approach tốt hơn — type-safe, versioned, generated |
| 7 | Middleware sẵn | **Pass+** | 10 middleware: recovery, request-id, logger, body-limit, gzip, security-headers, CORS, timeout, rate-limit, error-handler | Vượt mong đợi: thêm gzip, security-headers, body-limit |
| 8 | Logging chuẩn | **Pass** | `log/slog` structured, JSON(prod)/text(dev), sensitive redaction, context-aware, configurable level | Production-ready. Redaction là điểm cộng |
| 9 | Validation | **Pass** | 3 tầng: protobuf rules (buf.validate) → domain constructors → repository uniqueness | Proto validation auto-generate, giảm boilerplate |
| 10 | Testing pattern | **Pass+** | Testcontainers (real Postgres/Redis/RabbitMQ), race detection, integration tags, coverage | Không mock — test trên real infra |
| 11 | Dev commands | **Pass** | Taskfile.yml: `task dev/test/lint/migrate:*/build/check/dev:setup` | Task > Make (cross-platform, YAML readable) |
| 12 | Local development | **Pass** | docker-compose.dev.yml: Postgres 16, Redis 7, RabbitMQ 3, ES 8, MailHog. Hot-reload via Air | `task dev:setup` = one-command setup |
| 13 | Code generation | **Partial** | `task generate` = buf (proto) + sqlc. Auto-generate Go, OpenAPI, Connect RPC | **Thiếu module scaffold generator** (same as #2) |
| 14 | Documentation | **Pass** | README, architecture.md, adding-a-module.md, code-standards.md, error-codes.md, changelog | `adding-a-module.md` tốt cho onboarding |
| 15 | Example module | **Pass** | `internal/modules/user/` hoàn chỉnh: domain, app (5 use cases), adapters (postgres+grpc), module.go | Reference implementation. Có thêm audit + notification |
| 16 | Guardrails | **Pass** | lefthook (pre-commit: lint; pre-push: test), golangci-lint, GitLab CI (lint→test→build) | Buf breaking change detection là bonus |
| 17 | Performance defaults | **Pass** | PG: 25/5 conns, 1h lifetime. Redis: 10×NumCPU. HTTP: 30s timeout, 10MB body. RabbitMQ: 3 retries | Tuned cho production, retry with backoff |
| 18 | Observability | **Pass+** | `/healthz` + `/readyz`, OpenTelemetry traces+metrics, OTLP exporter, SigNoz, Docker HEALTHCHECK | Vượt mong đợi: distributed tracing |
| 19 | API versioning | **Pass** | Proto package versioning (`user.v1`), `buf breaking` detect breaking changes | Schema-level versioning > URL path versioning |
| 20 | Scalable module design | **Pass** | fx.Module độc lập, event-driven (Watermill/RabbitMQ). User/Audit/Notification demonstrate pattern | Thêm module = thêm folder + register fx.Module |

## Score

| Metric | Count |
|--------|-------|
| Pass | 17/20 |
| Pass+ (vượt mong đợi) | 3/20 (#7, #10, #18) |
| Partial | 2/20 (#2, #13) |
| Differs (hợp lý) | 1/20 (#6) |

## Gaps cần bổ sung

| Priority | Item | Effort |
|----------|------|--------|
| **High** | `task module:create name=X` — scaffold domain/app/adapters/module.go + proto + sqlc query template | 1-2 ngày |
| **Low** | Migration template kèm common patterns (soft delete, audit columns) | Vài giờ |

## Điểm mạnh nổi bật

- **Protobuf-first**: type-safe API→DB, auto-gen, breaking change detection
- **Real infra testing**: testcontainers > mocks
- **Event-driven**: Watermill + RabbitMQ sẵn sàng async workflows
- **Distributed tracing**: OpenTelemetry + SigNoz
- **Security defaults**: header hardening, rate-limit, JWT blacklist, sensitive redaction

## Kết luận

Boilerplate **production-grade**. Gap duy nhất đáng kể: thiếu module scaffold generator (`task module:create`).
