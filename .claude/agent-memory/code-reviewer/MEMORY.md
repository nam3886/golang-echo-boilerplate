# Code Reviewer Memory - golang-echo-boilerplate

## Project Structure
- Go 1.26.0 modular monolith using Fx DI, Echo HTTP, Connect RPC, pgx+sqlc, Watermill+RabbitMQ
- Hexagonal architecture: `domain/ -> app/ -> adapters/{postgres,grpc}`
- Generated code in `gen/sqlc/`, `gen/proto/`, `gen/ts/`, `gen/openapi/`
- Migrations in `db/migrations/` (3 files: initial, pagination index, role constraint fix)
- CI/CD: `.gitlab-ci.yml` (4 stages: quality, test, build, deploy)
- Task runner: `Taskfile.yml` (go-task)
- Sentinel errors: shared pkg uses constructor funcs (ErrNotFound(), ErrNoChange()) copying unexported templates
- EventPublisher interface in events/publisher.go decouples app from EventBus
- Event contracts in `internal/shared/events/contracts/` (shared types/topics, no cross-module imports)
- Domain re-exports contracts via type aliases
- Auth blacklist centralized in auth/blacklist.go with shared prefix constant
- RBAC interceptor uses exact procedure path constants; fail-closed design
- Test stubs consolidated in testutil/stubs.go + testutil/helpers.go (Ptr[T])

## Key Patterns
- Domain entities use unexported fields + getters + `Reconstitute()` for persistence hydration
- DomainError.Is() matches on Code field, not pointer identity
- ErrNoChange signals no-op updates: app returns it, repo intercepts to skip SQL UPDATE + commit read-tx
- Closure-based `Update(ctx, id, func(*Entity) error)` for transactional UoW in repos
- SoftDelete uses single `UPDATE ... RETURNING` (no TOCTOU race)
- Auth middleware on route groups; all RBAC via RBACInterceptor per procedure (no RequirePermission on group)
- Cursor-based pagination with base64-encoded JSON cursors (keyset: created_at DESC, id DESC)
- Cursor validation: rejects zero time or nil UUID
- Config String() uses strings.Builder (not positional fmt.Sprintf)
- Exponential backoff in retry.Connect: `1<<uint(i)` seconds, capped at 30s
- isPermanentSMTPError uses textproto.Error type assertion (not string matching)
- OTel tracer/metrics: empty OTLPEndpoint returns no-op provider; shared resource in resource.go
- Recovery middleware truncates panic value to 200 chars (PII protection)
- Audit subscriber logs msg_id on unmarshal error (not raw payload)
- Import alias: `sharederr` for shared/errors everywhere (domainerr fully eliminated)
- Password hashing: argon2id (not bcrypt), maxPasswordBytes=72, Verify silently returns false for oversized
- Auth blacklist: fail-closed on Redis error (rejects token)
- Rate limiter: fail-open on Redis error, IP-based (user-keying impossible before Auth)
- Health endpoints (/healthz, /readyz) registered BEFORE middleware chain
- Middleware order: OTel > Recovery > RequestID > RateLimit > Logger > BodyLimit > Gzip > Security > CORS > Timeout
- Per-handler subscriber queues via SubscriberFactory (prevents round-robin message loss)
- DLQ uses separate AMQP connection at startup for exchange/queue declaration
- Event flow: EventBus.Publish -> AMQP fanout -> per-handler queues (SubscriberFactory) -> Watermill router -> handlers
- Watermill router middleware: otelExtract > Recoverer > Retry(3x, 1s initial, 2x multiplier, 10s max, 0.5 randomization)
- Subscribers use msg.Context() for DB/ES/SMTP operations (context propagation); audit slog calls miss this
- Notification uses html/template (auto-escapes HTML, prevents XSS in emails)
- SMTP sender sanitizes CRLF in all header values, uses mime.QEncoding for subject
- Notification only handles user.created (welcome email); no updated/deleted handlers (intentional MVP)
- Audit handler: idempotent via msg UUID as PK + ON CONFLICT DO NOTHING
- Notification handler: permanent SMTP 5xx errors acked (not retried), transient errors returned for retry
- Fx shutdown order: router.Close() (stops consuming) -> publisher.Close() (correct order via reverse invoke)
- Integration tests: testcontainers for Postgres/Redis/ES/RabbitMQ, CI env var fallback
- Config validation: APP_ENV whitelist, JWT_SECRET >=32 chars, URL scheme+host, DBMinConns<=DBMaxConns

