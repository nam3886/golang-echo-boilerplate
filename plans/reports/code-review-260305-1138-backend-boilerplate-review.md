# Backend Boilerplate Review — GNHA Services

**Date:** 2026-03-05
**Reviewer:** Senior Backend Architect
**Stack:** Go 1.26 · Echo · Connect RPC · PostgreSQL · Redis · RabbitMQ · Uber fx

---

## TL;DR

Solid 8/10 boilerplate. Architecture is clean, conventions are enforced, DX tooling is good. The hexagonal + modular monolith combo is production-proven. Main gaps: no module scaffold script, missing graceful degradation, incomplete auth flow, no API versioning strategy despite `/api/v1/` being in goals.

---

## Scorecard (Your 20 Points)

| # | Item | Status | Score |
|---|------|--------|-------|
| 1 | Convention over Configuration | Done well — fx modules, naming, folder structure enforced | 9/10 |
| 2 | Module Template / CLI | **MISSING** — only docs, no `task module:create name=X` | 3/10 |
| 3 | Structure rõ ràng | Excellent — `cmd/`, `internal/modules/`, `internal/shared/`, `pkg/` (implicit) | 9/10 |
| 4 | Enforced Architecture | handler→app→repo enforced by fx DI + package boundaries | 9/10 |
| 5 | Standard Error System | `DomainError` with codes, HTTP mapping, Unwrap() | 9/10 |
| 6 | Standard Response Format | Connect RPC handles this via protobuf, but no REST equivalent | 7/10 |
| 7 | Middleware sẵn | 9 middlewares chained correctly — recovery, requestID, logger, bodyLimit, gzip, security, cors, timeout, rateLimit | 9/10 |
| 8 | Logging chuẩn | slog structured logging, JSON in prod, text in dev | 9/10 |
| 9 | Validation | protovalidate via buf — good for RPC, no domain-level validator lib | 7/10 |
| 10 | Testing pattern | testcontainers for integration, but no test template/helper for new modules | 6/10 |
| 11 | Dev commands | Taskfile with 15+ commands — dev, test, lint, migrate, generate, docker, monitor | 9/10 |
| 12 | Local development | Docker Compose for full stack + Air hot-reload | 9/10 |
| 13 | Code generation | buf + sqlc, but no `task module:create` scaffold | 7/10 |
| 14 | Documentation | `adding-a-module.md`, `architecture.md`, `code-standards.md` | 8/10 |
| 15 | Example module | User module serves as example — fully fleshed out | 9/10 |
| 16 | Guardrails | golangci-lint, lefthook pre-commit/pre-push, CI quality gates | 9/10 |
| 17 | Performance-safe defaults | pgx pool (25 max/5 min), Redis pool (10*CPU), 30s timeout | 8/10 |
| 18 | Observability | `/healthz`, `/readyz`, OTel traces+metrics, SigNoz | 8/10 |
| 19 | API versioning | Proto packages versioned (`user.v1`), but no URL prefix `/api/v1/` | 5/10 |
| 20 | Scalable module design | fx.Module per domain, clean boundaries, event-driven decoupling | 9/10 |

**Overall: 159/200 (79.5%)**

---

## Critical Issues (Fix Before Use)

### 1. No Module Scaffold Script

Your #2 goal (`make module name=user`) doesn't exist. `adding-a-module.md` is 267 lines of manual steps. A new dev will:
- Miss steps
- Copy-paste wrong
- Waste 30+ min per module

**Fix:** Create `scripts/new-module.sh` + `task module:create` that generates:
```
internal/modules/{name}/
├── domain/{name}.go, repository.go, errors.go
├── app/create_{name}.go, get_{name}.go
├── adapters/postgres/repository.go
├── adapters/grpc/handler.go, routes.go, mapper.go
└── module.go
proto/{name}/v1/{name}.proto
db/queries/{name}.sql
```

With proper package names, imports, interface stubs. This is the **single biggest DX win** you're missing.

### 2. Sentinel Error Pointer Comparison Bug

```go
// domain_error.go:39-44
var (
    ErrNotFound      = &DomainError{Code: CodeNotFound, Message: "not found"}
    ErrAlreadyExists = &DomainError{Code: CodeAlreadyExists, Message: "already exists"}
)
```

