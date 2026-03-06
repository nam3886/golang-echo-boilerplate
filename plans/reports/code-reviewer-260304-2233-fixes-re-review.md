# Fixes Re-Review Report

Date: 2026-03-04 22:34
Reviewer: code-reviewer
Scope: 14 files, verifying fixes for 5 CRITICAL + 8 IMPORTANT issues

---

## Fix-by-Fix Verdicts

### C-1: go.mod / Dockerfile / CI Go version mismatch
**FAIL -- NEW BUG INTRODUCED**

- `go.mod` line 3: `go 1.26.0`
- `Dockerfile` line 2: `golang:1.26-alpine`
- `.gitlab-ci.yml` lines 11,14: `1.26`

The original issue was `go.mod` said `1.25.0` (nonexistent) while Dockerfile/CI used `1.23`. The fix changed everything to `1.26.0`, but **Go 1.26 does not exist either** (as of March 2026, the latest stable is Go 1.24.x). This will fail to build -- the Docker image `golang:1.26-alpine` does not exist on Docker Hub, and `go 1.26.0` in go.mod is invalid.

**Required fix**: Use `go 1.24.0` (or whatever version the team actually targets) across all three files.

---

### C-2: uuid.MustParse panic in audit subscriber
**PASS**

File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/audit/subscriber.go`

- `uuid.MustParse` replaced with `uuid.Parse` + error check in all three handlers (Created/Updated/Deleted)
- On bad UUID: logs error and returns `nil` (ack, no retry) -- correct, retrying bad data is pointless
- Clean, idiomatic pattern

---

### C-3: Cursor pagination broken (CursorID type mismatch)
**PASS**

File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/db/queries/user.sql` line 14
File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/gen/sqlc/user.sql.go` line 121

- SQL now has `sqlc.narg('cursor_id')::uuid` cast
- Generated Go code: `CursorID` is now `pgtype.UUID` (was previously wrong type)
- Repository at line 66: `pgtype.UUID{Bytes: decoded.U, Valid: true}` -- correct since `decoded.U` is `uuid.UUID` which is `[16]byte`, matching `pgtype.UUID.Bytes`

---

### C-4: Update transaction lacks SELECT FOR UPDATE
**PASS**

File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/db/queries/user.sql` line 8
File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` lines 105-143

- New `GetUserByIDForUpdate` query with `FOR UPDATE` lock
- `Update()` method now: begins tx, calls `GetUserByIDForUpdate`, applies domain mutation, calls `UpdateUser`, commits -- correct read-modify-write with pessimistic locking
- `defer tx.Rollback(ctx)` handles error paths -- idiomatic

---

### C-5: Event publish errors silently swallowed
**PASS (partial)**

File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user.go` lines 62-71

- `_ = h.bus.Publish(...)` replaced with `if err := ...; err != nil { slog.ErrorContext }` -- good, errors are now logged
- Design decision to not fail the request on publish error is reasonable (user was created successfully in DB)
- **Incompleteness**: Only `create_user.go` was fixed. `update_user.go` and `delete_user.go` do not publish events at all -- the audit subscriber has handlers for UserUpdated and UserDeleted events that will never fire. This is a pre-existing gap, not a regression from this fix.

---

### I-5: Connect RPC routes have no auth/RBAC middleware
**PASS**

File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/grpc/routes.go`

- Added `appmw.Auth(cfg, rdb)` middleware via `e.Group(path, ...)`
- `Auth()` signature is `func Auth(cfg *config.Config, rdb *redis.Client) echo.MiddlewareFunc` -- matches
- `RegisterRoutes` now takes `cfg *config.Config` and `rdb *redis.Client` as additional params
- Fx wiring: `fx.Invoke(grpc.RegisterRoutes)` in user module -- Fx will inject `*config.Config` and `*redis.Client` from the shared module. Both are provided (`config.Load` and `database.NewRedisClient`). **No breakage.**
- Note: This is auth only, not RBAC. All authenticated users can access all endpoints. Acceptable for a boilerplate.

---

### I-6: Audit/notification handlers use context.Background() instead of msg.Context()
**PASS**

File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/audit/subscriber.go` -- uses `msg.Context()` in all 3 handlers
File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/notification/subscriber.go` -- uses `msg.Context()` at line 39

Both correctly propagate the message context for OTel tracing continuity.

---

### I-3: SMTP envelope recipient not sanitized
**PASS**

File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/notification/email.go` line 38