## Remaining Issues (updated 2026-03-10 round 7 deep dive)
### CRITICAL
- C-1: Rate limiter user-keying is dead code (Auth runs after RateLimit in chain) -- WONTFIX (by design)
### HIGH
- H-EVT-1: OTel trace extraction on subscribe — FIXED (otelExtractMiddleware using MapCarrier)
- H-EVT-2: Watermill Retry Multiplier/MaxInterval — FIXED (Multiplier:2, MaxInterval:10s)
- H-R7-1: repo.Create password preservation — FIXED (save pwd before overwrite, Reconstitute with it)
- H-R7-2: repo.Update password preservation — FIXED (same pattern as Create)
- H-SCAF-2: Scaffold Update proto template missing validation on optional name field — FIXED
- H-SCAF-6: Scaffold mapper WARNING about per-query variants — FIXED (prominent WARNING block)
- H-DOC-7: code-standards.md test example uses wrong handler constructor (NoopPublisher directly vs EventBus wrapper)
- H-INFRA-2: search.NewClient returns (nil, nil) -- ACCEPTED (documented + Enabled() helper)
- H-MW-1: Global 30s ContextTimeout may cancel context mid-transaction — DOCUMENTED (warning comment)
### MEDIUM (remaining)
- M-R7-1: mail.ParseAddress accepts RFC 5322 display-name format; stored email correct but API behavior surprising
- M-R7-2: No name length validation in domain entity (DB VARCHAR(255) is only guard)
- M-R7-3: DomainError.Is code-only matching lacks identity-matching test/documentation
- M-2: No login/logout endpoint despite full auth infrastructure
- M-7: Swagger discoverSpecs silently swallows filepath.Walk errors
- M-9: No RabbitMQ health check in /readyz
- M-10: Event contracts missing version field
- M-14: File naming conflict: development-rules.md kebab-case needs Go exemption
- M-NEW-15: No mapper_test template (user module has 115-line mapper_test.go)
- M-NEW-17: No golden-file test for scaffold CLI
- M-INFRA-3: CapturingPublisher not thread-safe (test-only, low risk)
- M-INFRA-7: No test for GenerateRefreshToken
- M-INFRA-10: No DB connection pool stats exposed to OTel
- M-DOM-3: No ChangePassword method on User entity
- M-CFG-3: DB MaxConnIdleTime hardcoded 30m, not configurable via env
- M-DC-1: ES version mismatch: dev compose 8.13.0 vs testutil 8.17.0
- M-TEST-4: NewTestRabbitMQ doesn't check RABBITMQ_URL env var (unlike Postgres/Redis)
- M-SCAF-3: Multi-word module names create package/directory name mismatch (root: main.go L241 + module.tmpl L1)
- M-SCAF-4: Scaffold rollback doesn't clean intermediate directories from MkdirAll
- M-SCAF-5: Scaffold RBAC injection assumes Connect naming convention (fragile)
- M-SCAF-7: Missing handler_test.go and mapper_test.go templates for gRPC adapter
- M-SCAF-9: domain_test ChangeName time assertion uses Before (should be After)
- M-SCAF-12: Multi-word proto go_package produces underscore in package name
- M-CFG-4: Config Load() does not validate negative DB pool sizes
- M-CFG-5: Config String() omits RequestTimeout and DBMaxConnLifetime
- M-EVT-5: No unit tests for EventBus.Publish or SubscriberFactory.Create
- M-TEST-5: Integration test setupRepo() creates new testcontainer per test function (slow)
- M-TEST-6: Blacklist integration test uses time.Sleep(2s) for TTL expiry (flaky)
- M-ENV-1: No production .env example showing Redis password requirement
- M-R8-1: Audit tests have no DB-error-path coverage (all pass nil execErr)
- M-R8-2: Audit subscriber slog calls lack context (no OTel trace correlation)
- M-R8-3: Audit/notification tests use non-UUID msg ID ("test-uuid"), idempotency untested
- M-R8-6: No template-rendering-failure test in notification
### MEDIUM (round 8b — proto/SQL/deploy)
- H-R8b-1: OpenAPI spec empty — openapiv2 plugin needs google.api.http annotations (Connect uses different routes)
- M-R8b-2: SQL RETURNING clauses include unused deleted_at in Create/Update
- M-R8b-4: Swagger static path relative — breaks in Docker (dev-only, low impact)
- M-R8b-5: Dockerfile goose install after COPY invalidates layer cache
- M-R8b-7: Redis healthcheck env var expansion fragile in production compose
- M-R8b-8: Seed cmd requires full config.Load() but only uses Postgres
- M-R8b-10: No updated_at trigger (app-layer only, explicit but fragile)
### LOW
- Non-UUID strings as IDs in unit tests bypass parseUserID
- Swagger UI CDN lacks SRI integrity hashes
- DLQ declaration opens/closes separate AMQP connection at startup (one-time, startup only)
- Scaffold does not validate plural against reserved words
- EventBus.Publish topic param is untyped string (typo-prone)
- Scaffold ChangeName test uses weak time assertion (>=, not >)

