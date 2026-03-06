# Boilerplate Codebase Review - Consolidated Report

**Date:** 2026-03-06 | **Reviewers:** 6 parallel agents | **Codebase:** gnha-services (Go gRPC boilerplate)

---

## Overall Score: 8.1 / 10

| # | Review Area | Score | Report |
|---|------------|-------|--------|
| 1 | Architecture & Structure | 8.3/10 | [architecture-project-structure-review](code-reviewer-260306-1447-architecture-project-structure-review.md) |
| 2 | Security | 8.0/10 | [security-audit](code-reviewer-260306-1447-security-audit.md) |
| 3 | Database & Data Layer | 8.0/10 | [database-data-layer-review](code-reviewer-260306-1447-database-data-layer-review.md) |
| 4 | gRPC/Connect API Layer | 8.5/10 | [grpc-connect-api-layer](code-reviewer-260306-1447-grpc-connect-api-layer.md) |
| 5 | Observability & DevOps | 7.5/10 | [observability-devops-infra](code-reviewer-260306-1447-observability-devops-infra.md) |
| 6 | Event System & Tests | 7.5/10 | [event-testing-quality](code-reviewer-260306-1447-event-testing-quality.md) |

---

## Strengths (What's Done Well)

### Architecture
- **Textbook hexagonal layering** -- domain has zero infra imports, clean separation domain/app/adapters
- **Consistent Fx DI wiring** with one `fx.Module` per package, 6 separate `OnStop` hooks for graceful shutdown
- **Domain entity encapsulation** -- unexported fields, validated constructors, `Reconstitute()` pattern
- **Event-driven cross-module communication** with OTel trace propagation through AMQP

### Security
- **Argon2id + `subtle.ConstantTimeCompare`** -- best-in-class password hashing
- **JWT algorithm pinning** via `SigningMethodHMAC` type assertion (prevents alg-none attacks)
- **sqlc parameterized queries** -- zero SQL injection surface
- **protovalidate interceptor** at RPC boundary (email, UUID, enum, length constraints)
- **Centralized HTTP error handler** strips internals from 500 responses

### Database
- Clean domain/adapter boundary, correct `FOR UPDATE` transactional pattern
- Proper **keyset pagination** with LIMIT+1 trick
- Constraint-violation-to-domain-error mapping

### API
- Well-structured proto with versioning (`user/v1/`)
- Clean mapper layer between proto and domain
- Connect-RPC dual protocol (gRPC + gRPC-Web + HTTP)

### DevOps
- Structured slog logging with JSON/text toggle, trace_id + request_id correlation
- Multi-stage Docker build with stripped binary
- Comprehensive Taskfile with 20+ tasks
- Lefthook pre-commit lint + pre-push test gates

---

## Critical & High Priority Issues

### CRITICAL (0) -- None found

### HIGH (12 total across all reviews)

| ID | Area | Issue | File | Fix Effort |
|----|------|-------|------|-----------|
| H-1 | Security | Rate limiter **fails open** when Redis unavailable -- unlimited throughput during outages | `middleware/rate_limit.go` | 15 min |
| H-2 | Security/DevOps | Dockerfile runtime runs as **root** -- container escape = host compromise | `Dockerfile` | 5 min |
| H-3 | Security | Password policy only `min_len: 8`, no max length -- long passwords can **DoS Argon2id** | `auth/password.go` | 10 min |
| H-4 | API | Internal error leakage -- `mapper.go:32` passes raw Go errors (DB strings) to Connect clients | `adapters/grpc/mapper.go` | 10 min |
| H-5 | Database | `SELECT *` carries **password hashes** through entire call chain for ListUsers/GetByID | `db/queries/user.sql` | 15 min |
| H-6 | Architecture | Audit module's `fx.Provide(*sqlcgen.Queries)` **will collide** if another module provides same type | `audit/module.go` | 10 min |
| H-7 | Database | No `CHECK` constraint on `role` column at database level | `db/migrations/00001_initial_schema.sql` | 5 min |
| H-8 | DevOps | CI coverage artifact mismatch -- produces `coverage.out` but declares `coverage.xml` | `.gitlab-ci.yml` | 5 min |
| H-9 | DevOps | Missing `deploy/docker-compose.monitor.yml` -- OTel exporters send to nothing | Referenced in Taskfile | 30 min |
| H-10 | DevOps | Traefik `${ACME_EMAIL}` not interpolated -- YAML doesn't support env vars | `deploy/traefik/traefik.yml` | 10 min |
| H-11 | Tests | `create_user_test.go` missing error-path tests -- only 2 of ~6 needed cases exist | `app/create_user_test.go` | 30 min |
| H-12 | Database | Missing composite index `(created_at DESC, id DESC) WHERE deleted_at IS NULL` for pagination | `db/migrations/` | 5 min |

---

## Medium Priority Issues (Top 10)

