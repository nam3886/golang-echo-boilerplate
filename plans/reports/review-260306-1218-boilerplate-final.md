# Final Codebase Review -- GNHA Services Go Boilerplate

**Date:** 2026-03-06
**Reviewer:** code-reviewer
**Scope:** All hand-written Go files under `internal/` and `cmd/` (3,513 LOC, 44 files)
**Build status:** `go vet ./...` passes clean

---

## 1. Fixes Verification

| # | Fix Description | File | Status | Notes |
|---|---|---|---|---|
| 1 | RBAC on routes (`RequirePermission(PermUserRead)`) | `internal/modules/user/adapters/grpc/routes.go:23` | **PASS** | Auth + RequirePermission applied to route group |
| 2 | AMQP shutdown (publisher + subscriber closed) | `internal/shared/events/module.go:22-38` | **PASS** | `registerAMQPShutdown` closes both, logs errors |
| 3 | X-Request-ID validation (>128 chars rejected) | `internal/shared/middleware/request_id.go:20` | **PASS** | `len(id) > 128` generates new UUID |
| 4 | Swagger XSS fix (`html.EscapeString`) | `internal/shared/middleware/swagger.go:57-58,66` | **PASS** | Both spec URL and name escaped |
| 5 | Watermill router context (cancellable) | `internal/shared/events/subscriber.go:56-69` | **PASS** | `context.WithCancel` + cancel on OnStop |
| 6 | Cron AddJob returns error | `internal/shared/cron/scheduler.go:32,51` | **PASS** | Returns error from `cron.AddFunc` |
| 7 | OTel WithInsecure guard (dev only) | `internal/shared/observability/tracer.go:23-25`, `metrics.go:23-25` | **PASS** | Conditional on `IsDevelopment()` |
| 8 | DB shutdown hooks (Postgres + Redis) | `internal/shared/module.go:40-53` | **PASS** | `pool.Close()` and `rdb.Close()` on Fx OnStop |
| 9 | Readiness probe (`/readyz` checks DB + Redis) | `cmd/server/main.go:56-65` | **PASS** | Pings both, returns 503 on failure |
| 10 | Safe UUID parsing (no `MustParse`) | All `internal/` files | **PASS** | Zero occurrences of `uuid.MustParse` |

**Result: 10/10 fixes verified correct.**

---

## 2. New Issues Found

| Sev | ID | Description | File | Line |
|---|---|---|---|---|
| Medium | N-1 | `SanitizeHeader` is exported but never called anywhere -- dead code | `middleware/request_log.go` | 63 |
| Medium | N-2 | RBAC applies `PermUserRead` to ALL endpoints including CreateUser, UpdateUser, DeleteUser. Write/delete operations should require `PermUserWrite`/`PermUserDelete` respectively. Comment on line 22 says "Write/delete checked in handler via Connect interceptors" but no such interceptors exist. | `user/adapters/grpc/routes.go` | 22-23 |
| Medium | N-3 | `repository.go` is 222 lines, slightly over the 200-line project guideline. Cursor helpers (lines 200-221) could be extracted to a separate file. | `user/adapters/postgres/repository.go` | 200-222 |
| Medium | N-4 | `create_user_test.go` only tests success and email-taken paths. Missing tests for: invalid role, hash failure, repo.Create failure. | `user/app/create_user_test.go` | -- |
| Medium | N-5 | `os.Exit(1)` in Echo start goroutine bypasses Fx shutdown hooks. Server start failure won't cleanly close DB/Redis/AMQP. | `cmd/server/main.go` | 78 |
| Medium | N-6 | `mp.Shutdown(ctx)` error is silently discarded with `_`. Should at minimum log it. | `internal/shared/module.go` | 33 |
| Medium | N-7 | Audit module `fx.Provide(func(pool) *sqlcgen.Queries)` creates a second `*sqlcgen.Queries` instance. If another module also needs Queries, Fx will have ambiguity. Should be deduplicated or named. | `internal/modules/audit/module.go` | 12-14 |
| Minor | N-8 | `RequestLogger` middleware calls `c.Error(err)` then returns nil, which means the error is handled twice (once by `c.Error` which triggers `ErrorHandler`, and the original `return nil` suppresses the error from the middleware chain). This is an intentional Echo pattern for logging but worth a comment. | `middleware/request_log.go` | 24-26 |
| Minor | N-9 | Cron `s.cron.Stop()` is not `StopCtx()` -- running jobs won't be waited for. `robfig/cron/v3` has `Stop()` which returns a channel but the code doesn't wait on it. | `internal/shared/cron/scheduler.go` | 63 |
| Minor | N-10 | `errors.Is(err, domain.ErrEmailTaken)` in `create_user_test.go:67` works because `ErrEmailTaken` is a pointer sentinel, but the repo returns the same pointer via pgErr path. This is correct but fragile -- if anyone wraps the error, the test breaks. | `user/adapters/postgres/repository_test.go` | 67 |
| Minor | N-11 | `UserCreatedEvent` includes `Email` and `Name` in the event payload published to RabbitMQ. If audit log stores `changes` JSON, PII is persisted in audit trail. This is likely intentional for audit but worth noting for GDPR. | `internal/shared/events/topics.go` | 13-20 |

