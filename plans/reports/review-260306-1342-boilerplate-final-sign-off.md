# Final Sign-Off Review: gnha-services Boilerplate

**Date:** 2026-03-06
**Reviewer:** code-reviewer agent
**Scope:** ALL 44 hand-written Go files in `internal/` + `cmd/server/` (2,918 LOC)
**Build:** `go build` PASS | `go vet` PASS | zero `os.Exit`/`log.Fatal`/`panic()`/`uuid.MustParse`

---

## 1. Fixes #15-17 Verification

### Fix #15: os.Exit(1) replaced with fx.Shutdowner -- PASS

**File:** `cmd/server/main.go:69-87`

`startServer` now accepts `fx.Shutdowner` and the goroutine calls `shutdowner.Shutdown(fx.ExitCode(1))` instead of `os.Exit(1)`. This allows Fx to run all OnStop hooks in order before process exit. Correct.

### Fix #16: RBAC Connect interceptor for write/delete -- PASS

**File:** `internal/shared/middleware/rbac_interceptor.go`

- `RBACInterceptor()` is a Connect `UnaryInterceptorFunc` that extracts the method name from the procedure string (format: `/package.Service/MethodName`).
- Maps `Create*` and `Update*` to `PermUserWrite`, `Delete*` to `PermUserDelete`.
- Read operations fall through (return empty Permission), relying on the Echo-level `RequirePermission(PermUserRead)`.
- Auth check: returns `CodeUnauthenticated` if no user, `CodePermissionDenied` if missing permission.
- Wired in `routes.go:17-21` with `validate.NewInterceptor()` first, then `RBACInterceptor()`.

Implementation is correct. The `strings.HasPrefix(method, prefix)` approach works for Connect-generated method names (`CreateUser`, `UpdateUser`, `DeleteUser`, `GetUser`, `ListUsers`).

### Fix #17: mp.Shutdown error logged -- PASS

**File:** `internal/shared/module.go:33-34`

```go
if err := mp.Shutdown(ctx); err != nil {
    slog.Warn("meter provider shutdown error", "err", err)
}
```

Error is now logged instead of silently discarded. Correct.

---

## 2. RBAC Interceptor Deep Analysis

**Edge case: procedure name parsing.**
The procedure format is always `/package.v1.Service/MethodName` in Connect RPC. `strings.LastIndex(procedure, "/")` correctly isolates the method name. The `HasPrefix` approach means any future RPC starting with "Create", "Update", or "Delete" will automatically require the correct permission. Methods like `Get` or `List` have no prefix match and fall through to read-only (Echo-level RBAC). This is correct and extensible.

**Edge case: admin bypass.**
`HasPermission` in `auth/context.go:37-43` checks for `admin:*` permission OR `admin` role, so admins always pass. Correct.

**Edge case: unauthenticated requests.**
The Echo-level `Auth()` middleware runs before the interceptor. If JWT is missing/invalid, the request is rejected at Echo level (401) before reaching the Connect interceptor. The interceptor's own nil-user check is defense-in-depth. Correct.

---

## 3. Architecture Assessment

**Hex arch compliance:** Clean. Domain has zero imports from adapters/infra. App layer depends only on domain + shared. Adapters depend on app + domain. No import cycles detected.

**Module boundaries:** Each module (user, audit, notification, cron) is self-contained with its own Fx module. Cross-module communication is exclusively through the event bus (Watermill). No direct imports between user/audit/notification modules.

**DI graph:** Fx provides/invokes are well-structured. The `group:"event_handlers"` pattern for handler registration is clean and extensible.

---

## 4. Auth Chain Verification

Request flow for a write operation (e.g., `CreateUser`):

1. **Recovery** -- catches panics
2. **RequestID** -- generates/validates X-Request-ID (>128 chars rejected)
3. **RequestLogger** -- logs with trace_id, user_id, request_id
4. **BodyLimit** -- 10MB
5. **Gzip** -- compression
6. **SecurityHeaders** -- HSTS, X-Frame-Options, etc.
7. **CORS** -- configurable origins
8. **ContextTimeout** -- 30s
9. **RateLimit** -- 100 req/min per user/IP via Redis sliding window
10. **Auth** (route group) -- JWT validation + blacklist check
11. **RequirePermission(PermUserRead)** (route group) -- base RBAC
12. **validate.NewInterceptor()** (Connect) -- protobuf validation
13. **RBACInterceptor()** (Connect) -- write/delete permission check

