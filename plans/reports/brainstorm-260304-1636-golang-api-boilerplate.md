# Brainstorm: Go API Boilerplate — Production-Ready Modular Monolith

**Date:** 2026-03-04
**Status:** Agreed
**Participants:** namnguyen + Claude

---

## Problem Statement

Need production-ready Go boilerplate for API projects with:
- Good DX, easy to maintain/develop/debug
- Full-featured tooling ("đầy đủ đồ chơi")
- Modular monolith architecture
- REST + gRPC + Event-driven support
- Previous experience: gin/echo/go-kratos — pain points: too much boilerplate, hard to debug, no conventions

---

## Final Stack Decision

| Layer | Tool | Version | Rationale |
|-------|------|---------|-----------|
| **Language** | Go | 1.26 | Latest stable (Feb 2026), Green Tea GC, `errors.AsType`, `slog.NewMultiHandler` |
| **HTTP** | Echo | v4 | Familiar from gin/echo, solid middleware, net/http compatible |
| **gRPC** | Connect RPC | v1.19+ | gRPC + JSON on same port, net/http native, cURL-friendly |
| **Events** | Watermill | v1.5+ | 12 backends, CQRS built-in, swap broker easily |
| **Message Broker** | RabbitMQ | 3.x | Mature, flexible routing |
| **Database** | PostgreSQL + sqlc + pgx/v5 | 16 / latest | Write SQL, get type-safe Go. Zero runtime magic |
| **Migrations** | goose | v3 | Simple, widely used |
| **Cache** | go-redis | v9 | Standard Redis client |
| **Search** | Elasticsearch | 8.x | Full-text search, sync via events |
| **DI** | Uber Fx | latest | Lifecycle hooks, module = bounded context |
| **Config** | caarlos0/env | v11 | 12-factor, ENV vars only |
| **Logging** | log/slog (stdlib) | Go 1.26 | `NewMultiHandler`, structured, zero deps |
| **Observability** | OpenTelemetry SDK | latest | Traces + metrics + logs — app-side instrumentation |
| **Monitoring** | SigNoz (self-hosted) | latest | All-in-one: traces + logs + metrics + dashboards + alerts. OTel native, ClickHouse storage |
| **Proto tooling** | buf CLI | latest | Replaces protoc, linting, breaking change detection |
| **API docs** | OpenAPI from protobuf | grpc-gateway plugin | Auto-gen from .proto |
| **Validation** | protovalidate | latest | Validation rules in .proto |
| **Hot reload** | air | latest | Standard Go hot reload |
| **Linting** | golangci-lint | latest | Industry standard |
| **Testing** | testify + testcontainers | latest | Real DB/Redis/RabbitMQ in tests |
| **Git hooks** | lefthook | latest | Pre-commit lint, pre-push test |
| **Task runner** | Taskfile | v3 | YAML, cross-platform, dependency-aware |
| **CI/CD** | GitLab CI/CD | - | `.gitlab-ci.yml`, Container Registry |
| **Reverse proxy** | Traefik | v3 | Auto SSL, Docker-native |
| **Cron** | robfig/cron | v3 | + Redis distributed lock |

---

## Architecture: Simplified Hexagonal (Ports & Adapters)

### Project Structure

```
cmd/
  server/main.go              # Entrypoint (Fx app)
  seed/main.go                # Database seeder

internal/
  modules/
    <module>/                  # One dir per bounded context
      domain/                  # Entities, value objects, errors, repository interfaces (PORTS)
      app/                     # Command/query handlers (use cases)
      adapters/
        postgres/              # sqlc implementation (ADAPTER)
        redis/                 # Cache adapter
        http/                  # Echo handlers (non-RPC routes)
        grpc/                  # Connect RPC handlers
      module.go                # Uber Fx module definition
  shared/
    middleware/                # Echo + Connect interceptors
    config/                    # App config struct
    database/                  # DB connection, retry, pool
    observability/             # OTel setup, slog setup, metrics
    errors/                    # Domain error types, error codes registry
    model/                     # BaseModel (ID, timestamps, soft delete)

proto/
  <module>/v1/*.proto          # Protobuf definitions per module

db/
  queries/*.sql                # sqlc SQL files
  migrations/*.sql             # goose migrations

gen/
  proto/                       # buf generated Go code
  sqlc/                        # sqlc generated Go code
  openapi/                     # OpenAPI spec from proto
  ts/                          # TypeScript client (connect-es)

deploy/
  docker-compose.yml           # Production (app + infra)
  docker-compose.dev.yml       # Dev infra only
  docker-compose.monitor.yml   # SigNoz (self-hosted, OTel native)

README.md                      # Onboarding guide, architecture overview, conventions
.gitlab-ci.yml
Dockerfile
Taskfile.yml
.air.toml
.lefthook.yml
.env.example
buf.yaml
buf.gen.yaml
sqlc.yaml
.golangci.yml
```

