# Full Codebase Review -- gnha-services Boilerplate

**Date:** 2026-03-06
**Reviewer:** code-reviewer agent
**Scope:** All Go files under `internal/`, `cmd/`, `db/`, `deploy/`, Dockerfile, CI/CD, Taskfile
**LOC:** ~3,500 hand-written Go (55 files, excludes gen/)

---

## Area Ratings

| Area | Score | Notes |
|---|---|---|
| 1. Architecture | 9/10 | Textbook hexagonal. Clean module boundaries, proper DI. |
| 2. Security | 7.5/10 | Solid foundation but RBAC too coarse, no login endpoint, no CSRF. |
| 3. Error Handling | 9/10 | DomainError -> HTTP/Connect mapping is clean and consistent. |
| 4. Database | 8.5/10 | Good pool config, sqlc, keyset pagination, FOR UPDATE in tx. |
| 5. Event System | 8/10 | Watermill + OTel propagation, proper retry + recoverer. |
| 6. Testing | 7/10 | Good integration tests, but unit test coverage is thin. |
| 7. Observability | 8.5/10 | OTel traces + metrics, structured slog, request logging. |
| 8. Code Quality | 8.5/10 | Idiomatic Go, good naming, files under 200 LOC (except one). |
| 9. Deployment | 8/10 | Multi-stage Docker, Traefik TLS, CI/CD 4-stage. |
| 10. Scaffold Generator | 8.5/10 | 19 templates, conflict detection, reserved word guard. |

**Overall: 8.3/10**

---

## Critical Issues

None found. No data-loss bugs, no exposed secrets, no broken auth bypass.

---

## High Priority Issues

### H-1: RBAC Only Enforces `user:read` on All Endpoints (Security)

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/grpc/routes.go:23`

The route group applies `appmw.RequirePermission(appmw.PermUserRead)` to ALL user endpoints including Create, Update, and Delete. A viewer with `user:read` permission can write and delete users.

```go
// Current -- all endpoints get user:read only
g := e.Group(path, appmw.Auth(cfg, rdb), appmw.RequirePermission(appmw.PermUserRead))
```

**Fix:** Apply granular permissions per operation. The simplest approach: use Connect interceptors or per-method middleware. Alternatively, move RBAC into each handler's `Handle()` method.

**Scaffold template also affected:** `cmd/scaffold/templates/adapter_grpc_routes.tmpl` line 21 applies `Auth` but no `RequirePermission` at all -- better than wrong permission, but still means scaffolded modules have no RBAC by default.

**Severity:** High

### H-2: `os.Exit(1)` in Server Start Goroutine Bypasses Fx Shutdown (Reliability)

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/cmd/server/main.go:78`

```go
go func() {
    if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
        slog.Error("server error", "err", err)
        os.Exit(1) // kills process, skips DB close, OTel flush, AMQP close
    }
}()
```

**Fix:** Signal Fx to shut down gracefully instead:
```go
go func() {
    if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
        slog.Error("server error", "err", err)
        // Let Fx handle graceful shutdown
    }
}()
```
Or inject `fx.Shutdowner` and call `shutdowner.Shutdown()`.

**Severity:** High

### H-3: `create_user_test.go` Missing Error-Path Tests (Testing)

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user_test.go`

Only tests: success + email-taken. Missing:
- Invalid role (empty, unknown)
- Hash failure
- Repo.Create failure
- Repo.GetByEmail non-ErrNotFound failure

The test file is the only unit test for application-layer logic. Other handlers (get, list, update, delete) have zero unit tests.

**Severity:** High

---

## Medium Priority Issues

### M-1: ListUsers Returns Password Hashes Through Call Chain

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go:79`

`toDomain()` includes `row.Password` in every User returned by `List()`. The proto mapper at `mapper.go:14` correctly strips it (no password field in `userv1.User`), but any internal caller of `ListUsersHandler.Handle()` receives User objects with password hashes exposed.

**Mitigation:** Already mitigated by proto mapper, but worth noting for future internal API consumers.

**Severity:** Medium

### M-2: Audit `ActorID` Falls Back to `EntityID` for System Operations

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/audit/subscriber.go:24-35`

When `actorID` is empty (e.g., system-initiated operations like seeding), `parseActorID` falls back to `entityID`. This means the audit log shows the user created themselves. This is a design choice, not a bug, but could mislead audit trail analysis.

**Severity:** Medium (by design)

### M-3: Audit Module Creates Its Own `sqlcgen.Queries` Instance

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/audit/module.go:12-13`

