# Boilerplate Audit Report — gnha-services

**Date:** 2026-03-05 | **Type:** Codebase Review | **Stack:** Go 1.26, Echo, Connect RPC, PostgreSQL, Redis, RabbitMQ

---

## Scorecard (20 Criteria)

| # | Criteria | Score | Status |
|---|----------|-------|--------|
| 1 | Architecture | 9/10 | PASS |
| 2 | Dependency Injection | 10/10 | PASS |
| 3 | Config Management | 9/10 | PASS |
| 4 | Logging | 8/10 | PASS |
| 5 | Error Handling | 9/10 | PASS |
| 6 | Database Layer | 10/10 | PASS |
| 7 | Migration | 9/10 | PASS |
| 8 | Authentication | 9/10 | PASS |
| 9 | Middleware | 10/10 | PASS |
| 10 | API Documentation | 8/10 | PASS |
| 11 | Testing | 4/10 | WARN |
| 12 | Docker | 9/10 | PASS |
| 13 | CI/CD | 9/10 | PASS |
| 14 | Lint | 9/10 | PASS |
| 15 | Security | 8/10 | PASS |
| 16 | Validation | 9/10 | PASS |
| 17 | Observability | 8/10 | PASS |
| 18 | CLI Tools | 10/10 | PASS |
| 19 | Scalable Structure | 10/10 | PASS |
| 20 | Documentation | 9/10 | PASS |

**Overall: 176/200 (88%) — Strong boilerplate, 1 critical gap (testing)**

---

## Detailed Assessment

### 1. Architecture — 9/10 PASS

**Structure:**
```
cmd/server/          # Entry point
cmd/seed/            # DB seeder
internal/modules/    # Domain modules (hexagonal)
  user/domain/       # Entities, interfaces, errors
  user/app/          # Command/query handlers
  user/adapters/     # Postgres + gRPC implementations
internal/shared/     # Cross-cutting infra
proto/               # Protobuf definitions
gen/                 # Generated code (sqlc, proto, openapi)
db/migrations/       # Goose migrations
db/queries/          # sqlc SQL files
deploy/              # Docker compose + Traefik
docs/                # Architecture docs
```

Clean hexagonal architecture. Business logic fully separated from framework. Modular monolith with extractable modules.

**-1:** Missing `/pkg` for shared libraries if this grows into multi-service. Not critical for monolith stage.

---

### 2. Dependency Injection — 10/10 PASS

Uber `fx` v1.24.0 — production-grade DI:
- Each module self-registers via `fx.Module()`
- Interface binding: `fx.Annotate(NewPgUserRepository, fx.As(new(domain.UserRepository)))`
- Lifecycle management: `OnStart`/`OnStop` hooks for graceful startup/shutdown
- Constructor injection throughout — no global state

Nothing to improve here.

---

### 3. Config Management — 9/10 PASS

`caarlos0/env` v11 with struct tags:
- Required validation: `env:"DATABASE_URL,required"`
- Defaults: `env:"APP_ENV" envDefault:"development"`
- Duration parsing: `env:"JWT_ACCESS_TTL" envDefault:"15m"`
- Slice parsing: `env:"CORS_ORIGINS" envSeparator:","`
- Runtime validation (JWT secret min 32 chars)
- `.env.example` provided

**-1:** No config file support (YAML/TOML). Env-only is fine for containers but limits local dev flexibility. Minor.

---

### 4. Logging — 8/10 PASS

Go stdlib `slog` — good choice:
- JSON handler (prod) vs text handler (dev) based on `APP_ENV`
- Structured attributes: `slog.Info("event", "topic", topic, "id", id)`
- Request logging middleware sanitizes auth headers
- Configurable log level

**-2:** No log correlation with trace IDs in slog fields (OTel traces exist but not injected into log context automatically). No log rotation config.

---

### 5. Error Handling — 9/10 PASS

Custom `DomainError` type with:
- Error codes: `CodeNotFound`, `CodeAlreadyExists`, `CodeUnauthenticated`, etc.
- HTTP status mapping via `HTTPStatus()` method
- Error wrapping: `errors.Wrap(code, msg, cause)`
- Sentinel errors: `ErrNotFound`, `ErrForbidden`
- Centralized error handler middleware converts domain errors → HTTP responses
- PostgreSQL constraint violations mapped to domain errors

**-1:** No error code documentation/registry exposed to API consumers. `docs/error-codes.md` exists but OpenAPI specs don't include error schemas.