| ID | Area | Issue | Impact |
|----|------|-------|--------|
| M-1 | Security | Token blacklist silently discards Redis errors -- logged-out tokens accepted during outages | Fail-open security |
| M-2 | Security | JWT tokens lack `iss`/`aud` claims -- dangerous with multi-service secret sharing | Token confusion |
| M-3 | API | Interceptor ordering: `validate` before `RBACInterceptor` -- unauthorized users see schema | Info leakage |
| M-4 | API | Fragile `strings.HasPrefix(method, "Create")` RBAC matching | Silent auth bypass |
| M-5 | Architecture | App layer imports `shared/middleware` for `GetClientIP` -- breaks hexagonal layering | Coupling |
| M-6 | Database | Hardcoded connection pool settings instead of config-driven | Ops inflexibility |
| M-7 | Database | Invalid cursors silently reset to page 1 instead of error | Data duplication |
| M-8 | Events | No dead-letter queue -- poison messages requeue infinitely at AMQP level | Resource leak |
| M-9 | Events | `json.Marshal` errors silently discarded in audit subscriber (3 occurrences) | Silent data loss |
| M-10 | DevOps | No `otelecho.Middleware()` registered -- HTTP spans never created | Observability gap |

---

## Test Coverage Assessment

| Layer | Coverage | Status |
|-------|----------|--------|
| Domain (user entity) | ~80% | Good |
| App (use cases) | ~30% | Weak -- only happy path + 1 error |
| Repository (postgres) | ~60% | Missing Update, GetByEmail, edge cases |
| Subscribers (audit/notification) | 0% | Critical gap |
| Cron | 0% | Not testable yet |
| **Overall estimate** | **~40-50%** | **Needs work** |

---

## Quick Wins (< 2 hours total)

1. **Dockerfile non-root user** (5 min) -- add `RUN adduser` + `USER`
2. **Fix mapper error leakage** (10 min) -- log raw error, return generic "internal error"
3. **Add `CHECK` constraint on role** (5 min) -- `CHECK (role IN ('admin','member'))`
4. **Fix CI coverage artifact** (5 min) -- match format between test output and artifact declaration
5. **Add pagination index** (5 min) -- `CREATE INDEX CONCURRENTLY`
6. **Add `fx.Private` to audit module** (10 min) -- prevent Fx provider collision
7. **Explicit column list in ListUsers/GetByID** (15 min) -- exclude password_hash from SELECT
8. **Rate limiter fail-closed** (15 min) -- return 503 when Redis unavailable
9. **Max password length** (10 min) -- cap at 72 bytes (Argon2id limit)
10. **Swap interceptor order** (5 min) -- RBAC before validate

---

## Architecture Diagram

```
cmd/server/main.go
    |
    v
[Fx Container]
    |
    +-- internal/shared/
    |   +-- config/         (env-based config)
    |   +-- database/       (postgres + redis)
    |   +-- auth/           (jwt, password, context)
    |   +-- middleware/      (chain, auth, rbac, rate-limit, security, recovery)
    |   +-- observability/  (logger, tracer, metrics)
    |   +-- events/         (RabbitMQ bus + subscribers)
    |   +-- cron/           (scheduler)
    |   +-- errors/         (domain errors)
    |   +-- testutil/       (testcontainers helpers)
    |   +-- mocks/          (generated mocks)
    |
    +-- internal/modules/
    |   +-- user/
    |   |   +-- domain/     (entity, errors, repository interface)
    |   |   +-- app/        (use cases: create, get, list, update, delete)
    |   |   +-- adapters/
    |   |       +-- grpc/   (Connect-RPC handlers, mapper, routes)
    |   |       +-- postgres/ (repository impl)
    |   +-- audit/          (event subscriber -> DB)
    |   +-- notification/   (event subscriber -> SMTP)
    |
    +-- gen/                (sqlc + protobuf generated)
    +-- proto/              (proto definitions)
    +-- db/                 (migrations + queries)
    +-- deploy/             (docker-compose + traefik)
```

---

## Verdict

**GNHA-Services is a well-crafted Go boilerplate** scoring 8.1/10. The hexagonal architecture, Fx DI, domain encapsulation, and event-driven patterns are production-grade foundations. Security fundamentals (Argon2id, JWT pinning, parameterized queries, protovalidate) are strong.

**Main gaps:**
- Test coverage (~40-50%) needs doubling, especially subscriber and error-path tests
- Several fail-open patterns (rate limiter, token blacklist) that should fail-closed
- DevOps has missing files (monitor compose) and config issues (Traefik env vars, CI artifacts)
- Password hash leaking through `SELECT *` in read queries

**Recommendation:** Address the 10 quick wins (~2h) before using this boilerplate in production. The architecture is solid enough to scale to 10+ modules without refactoring.

---

## Unresolved Questions

1. Should event payloads stay in `shared/events` (simpler) or move to domain packages (purer DDD)?
2. No login/token-issuance endpoint -- intentional for boilerplate scope?
3. `EventBus` is concrete; should app handlers use an `EventPublisher` interface for testability?
4. Should the outbox pattern be implemented now or deferred?
5. If Connect serves gRPC-Web browser clients, CORS `AllowHeaders` may need `Grpc-Status` and `Grpc-Message`
6. Should notification subscriber dead-letter after 3 SMTP failures or retry indefinitely?

---

## Individual Reports

- [Architecture & Structure Review](code-reviewer-260306-1447-architecture-project-structure-review.md)
- [Security Audit](code-reviewer-260306-1447-security-audit.md)
- [Database & Data Layer Review](code-reviewer-260306-1447-database-data-layer-review.md)
- [gRPC/Connect API Layer Review](code-reviewer-260306-1447-grpc-connect-api-layer.md)
- [Observability & DevOps Review](code-reviewer-260306-1447-observability-devops-infra.md)
- [Event System & Testing Review](code-reviewer-260306-1447-event-testing-quality.md)