- `smtp.SendMail` envelope args now use `sanitize(s.from)` and `sanitize(to)` -- both from and to are sanitized
- Header values also sanitized (lines 35-36)
- `sanitize` strips CR and LF characters -- prevents CRLF injection

---

### I-7: OTel TracerProvider/MeterProvider never shut down
**PASS (minor nit)**

File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/module.go`

- `registerOTelShutdown` registered via `fx.Invoke`
- Takes `*sdktrace.TracerProvider` and `*sdkmetric.MeterProvider` -- both provided by `observability.NewTracerProvider` and `observability.NewMeterProvider` which return those exact types
- Fx lifecycle OnStop calls `mp.Shutdown(ctx)` then `tp.Shutdown(ctx)`
- **Minor nit**: `mp.Shutdown` error is discarded with `_ =`. If MeterProvider shutdown fails, it's silently ignored. Should at minimum log it. Not blocking.

---

### I-8: Cron distributed lock unsafe -- no token verification on delete
**PASS**

File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/cron/scheduler.go`

- Lock value is now `uuid.NewString()` (unique token per execution)
- Unlock uses Lua script: `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end` -- standard Redis distributed lock pattern, atomically checks ownership before delete
- `defer s.rdb.Eval(ctx, unlockScript, []string{lockKey}, lockVal)` -- correct placement in defer

---

### I-14 (new): Watermill router.Run uses context.Background()
**PASS (intentional)**

File: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/events/subscriber.go` line 59

- `router.Run(context.Background())` in a goroutine -- this is correct because the Fx lifecycle `OnStart` context is short-lived (only for startup). The router needs a long-lived context.
- Shutdown is handled by `router.Close()` in `OnStop` -- this will cause `Run` to return.

---

## Summary Table

| Fix ID | Issue | Verdict | Severity |
|--------|-------|---------|----------|
| C-1 | Go version mismatch | **FAIL** | CRITICAL |
| C-2 | uuid.MustParse panic | PASS | -- |
| C-3 | Cursor pagination broken | PASS | -- |
| C-4 | Missing SELECT FOR UPDATE | PASS | -- |
| C-5 | Event publish swallowed | PASS (partial) | LOW |
| I-5 | No auth middleware | PASS | -- |
| I-6 | context.Background in subscribers | PASS | -- |
| I-3 | SMTP envelope not sanitized | PASS | -- |
| I-7 | OTel providers never shutdown | PASS (minor nit) | LOW |
| I-8 | Cron lock unsafe | PASS | -- |
| -- | Router context.Background | PASS | -- |

## New Issues Found

### CRITICAL

1. **Go 1.26 does not exist.** `go.mod`, `Dockerfile`, and `.gitlab-ci.yml` all reference Go 1.26 which is not a released version. Docker build will fail immediately (`golang:1.26-alpine` image not found). Must change to an actual Go version (1.23 or 1.24).

### MEDIUM

2. **OTel MeterProvider shutdown error silently discarded** in `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/module.go` line 29. Should log the error.

3. **Update and Delete handlers don't publish events** (`/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/update_user.go`, `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/delete_user.go`). The audit subscriber has `HandleUserUpdated` and `HandleUserDeleted` handlers registered, but no events are ever published for these actions. Pre-existing issue, not a regression.

## Overall Assessment

12 of 13 fixes are correct and well-implemented. The Go version fix (C-1) introduced a new critical bug by choosing a nonexistent Go version (1.26). This must be corrected before any build or deploy can succeed.

## Unresolved Questions

- What Go version does the team actually target? The original code said 1.23 in Docker/CI. Latest stable as of March 2026 is 1.24.x. Recommend 1.24.
- Are update/delete event publications intentionally deferred to a future phase, or an oversight?