```go
fx.Provide(func(pool *pgxpool.Pool) *sqlcgen.Queries {
    return sqlcgen.New(pool)
}),
```

If another module also provides `*sqlcgen.Queries`, Fx will fail with a duplicate provider error. Should use `fx.Annotate` with names or provide via shared module.

**Severity:** Medium

### M-4: `mp.Shutdown(ctx)` Error Silently Discarded

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/module.go:33`

```go
_ = mp.Shutdown(ctx)
```

MeterProvider shutdown error is swallowed. Should at least log it.

**Severity:** Medium

### M-5: Cron Scheduler Starts With Zero Registered Jobs

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/cron/scheduler.go`

No jobs are added to the scheduler anywhere in the codebase. The cron module starts successfully but does nothing. This is by-design for a boilerplate, but should be documented.

**Severity:** Medium (by design)

### M-6: No Login/Auth Endpoint

The boilerplate has JWT generation (`auth.GenerateAccessToken`) and validation infrastructure but no actual login endpoint. Users cannot obtain tokens. This means the entire auth + RBAC middleware chain cannot be tested end-to-end without manually crafting JWTs.

**Severity:** Medium

### M-7: PII in Audit Trail

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/events/topics.go:14-18`

`UserCreatedEvent` includes `Email` and `Name`, which get stored as JSON in `audit_logs.changes`. Under GDPR, this creates PII in the audit trail that may need special handling (retention, deletion, anonymization).

**Severity:** Medium

### M-8: `CreateAuditLog` Query Missing `ip_address` Parameter

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/db/queries/audit.sql:2`

```sql
INSERT INTO audit_logs (entity_type, entity_id, action, actor_id, changes, ip_address)
VALUES ($1, $2, $3, $4, $5, $6);
```

The query expects 6 parameters including `ip_address`, but the subscriber at `subscriber.go:54` passes a `CreateAuditLogParams` with no `IpAddress` field set. This will insert NULL, which is valid, but the field is there for a reason and never populated.

**Severity:** Medium

---

## Low Priority Issues

### L-1: `SanitizeHeader` Dead Code Pattern

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/notification/email.go:31`

The `sanitize` closure is defined inline in `Send()`. This is fine, but `request_log.go` had a `SanitizeHeader` function in a previous iteration that appears to have been removed. No issue currently -- just noting code is clean.

### L-2: `repository.go` at 222 Lines

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go`

Slightly over the 200-line guideline from `development-rules.md`. The cursor helpers (lines 200-221) could be extracted to a shared `pagination` package.

### L-3: Cron `Stop()` Does Not Wait for Running Jobs

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/cron/scheduler.go:63`

```go
s.cron.Stop() // returns immediately
```

`robfig/cron.Stop()` returns a context that completes when running jobs finish. Should use `<-s.cron.Stop().Done()` or the context-aware variant.

### L-4: Dev Redis Has No Password, Prod Does

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/deploy/docker-compose.dev.yml:21`

Dev Redis runs without `--requirepass` while prod uses `${REDIS_PASSWORD}`. This is normal for dev but could cause surprise if code relies on auth.

### L-5: `Reconstitute()` Uses Exported Function with Unexported Fields

The `Reconstitute()` function in domain entities takes a long parameter list. For entities with many fields, this becomes unwieldy. Consider a `ReconstitutionData` struct pattern for future modules with 10+ fields.

### L-6: CI Coverage Report References `coverage.xml` But Generates `coverage.out`

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/.gitlab-ci.yml:56`

```yaml
coverage_report:
  coverage_format: cobertura
  path: coverage.xml  # This file is never generated
```

The pipeline generates `coverage.out` (Go format) but references `coverage.xml` (Cobertura format). Need `gocover-cobertura` to convert, or remove the Cobertura artifact.

**Severity:** Low (CI cosmetic)

### L-7: Dockerfile Uses `alpine:3.19` (Slightly Old)

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/Dockerfile:13`

Alpine 3.19 is from Dec 2023. Current is 3.21. Not a security issue yet but should be updated periodically.