---

## 3. Architecture Assessment

**Strengths:**
- Clean hexagonal architecture: domain has no infrastructure imports
- Port/adapter boundary is well-defined (`domain.UserRepository` interface)
- Closure-based `Update(ctx, id, func(*User) error)` pattern is elegant for transactional UoW
- Domain entities use unexported fields with getters -- proper encapsulation
- `Reconstitute()` function for hydration from persistence -- correct DDD pattern
- Fx DI wiring is well-organized with named modules
- Event handler registration via `group:"event_handlers"` tag is extensible

**Weaknesses:**
- N-2 above: RBAC is too coarse (read-only permission covers write endpoints)
- Single module (user) makes it hard to fully validate cross-module patterns
- No login/auth endpoint means the entire auth flow is untestable end-to-end

---

## 4. Security Assessment

**Strengths:**
- Argon2id with proper parameters (3 iterations, 64MB, 4 threads, 32-byte key)
- Constant-time comparison for password verification
- JWT signing method validated against HMAC explicitly
- Token blacklist check in auth middleware
- CRLF injection prevention in SMTP sender
- Security headers (HSTS, X-Frame-Options, CSP via Permissions-Policy)
- Sensitive headers redacted from logs
- Input validation at domain layer (email, name, role)
- buf.validate interceptor for protobuf validation
- Swagger UI disabled in production
- Rate limiting with Redis sliding window
- Body limit (10MB)

**Weaknesses:**
- N-2: Write/delete operations only require `user:read` permission
- No password complexity validation (only hashing)
- No email format validation (only emptiness check)
- Rate limit is global (100/min) -- no per-endpoint differentiation

---

## 5. Scores

| Category | Score | Rationale |
|---|---|---|
| **Architecture** | 9/10 | Clean hexagonal, proper DDD patterns, extensible event system. Minor: single module limits validation. |
| **Security** | 8/10 | Solid crypto, auth, headers. RBAC granularity gap (N-2) and no password policy are the main gaps. |
| **Code Quality** | 8.5/10 | Clean, readable, well-organized. Minor dead code, one file over limit. Tests cover happy paths well. |
| **DX (Developer Experience)** | 8/10 | Testcontainers for integration tests, Fx modules are composable, clear file naming. Missing: no login endpoint, cron has zero jobs registered. |
| **Overall** | **8.3/10** | |

---

## 6. Verdict

**Ship-ready as a boilerplate** with one advisory:

The RBAC granularity issue (N-2) is the only item I would call "should fix before real use." All RPC endpoints (Create, Update, Delete) pass through with just `user:read` permission. The comment in routes.go claims write/delete are "checked in handler via Connect interceptors" but no such interceptors exist. For a boilerplate, this is acceptable if documented. For production use, it must be fixed.

Everything else is clean, well-structured, and production-grade infrastructure code. The 10 prior fixes have all been correctly implemented. Build passes. No panics, no secrets exposed, no unsafe patterns.

**Recommendation:** Ship with a `TODO` comment on routes.go line 22 noting that write/delete operations need `PermUserWrite`/`PermUserDelete` when real RBAC is implemented.

---

## 7. File Reference

Key files reviewed (all absolute paths):

- `/Users/namnguyen/Desktop/www/freelance/gnha-services/cmd/server/main.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/module.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/config/config.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/events/module.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/events/subscriber.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/request_id.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/swagger.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/rbac.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/cron/scheduler.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/observability/tracer.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/observability/metrics.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/grpc/routes.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go`
- `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user_test.go`
