# Post-Fix Full Codebase Review

**Date:** 2026-03-06
**Scope:** All 55 non-generated Go source files, 6 docs, 7 config files, 19 scaffold templates
**Build:** `go build ./...` PASS | `go vet ./...` PASS | Unit tests PASS

---

## Verification of Recent Fixes

| Fix | Status | Notes |
|-----|--------|-------|
| C3: parseUserID in repository.go | VERIFIED | `uuid.Parse` with proper DomainError on failure |
| I1: Import ordering audit/notification subscribers | VERIFIED | stdlib, third-party, internal grouping correct |
| I2: slog.ErrorContext + module prefix in notification | VERIFIED | Line 41 uses `slog.ErrorContext(ctx, "notification: ..."` |
| I4-I5: code-standards.md examples | VERIFIED | Postgres adapter, fx.Module, test stubs all match actual code |
| I12: Removed model/ from architecture.md | VERIFIED | No mention of model/ anywhere |
| I13: FAILED_PRECONDITION + UNAVAILABLE in error-codes.md | VERIFIED | Both present in table |
| I6-I7: README Prerequisites + Dev Services | VERIFIED | Tables present and accurate |

All 7 fixes are correct. No regressions introduced.

---

## Remaining Issues

| # | File | Issue | Severity | Recommendation |
|---|------|-------|----------|----------------|
| 1 | `internal/shared/observability/tracer.go:22` | `otlptracegrpc.WithInsecure()` hardcoded -- production traffic to OTLP collector will be unencrypted | Important | Guard with `if cfg.IsDevelopment()` or use `WithTLSCredentials` for prod |
| 2 | `internal/shared/observability/metrics.go:20` | Same `WithInsecure()` issue for metric exporter | Important | Same fix as #1 |
| 3 | `cmd/server/main.go:50-55` | `/healthz` and `/readyz` return static `{"status":"ok"}` without checking DB/Redis/RabbitMQ | Important | `/readyz` should ping `pool.Ping()`, `rdb.Ping()` at minimum; `/healthz` can stay static for liveness |
| 4 | `internal/shared/database/postgres.go` | Pool not shut down via Fx lifecycle -- `pool.Close()` never called on graceful shutdown | Important | Add `fx.Invoke(registerPoolShutdown)` or wrap in a constructor that registers `lc.Append(OnStop: pool.Close)` |
| 5 | `internal/shared/database/redis.go` | Same: Redis client not closed on Fx shutdown | Important | Register `rdb.Close()` in Fx lifecycle OnStop |
| 6 | `internal/shared/events/subscriber.go:59` | `router.Run(context.Background())` ignores Fx shutdown context -- router may hang during graceful shutdown | Minor | Pass a cancellable context derived from OnStart, cancel it in OnStop before `router.Close()` |
| 7 | `internal/modules/user/adapters/grpc/routes.go` | Auth middleware applied but no RBAC -- any authenticated user can CRUD all users | Important | Apply `appmw.RequireRole("admin")` or `appmw.RequirePermission(...)` for write operations |
| 8 | `db/queries/user.sql` + generated `ListUsers` | SELECT * returns password hash in result set; flows through `toDomain()` -> domain entity -> `toProto()` (proto omits it, but hash sits in memory through entire call chain) | Minor | Use explicit column list excluding `password` in ListUsers query, or accept the risk since proto mapping strips it |
| 9 | `internal/shared/middleware/request_id.go:19` | Client-supplied X-Request-ID not validated for length or content -- could be used for header injection or log poisoning | Minor | Add `len(id) > 128 || !isAlphanumDash(id)` guard, generate new UUID on invalid input |
| 10 | `internal/shared/cron/scheduler.go` | Scheduler starts with zero registered jobs -- `s.cron.Start()` is a no-op but wastes a goroutine | Minor | Guard `Start()` with `if len(s.cron.Entries()) > 0` or document that this is intentional placeholder |
| 11 | `Taskfile.yml:157-162` + `deploy/` | `monitor:up` references `deploy/docker-compose.monitor.yml` which does not exist | Minor | Create the file or remove the task |
| 12 | `Taskfile.yml` | No `dev:down` task to stop dev infra containers | Minor | Add `dev:down: cmds: [docker compose -f deploy/docker-compose.dev.yml down]` |
| 13 | `internal/shared/middleware/swagger.go:30` | Swagger UI hardcodes `user.swagger.json` -- won't auto-discover new module APIs | Minor | Enumerate `gen/openapi/*/v1/*.swagger.json` or accept manual update per module |
| 14 | `code-standards.md:138-146` | Error codes section lists only 6 codes; actual code has 8 (missing `FAILED_PRECONDITION`, `UNAVAILABLE`) | Minor | Add the two missing codes to the code-standards.md error codes listing |
| 15 | `docs/adding-a-module.md:276` | Product adapter example uses `uuid.MustParse` in `Create()` -- contradicts the safe `parseUserID` pattern used in actual user module and scaffold template | Important | Replace `uuid.MustParse(string(p.ID()))` with `parseProductID(p.ID())` in the doc example |
| 16 | `code-standards.md:598` | Test example uses `require.NoError(t, err)` / `assert.Equal()` from testify, but go.mod uses stdlib + gomock only (testify is indirect via testcontainers) | Minor | Change doc example to use stdlib `t.Fatal`/`t.Errorf` to match actual test patterns |
| 17 | `code-standards.md:597` | Test example passes `&noopPublisher{}` directly to `NewCreateUserHandler` but actual signature takes `*events.EventBus` -- doc example won't compile | Minor | Fix to `events.NewEventBus(&noopPublisher{})` matching actual code |
| 18 | `docs/adding-a-module.md:274` | Product `Create` adapter doc example doesn't include `pgconn.PgError` unique constraint handling that the user module and scaffold template both have | Minor | Add constraint error handling to match actual pattern |
| 19 | `internal/modules/audit/subscriber.go` | All 3 handlers log+return `nil` on invalid UUID (ack bad data) but log+return `err` on bad JSON (nack for retry). This is correct but undocumented -- a new dev might be confused | Minor | Add brief comment explaining the ack-vs-retry strategy |
| 20 | `.gitlab-ci.yml:56` | `coverage_report` artifact references `coverage.xml` (Cobertura format) but `go test -coverprofile` produces Go format, not Cobertura XML. GitLab will silently ignore it | Minor | Add `gocover-cobertura` conversion step, or remove the Cobertura artifact block |
| 21 | `docs/adding-a-module.md:390-397` | Event publish example for product omits `ActorID` field, inconsistent with the pattern shown in code-standards.md and actual user module | Minor | Add `ActorID: actorID` to the event publish example |

---

## Summary

**Remaining issues: 0 critical, 5 important, 16 minor**

### Important (should fix before production):
1. **OTel WithInsecure()** in tracer.go + metrics.go (#1, #2)
2. **Health probes are stubs** -- /readyz should check infra (#3)
3. **DB/Redis pools not shutdown** via Fx lifecycle (#4, #5)
4. **No RBAC on Connect routes** -- auth-only, no role check (#7)
5. **adding-a-module.md uses MustParse** -- contradicts safe pattern (#15)

### Positive Observations:
- Build is clean: `go build`, `go vet`, unit tests all pass
- All recent fixes verified correct, no regressions
- Scaffold templates match actual code patterns (parseID, fx.Module, testutil names)
- Error code mapping is complete across domain_error.go, mapper.go, and error-codes.md
- Event system is consistent: all mutations publish, all include ActorID
- Security fundamentals solid: Argon2id, JWT blacklist, CRLF sanitization, security headers
- Cursor pagination implementation is correct and well-documented

### Unresolved Questions
- Is the cron module intended as a placeholder, or should it be removed until jobs exist?
- Should the monitoring compose file be created now, or deferred until SigNoz is actually configured?