### Architecture Principles

1. **Dependency direction:** Always inward (adapters → app → domain)
2. **Module isolation:** Module A never imports Module B's internals. Cross-module = Watermill events
3. **Protobuf-first:** API contract defined in .proto, Go code generated
4. **Repository per aggregate:** Interface in domain/, implementation in adapters/
5. **Closure-based transactions:** No `*sql.Tx` in interfaces
6. **Go packages enforce boundaries:** `internal/` = compile-time encapsulation

---

## Patterns & Conventions

### Error Handling
- gRPC-style error codes for both REST and RPC (InvalidArgument, NotFound, PermissionDenied, etc.)
- Domain errors in `domain/` package with `ErrorCode`
- `errors.AsType[*DomainError](err)` (Go 1.26) for type-safe assertion
- Adapter layer translates infra errors → domain errors
- Centralized Echo error handler + Connect error interceptor

### API Design
- Protobuf-first: define in .proto → auto-gen REST + gRPC + OpenAPI
- URL versioning via proto package: `order.v1`
- Cursor-based pagination (base64 encoded `(sort_val, id)`)
- Flat JSON responses, no envelope
- Idempotency keys via Redis middleware for POST/PATCH
- protovalidate for request validation

### Security
- JWT access token (15min) + refresh token (7d, HTTP-only cookie, Redis-backed)
- **Password hashing:** argon2id (preferred) or bcrypt for user passwords
- **API Key management:** For external/public API consumers (hashed in DB, prefix for identification)
- RBAC: roles/permissions in DB, cached in JWT claims
- Rate limiting: Redis sliding window, per-user + per-IP + per-API-key
- Request body limit: 10MB default
- CORS: explicit origins only
- Connect-Protocol-Version header in allowed CORS headers
- **Security headers middleware:** Strict-Transport-Security, X-Content-Type-Options, X-Frame-Options, X-XSS-Protection
- **Log sanitization:** Redact passwords, tokens, PII from request/response logs
- **Retry + exponential backoff:** For external service calls (ES, email, webhooks)

### Middleware Ordering (CRITICAL)
```
1. Recovery (panic → 500, not crash)
2. Request ID (generate/propagate)
3. Request Logger (method, path, latency, status)
4. Body Limit (10MB)
5. Gzip Compression
6. CORS
7. Global Timeout (30s)
8. Rate Limiting
9. OpenTelemetry
10. Auth (JWT) — route group level
11. RBAC — route group level
```

### Soft Delete
- `deleted_at *time.Time` in BaseModel
- Partial index: `CREATE INDEX ... WHERE deleted_at IS NULL`
- sqlc queries default filter `AND deleted_at IS NULL`

### Audit Trail
- Event-driven: service → publish AuditEvent → Watermill subscriber → `audit_logs` table
- Fields: entity_type, entity_id, action, actor_id, changes (JSONB), ip_address

### Testing
- Table-driven unit tests
- testcontainers for integration (real Postgres/Redis/RabbitMQ)
- E2E API tests: full request → response flow via Connect httptest
- Watermill GoChannel for in-memory event testing
- Manual test doubles over mocking frameworks
- Golden files for API response stability

### CQRS (via Watermill)
- CommandBus for write operations with side effects
- EventBus for publishing domain events
- Subscribers handle: notifications, audit, cache invalidation, ES sync
- Skip Event Sourcing unless audit trail is business requirement

---

## Logging & Monitoring

### Logging
- `log/slog` with `NewMultiHandler` (Go 1.26)
- Dev: TextHandler (human-readable) | Prod: JSONHandler + OTLP
- 3 tiers: request log (auto), application log (developer), audit log (security)
- Request ID + user ID + trace ID auto-injected via context

### Monitoring: SigNoz (Self-Hosted)
```
App → OTel SDK → OTLP (gRPC :4317) → SigNoz → ClickHouse
```
- **Replaces:** Prometheus + Jaeger + Loki + Grafana + AlertManager (5 tools → 1)
- OpenTelemetry native — zero vendor lock-in
- Auto-correlates traces ↔ logs ↔ metrics (click trace → see related logs)
- Built-in alerting (threshold-based across all signals)
- Self-hosted via Docker Compose
- Go app unchanged: same OTel SDK, just point OTLP endpoint to SigNoz
- SigNoz UI: dashboards, trace explorer, log search, metrics, alerts

### Health Checks
- `/healthz` — liveness (no deps, is process alive?)
- `/readyz` — readiness (check DB, Redis, RabbitMQ)

---

## Dev Workflow

### Code Gen Pipeline
```
task generate
  ├── task generate:proto   → buf lint + buf breaking + buf generate
  └── task generate:sqlc    → sqlc generate
```

### Development Flow
```
task dev:setup     → install tools, start infra, migrate, seed
task dev           → air (hot reload)
task generate      → all code gen
task lint          → golangci-lint
task test          → unit tests
task test:integration → integration tests (testcontainers)
task check         → lint + test
```

