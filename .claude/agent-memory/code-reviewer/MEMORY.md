# Code Reviewer Memory - gnha-services

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
- Import alias: `sharederr` for shared/errors in repos/templates; `domainerr` still used in middleware
- Password hashing: argon2id (not bcrypt), maxPasswordBytes=72, Verify silently returns false for oversized
- Auth blacklist: fail-closed on Redis error (rejects token)
- Rate limiter: fail-open on Redis error, IP-based (user-keying impossible before Auth)
- Health endpoints (/healthz, /readyz) registered BEFORE middleware chain
- Middleware order: OTel > Recovery > RequestID > RateLimit > Logger > BodyLimit > Gzip > Security > CORS > Timeout
- Per-handler subscriber queues via SubscriberFactory (prevents round-robin message loss)
- DLQ uses separate AMQP connection at startup for exchange/queue declaration

## Remaining Issues (updated 2026-03-10 post-fix)
### CRITICAL
- C-1: Rate limiter user-keying is dead code (Auth runs after RateLimit in chain) -- WONTFIX (by design)
### HIGH (all fixed 2026-03-10)
- ~~H-DOC-1 thru H-DOC-6~~: FIXED — docs accuracy (architecture.md, CLAUDE.md, code-standards.md, rbac.md)
- ~~H-SEC-1~~: FIXED — JWT time.Now() single call
- ~~H-SEC-2~~: FIXED — CORS localhost production warning
- ~~H-ERR-1~~: FIXED — errors.Is() for http.ErrServerClosed
- ~~H-ARCH-1~~: FIXED — audit uses shared contracts
- ~~H-DX-1~~: FIXED — removed dead RequirePermission/RequireRole
- ~~H-CI-1~~: FIXED — govulncheck in CI
- ~~H-CI-2~~: FIXED — 50% coverage threshold gate
- ~~H-SCAF-1~~: FIXED — scaffold rollback on partial failure
- H-INFRA-2: search.NewClient returns (nil, nil) -- ACCEPTED (documented + Enabled() helper)
- H-MW-1: Global 30s ContextTimeout may cancel context mid-transaction — DOCUMENTED (warning comment)
### MEDIUM (remaining)
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
### LOW
- Non-UUID strings as IDs in unit tests bypass parseUserID
- Swagger UI CDN lacks SRI integrity hashes
- DLQ declaration opens/closes separate AMQP connection at startup
- code-standards.md test examples use testify but codebase uses stdlib testing
- Scaffold does not validate plural against reserved words
- EventBus.Publish topic param is untyped string (typo-prone)

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

## Review History (30 reports, 2026-03-10)
See `review-history.md` for full report index.
- Latest: Consolidated deep review (30) — 4 parallel reviewers
- Report: `plans/reports/code-reviewer-260310-0847-consolidated-deep-review.md`
- Sub-reports: architecture-dx-consistency, docs-accuracy-onboarding, shared-infra-security, scaffold-codegen-cicd
- 57 issues total: 0C, 15H, 29M, 13L
- Key themes: docs accuracy (6H), CI gaps (2H), scaffold gaps (2H), infra (5H)

## Docs Accuracy Status (2026-03-10 post-fix)
- error-codes.md: VERIFIED ACCURATE
- authentication.md: VERIFIED ACCURATE
- event-subscribers.md: MOSTLY ACCURATE
- testing-strategy.md: MOSTLY ACCURATE
- architecture.md: FIXED — correct middleware order + request flow
- code-standards.md: FIXED — per-query mapper pattern + constraint name check added
- rbac.md: FIXED — clarified only RBACInterceptor enforces permissions
- CLAUDE.md: FIXED — 8 commands added, fixtures corrected, 9/9 docs listed

## Overall Score: 9.0/10 (post-fix) | 15 HIGH issues resolved

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