`errors.Is(err, ErrNotFound)` works via pointer comparison. But `Wrap(CodeNotFound, "user not found", originalErr)` creates a **new** pointer — `errors.Is` won't match the sentinel. Your `create_user.go:40` relies on this:

```go
if err != nil && !errors.Is(err, sharederr.ErrNotFound) {
```

This works only if repo returns the **exact** sentinel pointer. Any wrapping breaks it.

**Fix:** Implement `Is()` method on DomainError that compares by Code:
```go
func (e *DomainError) Is(target error) bool {
    t, ok := target.(*DomainError)
    return ok && e.Code == t.Code
}
```

### 3. Auth Flow Incomplete

- Refresh token rotation: schema exists (`refresh_tokens` table), handler doesn't
- Login endpoint: missing entirely (no AuthService RPC handler)
- Token blacklisting on logout: Redis setup exists, no endpoint
- API key CRUD: schema exists, no endpoints

A boilerplate without working login/logout/refresh is incomplete. Dev will build this from scratch anyway.

### 4. Race Condition in Email Uniqueness Check

```go
// create_user.go:39-45
existing, err := h.repo.GetByEmail(ctx, cmd.Email)
if existing != nil {
    return nil, domain.ErrEmailTaken
}
// ... window for concurrent insert with same email ...
if err := h.repo.Create(ctx, user); err != nil {
    return nil, fmt.Errorf("creating user: %w", err)
}
```

Between `GetByEmail` and `Create`, another request can insert the same email. You have a UNIQUE constraint in DB (good), but the app error returned will be a generic postgres error, not `ErrEmailTaken`.

**Fix:** Remove the pre-check. Use `ON CONFLICT` or catch the unique violation from pgx and map to `ErrEmailTaken`.

---

## Important Issues (Fix Soon)

### 5. API Versioning Strategy Missing

Goal #19 says `/api/v1/` but Connect RPC routes are `/user.v1.UserService/GetUser`. No URL prefix grouping. When you add v2, how do you route? How do clients know which version? No gateway/proxy config.

**Fix:** Either:
- Accept Connect RPC's built-in package versioning (`user.v1` vs `user.v2`) — document this as your strategy
- Or add a URL prefix wrapper: `e.Group("/api/v1", ...)` mounting Connect handlers

### 6. No Graceful Degradation

If Redis is down → rate limiter panics → all requests fail. If RabbitMQ is down → event publishing fails → mutations log error but user creation succeeds with silent audit gap.

**Fix:**
- Rate limiter: fallback to in-memory when Redis unavailable
- Event bus: consider outbox pattern or at minimum, document that events are fire-and-forget

### 7. /metrics Endpoint Missing

Goal #18 mentions `/metrics` but it doesn't exist. You have OTel meter provider pushing to OTLP endpoint, but no Prometheus-compatible `/metrics` scrape endpoint.

**Fix:** Add `promhttp.Handler()` or document that you're push-only via OTLP.

### 8. Connect RPC Response ≠ Standard JSON Format

Goal #6 wants `{"data": {}, "error": null, "meta": {}}`. Connect RPC returns protobuf-serialized JSON which looks like `{"user": {"id": "...", "name": "..."}}`. These are incompatible.

**Decision needed:** Either:
- Accept Connect RPC's native format (recommended — it's the standard for gRPC/Connect clients)
- Or add a REST gateway layer that wraps responses (adds complexity)

### 9. No Transaction Support in App Layer

`CreateUserHandler` does: repo.Create → bus.Publish. If you need multi-repo writes (e.g., create user + create initial settings), there's no transaction abstraction.

**Fix:** Add a `UnitOfWork` or `TxFunc` pattern:
```go
type TxRunner interface {
    RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}
```

### 10. Testing Gaps

- No test files exist yet (testutil/ has helpers but no actual tests)
- No test template for new modules
- No table-driven test examples
- Integration test build tag `integration` defined but no test files use it

---

## Minor Issues (Nice to Have)

### 11. `os.Exit(1)` in Goroutine

```go
// main.go:68-69
if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
    os.Exit(1)
}
```

`os.Exit` skips all deferred functions and fx shutdown hooks. OTel flush, DB pool close, Redis close — all skipped.

**Fix:** Send error to a channel, let fx handle shutdown.