Chain is complete and correctly ordered.

---

## 5. Shutdown Ordering

Fx hooks execute in LIFO order (last registered OnStop runs first):

1. `shared.Module` registers: OTel shutdown + DB shutdown
2. `events.Module` registers: router start + AMQP shutdown
3. `cron.Module` registers: cron start
4. `startServer` registers: Echo server start

On shutdown (reverse order):
1. Echo server stops (drains HTTP connections)
2. Cron stops
3. AMQP publisher/subscriber close
4. Watermill router closes
5. DB pool + Redis close
6. OTel providers flush

This is correct. HTTP stops first, then background workers, then infrastructure. No resource leak.

---

## 6. Remaining Issues

| # | Severity | File | Issue |
|---|----------|------|-------|
| 1 | LOW | `repository.go` | 222 lines (2 lines over 200-line guideline). Trivial. |
| 2 | LOW | `cron/scheduler.go:66` | `s.cron.Stop()` returns a `context.Context` channel for waiting on running jobs. Currently ignored. In production with long-running cron jobs, shutdown may not wait for completion. |
| 3 | INFO | `events/topics.go` | PII (email, name) in `UserCreatedEvent` persisted to audit trail. By-design for boilerplate; flag for GDPR review before production with real user data. |
| 4 | INFO | `audit/module.go:12` | Audit module creates its own `sqlcgen.Queries` instance. Fine for now but if multiple modules do this, consider a shared provider. |
| 5 | INFO | `create_user.go` | No password strength validation (length, complexity). The domain validates email/name/role but not password. Acceptable for boilerplate; add before production. |
| 6 | INFO | `email.go:29` | `Send` ignores the `context.Context` parameter (underscore). `net/smtp.SendMail` does not support context cancellation. Known Go stdlib limitation. |

No critical or high issues remain.

---

## 7. Positive Observations

- **Domain purity**: Unexported fields, getters, `Reconstitute()` pattern, domain validation in constructors. Textbook DDD.
- **Error handling**: Sentinel domain errors with HTTP/Connect code mapping. No leaked internal errors to clients.
- **Transactional update**: Closure-based `Update(ctx, id, fn)` with `SELECT FOR UPDATE` is race-condition-safe.
- **Event-driven architecture**: Fire-and-forget with logged errors. Events are published after DB commit (correct ordering).
- **Security**: Argon2id with constant-time comparison, JWT with algorithm pinning, token blacklist via Redis, CRLF injection prevention in SMTP, XSS prevention in Swagger HTML, security headers, rate limiting.
- **Observability**: Structured slog, OTel traces propagated through event bus, request logs include trace_id/user_id/request_id.
- **Cursor pagination**: Keyset pagination (created_at DESC, id DESC) with base64-encoded JSON cursors. Performant at scale.

---

## 8. Scores

| Category | Score | Notes |
|----------|-------|-------|
| Architecture | 9.0/10 | Clean hex arch, proper module boundaries, extensible DI |
| Security | 8.5/10 | Comprehensive for a boilerplate. Missing: password strength validation, RBAC is now granular |
| Production Readiness | 8.0/10 | Readiness probes, graceful shutdown, retry logic. Missing: login endpoint, health check for RabbitMQ |
| Code Quality | 9.0/10 | Consistent patterns, good error handling, minimal dead code |
| Developer Experience | 8.5/10 | Clear module structure, Fx DI, Taskfile, TypeScript codegen |
| **Overall** | **8.5/10** | Improved from 8.3 after fixes #15-17 |

---

## 9. Verdict

**SHIP.** The boilerplate is production-grade for its intended purpose (starter template). All 17 fixes verified. No critical or high-severity issues remain. The remaining LOW/INFO items are appropriate for a boilerplate and documented for future production hardening.

---

## Unresolved Questions

- Will a login/refresh endpoint be added before this ships? (Currently no auth endpoint exists -- JWT tokens can only be generated programmatically.)
- Is the PII-in-audit-trail acceptable for the target deployment jurisdiction?