### L-8: Scaffold `validateIdentifier` Rejects Digits

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/cmd/scaffold/main.go:170`

```go
if !unicode.IsLetter(r) && r != '_' {
```

Module names like `oauth2` would be rejected. Go identifiers allow digits (after first char). Minor since module names rarely contain digits.

---

## Positive Observations

1. **Hexagonal architecture done right.** Domain has zero infrastructure imports. Adapters depend inward only. Fx wires everything at the edge.

2. **Closure-based Update pattern** (`repo.Update(ctx, id, func(*User) error)`) is elegant -- keeps transaction management in the adapter while domain logic stays pure.

3. **Keyset pagination** with base64 JSON cursors is production-grade. `limit+1` fetch pattern for `hasMore` detection is correct.

4. **Security defense-in-depth**: Argon2id (not bcrypt), constant-time comparison, CRLF injection prevention in SMTP, XSS escaping in Swagger HTML, security headers, CORS with explicit origins, rate limiting with sliding window.

5. **Sentinel domain errors** that map cleanly to both HTTP status codes and Connect RPC codes. Single error mapping table, no duplication.

6. **OTel trace propagation** through Watermill messages is a detail most boilerplates miss.

7. **Scaffold generator** produces 19 files with proper build tags, mockgen directives, and integration test structure. Conflict detection prevents overwriting existing code.

8. **Clean shutdown chain**: Fx lifecycle hooks close DB pool, Redis, AMQP publisher/subscriber, OTel providers, cron scheduler, and Echo server in reverse order.

9. **Token blacklist** via Redis `EXISTS` check enables logout functionality without waiting for token expiry.

10. **CI/CD pipeline** has proper staging/production separation, manual approval for prod deploys, health checks in Docker, and generated-code drift detection.

---

## Edge Cases Found

1. **Race in `UpdateUserHandler`**: The `updated` variable is set inside the closure passed to `repo.Update()`. If the closure is called but the transaction commit fails, `updated` still points to the modified entity, and the event is published even though the DB write was rolled back. Current code publishes after `repo.Update()` returns nil, so this is safe -- the event fires only on commit success.

2. **Cursor decode failure silently ignored**: In `repository.go:66-69`, if `decodeCursor()` fails, params retain zero-value cursors, effectively returning the first page. This is reasonable fail-safe behavior but could mask client bugs.

3. **Concurrent duplicate email creation**: Two `CreateUser` calls with the same email could both pass the `GetByEmail` uniqueness check. The DB unique constraint (`users_email_key`) catches this correctly, mapped to `ErrEmailTaken`. Good defense-in-depth.

4. **Rate limit key uses `c.RealIP()`**: Behind a reverse proxy without proper `X-Forwarded-For` headers, all requests would share the same IP-based rate limit key. Traefik handles this correctly, but direct access would not.

5. **`uuid.NewString()` in domain `NewUser`**: This uses Google's UUID library which uses `crypto/rand`. Safe for production. No `uuid.MustParse` on untrusted input.

---

## Recommended Actions (Priority Order)

1. **[High] Fix RBAC granularity** -- Apply `PermUserWrite` to Create/Update and `PermUserDelete` to Delete, either via per-method middleware or Connect interceptors.

2. **[High] Replace `os.Exit(1)` with `fx.Shutdowner`** in server start goroutine.

3. **[High] Add unit tests** for all application handlers (get, list, update, delete) and error paths in create_user.

4. **[Medium] Add a login endpoint** -- even a minimal one -- so the boilerplate can be tested end-to-end.

5. **[Medium] Fix CI coverage artifact** -- either generate Cobertura XML or remove the `coverage_report` section.

6. **[Medium] Log `mp.Shutdown()` error** instead of discarding.

7. **[Medium] Use `fx.Annotate` with name tags** for audit module's `*sqlcgen.Queries` to avoid future DI conflicts.

8. **[Low] Extract cursor helpers** to a shared pagination package.

9. **[Low] Update Alpine base image** to 3.21 in Dockerfile.

10. **[Low] Use context-aware cron stop**: `<-s.cron.Stop().Done()`.

---

## Metrics

| Metric | Value |
|---|---|
| Hand-written Go LOC | ~3,500 |
| Files (hand-written) | 55 |
| Unit test files | 3 (domain, app create, repo integration) |
| Integration test files | 1 (repository_test.go with build tag) |
| Test coverage (estimated) | ~35-40% (domain well-covered, app/middleware/events not covered) |
| Lint issues | Not run (no golangci-lint config found in repo root) |
| Scaffold templates | 19 |
| Deployment configs | 4 (Dockerfile, 2x docker-compose, traefik) |

---

## Unresolved Questions

1. Should the boilerplate include a minimal login endpoint, or is that intentionally left for implementors?
2. Is the `admin:*` wildcard permission pattern intentional, or should it use the role-based `u.Role == "admin"` check exclusively?
3. Should generated code (`gen/sqlc`, `gen/proto`) be committed to the repo or regenerated in CI? Currently both approaches are supported (CI checks drift).
4. The `ELASTICSEARCH_URL` config field exists but no code uses Elasticsearch. Is this planned or should it be removed (YAGNI)?
