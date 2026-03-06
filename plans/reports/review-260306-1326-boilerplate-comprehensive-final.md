# Final Comprehensive Review -- GNHA Services Boilerplate

**Date:** 2026-03-06
**Reviewer:** code-reviewer
**Scope:** ALL hand-written Go files (58 files, ~3,510 LOC)
**Build:** `go vet ./...` clean, zero `MustParse`, zero `panic()`, zero `TODO/FIXME`, zero `SanitizeHeader` dead code

---

## Category Scores

| Category | Score | Notes |
|----------|-------|-------|
| Architecture | 9/10 | Clean hex arch, proper dependency direction, Fx wiring correct |
| Security | 8/10 | Strong auth/hashing/headers. RBAC granularity gap remains (known) |
| Production Readiness | 8.5/10 | Graceful shutdown, health probes, retry logic, OTel. One `os.Exit` in goroutine |
| Code Quality | 8.5/10 | DRY, consistent naming, no dead code. Two files slightly over 200 lines |
| DX (Developer Experience) | 9/10 | Scaffold generator, Taskfile, README, CI/CD, seed, hot reload |
| Testing Infrastructure | 7.5/10 | Testcontainers, mocks, fixtures present. Unit test coverage thin |
| **Overall** | **8.5/10** |  |

---

## Issues Table (NEW issues only -- not previously reported/fixed)

### None Critical

### High Priority

None found. All 10 previously-identified critical/high fixes verified present.

### Medium Priority

| # | File | Issue | Impact |
|---|------|-------|--------|
| M-1 | `cmd/server/main.go:78` | `os.Exit(1)` inside goroutine bypasses Fx graceful shutdown | Skips OnStop hooks (DB close, AMQP close, OTel flush). Process exits uncleanly. Replace with channel-based signal or `slog.Error` only and let Fx handle lifecycle. |
| M-2 | `grpc/routes.go:23` | All user endpoints gated on `PermUserRead` only | Create/Update/Delete should require `PermUserWrite` or `PermUserDelete`. Comment says "checked in handler via Connect interceptors" but no such interceptors exist. |
| M-3 | `audit/module.go:12-13` | Audit module creates its own `sqlcgen.Queries` instance | This is a second `sqlcgen.New(pool)` besides whatever other modules use. Not a bug per se, but creates an implicit dependency and makes it harder to trace DB usage. Acceptable for boilerplate. |
| M-4 | `events/topics.go:17` | `UserCreatedEvent` includes PII (email, name) in audit payload | Stored verbatim in audit_logs.changes JSONB column. GDPR consideration if deployed in EU. |
| M-5 | `shared/module.go:33` | `_ = mp.Shutdown(ctx)` silently discards MeterProvider shutdown error | Should log at warn level like Redis close does. |
| M-6 | `create_user_test.go` | Missing error-path tests | No tests for: invalid role, hasher failure, repo.Create failure. Only happy path + email-taken tested. |

### Minor Priority

| # | File | Issue | Impact |
|---|------|-------|--------|
| L-1 | `repository.go` | 222 lines (guideline: 200) | Slightly over. Cursor helpers could extract to a `cursor.go` file. |
| L-2 | `cmd/scaffold/main.go` | 214 lines | Acceptable for a CLI tool, but `toTitle`/`readGoModule`/`validateIdentifier` could be a `scaffoldutil` package. |
| L-3 | `cron/scheduler.go:63` | `s.cron.Stop()` does not wait for running jobs | `cron.Stop()` returns a context; should use `<-s.cron.Stop().Done()` or the returned channel to wait for in-flight jobs. |
| L-4 | `rate_limit.go` | Rate limiter is global (100 req/min) including health probes | `/healthz` and `/readyz` hit the rate limiter. Should be excluded or rate limit applied only to API groups. |
| L-5 | `notification/subscriber.go:49-56` | Welcome email template uses `html/template` but body is passed as raw string to SMTP | Template is safe (auto-escapes), but the `body` string from template execution is inserted into SMTP message body without Content-Transfer-Encoding. Works but could cause issues with long lines. |

---

## What's Done Well