### Git Hooks (lefthook)
- Pre-commit: golangci-lint --fix, generated code staleness check
- Pre-push: unit tests

### Conventional Commits
```
feat(order): add bulk creation endpoint
fix(auth): refresh token rotation
refactor(user): extract validation to domain
```

---

## DevOps

### Docker
- Multi-stage build: golang:1.26-alpine → alpine:3.19
- ~15MB final image

### GitLab CI/CD Pipeline
```
quality:  lint + generated-check (MR only)
test:     unit-test + integration-test (services: postgres, redis, rabbitmq)
build:    docker build + push to GitLab Container Registry
deploy:   staging (auto on main) + production (manual on tag)
```

### Zero-Downtime Deploy
- Docker Compose `deploy.update_config`: start-first, 1 at a time
- Traefik health-check based routing
- Graceful shutdown via Fx lifecycle (reverse order)

### Graceful Startup (Fx Order)
```
Config → Postgres (retry) → Redis (retry) → RabbitMQ (retry) → ES →
Migrations → Cache warm → Watermill subscribers → Cron → Echo server →
Health = ready
```

### Infrastructure (Docker Compose)
- `docker-compose.dev.yml` — Postgres, Redis, RabbitMQ, ES, MailHog
- `docker-compose.monitor.yml` — SigNoz (self-hosted, replaces Prometheus+Jaeger+Loki+Grafana+AlertManager)
- `docker-compose.yml` — Production (app + Traefik + infra)

---

## Background Jobs & Notifications

### Background Jobs
- `robfig/cron` v3 for scheduled tasks
- Redis distributed lock for multi-instance safety
- Event-driven jobs via Watermill subscribers

### Notifications
- Email adapter (SMTP default, swap to SendGrid/SES)
- Event-driven: domain event → Watermill subscriber → NotificationService
- `html/template` for email templates
- MailHog in dev for email testing

---

## Evaluated & Rejected Alternatives

| Alternative | Reason Rejected |
|-------------|----------------|
| go-kratos | Microservices primitives are noise for monolith |
| go-zero | DSL lock-in, no HTTP+gRPC hybrid |
| Fiber | Fasthttp breaks net/http ecosystem, can't serve gRPC |
| go-kit | Maintenance mode since 2023, high boilerplate |
| GORM | Reflection overhead, N+1 risks, auto-migrate dangerous |
| ent | Long compile times, magical queries, code bloat |
| Google Wire | Less suited for modular monolith (no lifecycle hooks) |
| Manual DI | Unmaintainable past ~15 components |
| Kafka | Ops-heavy for typical API projects, RabbitMQ sufficient |
| NATS | Less mature routing than RabbitMQ |
| swaggo/swag | Replaced by protobuf-first (auto-gen OpenAPI from .proto) |

---

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Connect RPC smaller community (3.8k stars) | buf.build actively maintained, growing adoption |
| Watermill learning curve | Good docs, Wild Workouts example |
| Protobuf-first overhead for simple CRUD | Echo still available for non-RPC endpoints |
| Monitoring stack complexity | Separate docker-compose.monitor.yml, opt-in |
| Uber Fx runtime DI debugging | `fx.Supply` for explicit wiring, good error messages |

---

## Success Criteria

1. New developer productive in <1 day (dev:setup → running API)
2. Adding new module: copy structure, register Fx module, <30 min
3. All generated code via `task generate`, no manual proto/SQL code
4. 80%+ test coverage achievable with testcontainers
5. Zero-downtime deploy working from day 1
6. Request traceability: request_id → logs + traces + metrics correlated
7. Breaking API changes caught in CI (`buf breaking`)

---

## Unresolved Questions

1. **Echo v5 timeline:** Echo v5 is in development. Should boilerplate target v4 (stable, security patches until Dec 2026) or v5? Recommend v4 for now.
2. **Connect + Echo same port:** Best pattern for mounting Connect RPC handlers alongside Echo routes? `echo.WrapHandler()` or separate mux?
3. **Watermill outbox pattern:** For atomic DB write + event publish, use Watermill's built-in outbox (PostgreSQL) or manual publish-after-commit?
4. **Elasticsearch sync strategy:** Watermill events vs PostgreSQL CDC (Debezium)? Events = simpler, CDC = more reliable. Start with events.
5. **sqlc emit_interface:** Use sqlc's `emit_interface` option for testability or wrap in manual repository interfaces? Recommend manual repository wrapping for clean domain boundary.

---

## Research Reports

- [Framework Research](./researcher-260304-1217-golang-boilerplate-research.md) — Framework comparison, ORM, DI, events
- [Architecture Patterns](./researcher-260304-1437-golang-architecture-patterns.md) — Patterns, principles, best practices

---

## Next Steps

→ Create detailed implementation plan with phases