### 12. Hardcoded Rate Limit

```go
e.Use(RateLimit(rdb, 100, time.Minute))
```

100 req/min for all endpoints. Auth endpoints should have different limits than data endpoints.

**Fix:** Make configurable via env, support per-route overrides.

### 13. No Request/Response Logging Sampling

In production, logging every request at info level with body details creates noise and cost. Consider sampling or debug-level for successful requests.

### 14. Missing `.env.example` Sync Check

No CI step to verify `.env.example` matches actual config struct fields. Easy to add a field to config.go and forget `.env.example`.

### 15. Dockerfile Health Check Uses curl

```dockerfile
HEALTHCHECK CMD curl http://localhost:8080/healthz
```

curl might not be in the Alpine image. Use `wget -q --spider` or build a tiny Go healthcheck binary.

---

## Architecture Strengths (What's Done Right)

1. **fx DI over manual wiring** — Compile-time validation, lifecycle hooks, module composition. Excellent choice.
2. **sqlc over ORM** — Type-safe SQL, no reflection, no N+1 surprise. Best choice for Go.
3. **Encapsulated domain entities** — Private fields + factory + getters. Prevents invalid state. Textbook DDD.
4. **Event-driven decoupling** — Audit and notification as subscribers, not coupled to user handler. Clean.
5. **Cursor-based pagination** — Correct choice over offset pagination for scalability.
6. **testcontainers over mocks** — Real infra in tests. Higher confidence, no mock maintenance.
7. **Lefthook + golangci-lint** — Catches issues before commit. Good guardrails.
8. **Connect RPC** — HTTP/1.1 compatible gRPC. Frontend can call with fetch(). Smart choice.
9. **Soft deletes + audit trail** — Data preservation built-in from day one.
10. **Modular monolith** — Can extract to microservices by swapping fx modules for network calls.

---

## Recommendations for Boilerplate Improvement

### Priority 1 — Must Have

| Action | Effort | Impact |
|--------|--------|--------|
| Add `task module:create` scaffold script | 2-3h | Massive DX win |
| Fix `DomainError.Is()` for code comparison | 15min | Prevents subtle bugs |
| Implement basic auth flow (login/logout/refresh) | 4-6h | Boilerplate completeness |
| Add DB unique constraint error mapping | 30min | Fixes race condition |
| Write at least 1 unit + 1 integration test as template | 1-2h | Testing pattern example |

### Priority 2 — Should Have

| Action | Effort | Impact |
|--------|--------|--------|
| Document API versioning strategy | 30min | Clarity |
| Add transaction/UnitOfWork pattern | 1-2h | Multi-repo operations |
| Redis fallback for rate limiter | 1h | Resilience |
| Add `/metrics` or document push-only OTel | 30min | Observability completeness |
| Configurable rate limits per route group | 1h | Flexibility |

### Priority 3 — Nice to Have

| Action | Effort | Impact |
|--------|--------|--------|
| Replace `os.Exit(1)` with channel-based shutdown | 30min | Clean shutdown |
| Add request logging sampling for prod | 1h | Cost reduction |
| CI check for `.env.example` sync | 30min | DX safety net |
| Add `HOW_TO_ADD_FEATURE.md` (goal #14) | 1h | Onboarding |

---

## Final Verdict

This is a **well-architected Go boilerplate** with strong foundations. The hexagonal architecture is properly layered, not over-engineered. fx DI is the right choice. Connect RPC is modern and practical.

**Main weakness:** It's more of a "reference project" than a "boilerplate" right now. A true boilerplate needs the scaffold script (`task module:create`) to be the star feature — that's what makes it "convention over configuration" instead of "copy-paste from docs."

Fix the 5 critical items, add the scaffold script, and this becomes a genuinely excellent Go boilerplate that saves hours per module and enforces consistency by default.

---

## Unresolved Questions

1. Connect RPC vs REST: Will you ever need a pure REST API alongside Connect? If yes, need gateway strategy.
2. Multi-tenancy: Any plans? Would affect DB queries, middleware, config significantly.
3. Background jobs: Cron module exists but empty. What jobs are planned? Worker pool needed?
4. File uploads: No storage abstraction. S3/MinIO adapter needed?
5. Elasticsearch: Configured in docker-compose but unused. Search feature planned?