---

### 6. Database Layer — 10/10 PASS

Excellent setup:
- **sqlc**: Type-safe SQL code generation — no ORM overhead, compile-time safety
- **pgx/v5**: Connection pooling (5-25 conns), health checks, retry logic (10 attempts, exponential backoff)
- **Repository pattern**: Interface in domain, implementation in adapters
- **Keyset pagination**: Base64-encoded cursor (timestamp + ID) — efficient for large datasets
- **Soft delete**: Built into user model
- **Constraint handling**: Unique violation → `ErrAlreadyExists`

Nothing to improve.

---

### 7. Migration — 9/10 PASS

Goose migrations in `db/migrations/`:
- `00001_initial_schema.sql` — users, audit_logs with indexes
- `00002_auth_tables.sql` — refresh_tokens, api_keys with indexes
- Taskfile commands: `migrate:up`, `migrate:down`, `migrate:status`, `migrate:create`

**-1:** No seed migrations or fixture data in migration pipeline (seeder is separate cmd, fine but could be more integrated).

---

### 8. Authentication — 9/10 PASS

Solid auth system:
- JWT access tokens (HS256, configurable TTL, custom claims with UserID/Role/Permissions)
- Refresh token rotation (cryptographic random, family-based revocation)
- API key authentication (per-user, prefix-based, permission scoping)
- Password hashing via `PasswordHasher` interface
- Token blacklisting via Redis
- Context injection: `WithUser(ctx, claims)` / `UserFromContext(ctx)`
- RBAC middleware with wildcard permissions (`admin:*`)

**-1:** No OAuth2/OIDC provider integration. JWT uses HS256 (shared secret) — RS256 (asymmetric) would be better for multi-service scenarios.

---

### 9. Middleware — 10/10 PASS

Complete 10-layer chain with correct ordering:
1. Recovery → 2. Request ID → 3. Request Logger → 4. Body Limit (10MB)
5. Gzip → 6. Security Headers → 7. CORS → 8. Context Timeout (30s)
9. Rate Limiting (Redis, 100 req/min) → 10. Auth + RBAC (route-level)

Plus: centralized error handler, Swagger UI (dev only). Each middleware in its own file.

Nothing to improve.

---

### 10. API Documentation — 8/10 PASS

- OpenAPI v2 specs auto-generated from protobuf via `buf generate`
- Swagger UI mounted at `/swagger/` (dev/staging only, disabled in prod)
- Proto files serve as source of truth with validation annotations

**-2:** OpenAPI v2 (not v3). Swagger specs generated but may lack examples/descriptions beyond proto comments. No Postman collection or API testing tool integration.

---

### 11. Testing — 4/10 WARN !!!

**Critical gap.** Infrastructure exists, zero tests written:
- testcontainers-go setup for Postgres/Redis/RabbitMQ
- `testutil/` package with helpers: `db.go`, `fixtures.go`, `rabbitmq.go`, `redis.go`
- Taskfile commands: `test`, `test:integration`, `test:coverage`
- CI pipeline stages defined for unit + integration tests

**But:** 0 `*_test.go` files in the entire codebase.

**Impact:** The boilerplate cannot demonstrate testing patterns. DI setup (fx) makes testing easy, but without example tests, adopters won't know how to test command handlers, repositories, or gRPC handlers.

**Recommendation:** Add at minimum:
- 1 unit test for a command handler (mock repository)
- 1 integration test for repository (testcontainers)
- 1 handler test for gRPC endpoint

---

### 12. Docker — 9/10 PASS

- Multi-stage Dockerfile: `golang:1.26-alpine` → `alpine:3.19`
- Health checks on `:8080`
- `docker-compose.dev.yml`: Postgres, Redis, RabbitMQ (management UI), Elasticsearch, MailHog
- `docker-compose.yml`: Production with Traefik, 2 replicas, health checks, volume persistence

**-1:** No `.dockerignore` review (may include unnecessary files). No Docker build cache optimization hints.

---

### 13. CI/CD — 9/10 PASS

GitLab CI with 5 stages:
1. **quality** — lint, buf breaking change detection
2. **test** — unit + integration (race detector, coverage)
3. **build** — Docker image build
4. **deploy:staging** — auto-deploy on main
5. **deploy:production** — manual trigger

Coverage reports generated. MR and main branch triggers.

**-1:** No security scanning stage (SAST/dependency audit). No artifact caching optimization visible.

---