## Test Coverage (2026-03-10, updated round 28)
| Package | Coverage |
|---------|----------|
| user/app | 93.7% |
| user/domain | 79.2% |
| user/adapters/grpc | 65.7% |
| shared/middleware | 55.0% |
| shared/errors | 54.2% |
| shared/auth | 50.7% |
| shared/config | 45.6% |
| audit | >0% (7 tests) |
| notification | >0% (5 tests) |
| shared/retry | 0% (needs unit tests) |
| shared/connectutil | 0% (needs tests) |

## Review History (37 reports, 2026-03-10)
See `review-history.md` for full report index.
- Latest: Round 8b — proto/SQL/deploy/swagger/seed/go.mod
- Report: `plans/reports/code-reviewer-260310-round8-proto-sql-deploy.md`
- 1 HIGH (empty OpenAPI spec), 6 MEDIUM, 4 LOW. H-R8-2/M-R8-1 verified OK.

## Key Verified Facts (round 8b)
- Repo Update() handles 23505 uniqueness violation at line 177 -> ErrEmailTaken
- Protovalidate skips unset optional fields by default (connectrpc/validate v0.6.0)
- Swagger disabled in production (cfg.AppEnv == "production" guard)
- Production compose does NOT expose infra ports (correct)
- sqlc nullable timestamptz correctly falls back to pgtype.Timestamptz

## Docs Accuracy Status (2026-03-10 fresh review 31)
- error-codes.md: VERIFIED ACCURATE
- authentication.md: VERIFIED ACCURATE
- event-subscribers.md: MOSTLY ACCURATE
- testing-strategy.md: MOSTLY ACCURATE
- architecture.md: VERIFIED ACCURATE (middleware order, request flow)
- code-standards.md: BUG — test example line 683 uses wrong constructor (H-DOC-7)
- rbac.md: VERIFIED ACCURATE
- CLAUDE.md: VERIFIED ACCURATE
- adding-a-module.md: VERIFIED ACCURATE (27 files, scaffold steps correct)

## Overall Score: 9.0/10 (review 35) | All HIGH issues now FIXED or ACCEPTED

## File Locations
- Entry: `cmd/server/main.go`
- Auth: `internal/shared/auth/{jwt,password,context,blacklist}.go`
- Middleware: `internal/shared/middleware/`
- User module: `internal/modules/user/`
- Events infra: `internal/shared/events/{bus,subscriber,dlq,module,publisher}.go`
- Event contracts: `internal/shared/events/contracts/user_events.go`
- Config: `internal/shared/config/config.go`
- Audit: `internal/modules/audit/`
- Notification: `internal/modules/notification/`
- Testutil: `internal/shared/testutil/`
- Errors: `internal/shared/errors/domain_error.go`
- Scaffold: `cmd/scaffold/templates/`
- CI: `.gitlab-ci.yml`