1. **Hex arch discipline** -- Domain has zero infrastructure imports. Adapters depend inward only. No import cycles.
2. **Domain entity encapsulation** -- Unexported fields, getters, `Reconstitute()` for persistence. Mutation methods validate.
3. **Error architecture** -- `DomainError` with error codes mapped to both HTTP status and Connect RPC codes. Centralized error handler.
4. **Transactional updates** -- `Update(ctx, id, func(*User) error)` pattern with `SELECT FOR UPDATE` inside a transaction.
5. **Keyset pagination** -- Base64-encoded cursor with `(created_at, id)` keyset. `limit+1` trick for hasMore detection.
6. **Security headers** -- CSP absent (fine for API-only), but HSTS, X-Frame-Options, X-Content-Type-Options all present.
7. **Argon2id with constant-time compare** -- Best-practice password hashing. Parameters are reasonable (64MB memory, 3 iterations).
8. **JWT validation** -- Signing method check prevents algorithm confusion. Token blacklist via Redis for logout.
9. **OTel trace propagation** -- Traces propagated into Watermill messages via metadata. WithInsecure guarded by IsDevelopment().
10. **Scaffold generator** -- Full module scaffolding with 19 files, validation, conflict detection, Go reserved word check.
11. **CI/CD pipeline** -- Lint, generated-code drift check, unit/integration tests, Docker build, staging/production deploy with manual gate.
12. **Retry logic** -- Both Postgres and Redis connections retry up to 10 times with linear backoff.
13. **Readiness probe** -- `/readyz` checks both DB and Redis. `/healthz` is unconditional liveness.
14. **Graceful shutdown** -- Fx lifecycle hooks close DB pool, Redis client, AMQP publisher/subscriber, OTel providers, Echo server, Watermill router, cron scheduler.
15. **Testcontainers** -- Integration tests use real Postgres, Redis, RabbitMQ containers. Migrations run automatically.
16. **Request logging** -- Structured with `request_id`, `trace_id`, `user_id`. Log level by status code (info/warn/error).

---

## Verdict

**Ship-ready as a boilerplate. Not yet production-ready for regulated environments.**

The codebase is clean, well-organized, and follows Go best practices for a modular monolith. After 5+ review cycles, all critical and high-severity issues have been resolved. The remaining issues are medium/minor and acceptable for a boilerplate:

- **M-1** (`os.Exit` in goroutine) is the most concerning remaining issue but only fires on catastrophic Echo bind failure, which is unlikely post-startup.
- **M-2** (RBAC granularity) is a known gap -- the middleware infrastructure supports it (`PermUserWrite`/`PermUserDelete` constants exist), it just needs to be wired per-endpoint.
- **M-4** (PII in audit) is a design decision that should be documented, not a bug.

**For production deployment**, address M-1 and M-2. Everything else is polish.

**Comparison to industry Go boilerplates** (go-blueprint, go-clean-arch, wild-workouts): This boilerplate is more complete in DX (scaffold generator, full CI/CD, seed command, testcontainers) and observability (OTel + SigNoz) than most open-source Go boilerplates. The event-driven audit/notification modules with RabbitMQ are a differentiator. Missing compared to some: no login/refresh endpoint, no rate-limit per-route granularity, no OpenAPI auto-generation from proto.

---

## File Inventory

| Area | Files | LOC |
|------|-------|-----|
| `cmd/server/` | 1 | 88 |
| `cmd/scaffold/` | 1 | 214 |
| `cmd/seed/` | 1 | 86 |
| `internal/shared/` | 24 | ~1,250 |
| `internal/modules/user/` | 13 | ~1,000 |
| `internal/modules/audit/` | 2 | 142 |
| `internal/modules/notification/` | 4 | 125 |
| `internal/shared/testutil/` | 5 | 205 |
| `internal/shared/mocks/` | 1 | 131 |
| **Total** | **58** | **~3,510** |

---

## Unresolved Questions

1. Is PII in audit trail (M-4) an intentional design decision? If GDPR applies, event payloads should reference user IDs only, not emails/names.
2. Should the rate limiter exclude health probe endpoints? Current setup counts `/healthz` and `/readyz` against the 100 req/min limit.
3. Is there a plan to add a login/refresh endpoint? Without it, the JWT+blacklist infrastructure cannot be tested end-to-end.