### 14. Lint — 9/10 PASS

`.golangci.yml` with 11 linters:
- errcheck, gosimple, govet, ineffassign, staticcheck, unused
- gocritic (diagnostic + style + performance)
- misspell, revive, unconvert, unparam
- Excludes `gen/`, `tmp/`, `vendor/`
- 5min timeout
- `.lefthook.yml` for pre-commit hooks

**-1:** No `gosec` (security linter) enabled. Would catch common security issues.

---

### 15. Security — 8/10 PASS

Present:
- Security headers: HSTS, CSP, X-Frame-Options, X-Content-Type-Options
- Rate limiting: Redis-backed sliding window (per-IP/per-user)
- Input validation: protovalidate at API boundary
- JWT token blacklisting via Redis
- Password hashing (bcrypt via interface)
- Auth header sanitization in logs
- Body limit (10MB)
- Context timeout (30s)

**-2:** No `gosec` linter. No CORS origin validation beyond config. No request payload signing. No audit of SQL injection vectors (sqlc mitigates but no explicit mention). No secrets management (plain env vars).

---

### 16. Validation — 9/10 PASS

Two-layer validation:
1. **API boundary**: `buf/validate` annotations in proto files — email format, UUID, string length, enum values, number ranges. Applied via Connect interceptor.
2. **Domain layer**: `domain.NewUser()` validates business rules (non-empty email/name, valid role)

**-1:** No custom validation error messages in proto annotations. Default messages may not be user-friendly.

---

### 17. Observability — 8/10 PASS

OpenTelemetry setup:
- **Tracing**: OTLP gRPC exporter → SigNoz, resource attributes (service name, version, env), TraceContext + Baggage propagation
- **Metrics**: OTLP periodic reader, service metadata
- Event publishing propagates trace context

**-2:** No health check metrics. No custom business metrics (e.g., auth failures, event processing latency). Trace-log correlation not implemented. No alerting rules defined.

---

### 18. CLI Tools — 10/10 PASS

`Taskfile.yml` with 20+ commands:
- `dev:setup` — full local environment bootstrap
- `generate` — buf + sqlc code generation
- `test`, `test:integration`, `test:coverage`
- `migrate:up/down/status/create`
- `seed` — database seeder
- `docker:build/up/down`
- `lint`, `fmt`
- `.air.toml` — hot reload (watches .go, .sql, .proto)

Nothing to improve.

---

### 19. Scalable Structure — 10/10 PASS

- Module pattern: each module self-contained (domain/app/adapters/module.go)
- `docs/adding-a-module.md` — step-by-step guide for new modules
- Fx DI allows modules to be added/removed without touching core
- Event bus decouples modules (audit + notification are pure subscribers)
- Designed as modular monolith → extractable to microservices
- Keyset pagination for efficient large dataset handling

Nothing to improve.

---

### 20. Documentation — 9/10 PASS

5 docs in `docs/`:
- `architecture.md` — module structure, request flow, event flow, design decisions
- `code-standards.md` — naming, patterns, DDD principles
- `adding-a-module.md` — step-by-step module creation guide
- `error-codes.md` — domain error mappings
- `project-changelog.md` — change tracking

`README.md` — quick start, stack, architecture overview, deployment.

**-1:** No API usage examples / getting started guide for API consumers. No ADR (Architecture Decision Records) beyond inline notes.

---

## Priority Fixes

### Critical (Must Fix)
1. **Write example tests** — At least 1 unit test (handler), 1 integration test (repository), 1 gRPC handler test. The testing infrastructure is ready, just needs examples.

### Important (Should Fix)
2. **Enable `gosec` linter** — Add to `.golangci.yml` for security static analysis
3. **Trace-log correlation** — Inject trace ID into slog fields via middleware
4. **Add SAST stage to CI** — `go-sec` or GitLab SAST template

### Nice to Have
5. Upgrade OpenAPI v2 → v3 (when buf supports it natively)
6. Add custom business metrics (auth failure count, event processing latency)
7. ADR (Architecture Decision Records) directory
8. OAuth2/OIDC provider support for SSO scenarios
9. Custom validation error messages in proto annotations

---

## Summary

This is a **strong, production-grade Go boilerplate** scoring 88%. Architecture, DI, database layer, middleware, CLI tools, and scalable structure are all excellent (10/10). The **only critical gap is testing** — infrastructure is ready but zero tests exist. Fix that and enable `gosec`, and this boilerplate is ready for production use.
