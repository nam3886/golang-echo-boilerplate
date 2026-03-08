# Code Reviewer Memory - gnha-services

## Project Structure
- Go 1.26.0 modular monolith using Fx DI, Echo HTTP, Connect RPC, pgx+sqlc, Watermill+RabbitMQ
- Hexagonal architecture: `domain/ -> app/ -> adapters/{postgres,grpc}`
- Generated code in `gen/sqlc/` and `gen/proto/`
- Migrations in `db/migrations/` (3 files: initial, pagination index, role constraint fix)
- CI/CD: `.gitlab-ci.yml` (4 stages: quality, test, build, deploy)
- Task runner: `Taskfile.yml` (go-task)
- Sentinel errors: shared pkg uses constructor funcs (ErrNotFound()) copying unexported templates
- EventPublisher interface in events/publisher.go decouples app from EventBus
- Event contracts in `internal/shared/events/contracts/` (shared types/topics, no cross-module imports)
- Domain re-exports contracts via type aliases (e.g. `type UserCreatedEvent = contracts.UserCreatedEvent`)
- Auth blacklist centralized in auth/blacklist.go with shared prefix constant
- RBAC interceptor uses exact procedure path constants from generated code
- Test stubs consolidated in testutil/stubs.go (StubHasher, NoopPublisher, CapturingPublisher, FailPublisher)

## Key Patterns
- Domain entities use unexported fields + getters + `Reconstitute()` for persistence hydration
- DomainError.Is() matches on Code field, not pointer identity
- SubscriberFactory creates per-handler AMQP subscribers (each gets own queue via GenerateQueueNameTopicNameWithSuffix)
- HandlerRegistration structs collected via Fx `group:"event_handlers"` tag, flattened in NewRouter
- Cursor-based pagination with base64-encoded JSON cursors (keyset: created_at DESC, id DESC)
- Auth middleware on route groups, not global; RBAC with PermUserRead/Write/Delete
- Closure-based `Update(ctx, id, func(*User) error)` for transactional UoW in repos
- CORS guard: AllowCredentials disabled when origins contain "*"
- Audit idempotency: uses Watermill msg UUID as PK with ON CONFLICT DO NOTHING
- Swagger CSP override per-route (not global) allowing unpkg.com CDN
- Config URL validation via url.Parse checking Scheme+Host non-empty

## Remaining Issues (updated 2026-03-08 round 3)
### CRITICAL -- None
### HIGH (open)
- H-1: DLQ routing key may be empty with fanout exchange -- dead-lettered msgs could be silently dropped. Need to verify Watermill AMQP adapter routing key behavior or set explicit x-dead-letter-routing-key per topic.
- I-12: SoftDelete not transactional -- concurrent CreateUser can race (postgres/repository.go:178)
- auth/ package ZERO unit tests
### MEDIUM (open)
- M-2: No-op update still hits DB (SELECT FOR UPDATE + UPDATE) before event suppression check
- M-4: Production deploy has no migration step in CI/CD or docker-compose
- M-14: 30s global timeout partial-write risk (documented with WARNING comment but not fixed)
- Three near-identical audit handlers (DRY opportunity)
- Swagger CDN no SRI hashes (pinned version mitigates)
### LOW (open)
- isPermanentSMTPError uses string matching instead of *textproto.Error
- StubHasher.Verify always true; linear backoff in retry
- Zero tests: auth, connectutil, events, retry, audit, notification
- Subscriber leak on factory error during startup (minor, startup-only)

## Overall Score: 8.5/10 (as of 2026-03-08 round 3)
- Up from 7.5 due to: I-9 fixed (subscriber fanout), I-8 fixed (email sanitization), I-10 fixed (Swagger CSP), Alpine/Redis/RabbitMQ hardened
- Top priority: verify H-1 (DLQ routing key), then I-12 (SoftDelete race)

## Resolved Issues (this round)
- I-9: Shared subscriber round-robin -> SubscriberFactory per-handler queues
- I-8: Email CRLF injection -> mail.ParseAddress + sanitize()
- I-10: CSP blocks Swagger -> per-route CSP override
- I-11: No-op update event -> suppressed (DB still hit, tracked as M-2)
- M-12: No URL validation -> validateURL() added
- M-15: Alpine 3.19 EOL -> 3.21
- M-16: Redis no persistence -> --appendonly yes
- M-17: RabbitMQ management in prod -> rabbitmq:3-alpine
- L-10: 429 as CodeInternal -> CodeResourceExhausted
- semconv deprecated -> v1.27 DeploymentEnvironmentName
- mailpit/golangci-lint unpinned -> pinned versions
- Router context.Background() -> cancellable context with Fx lifecycle
- Cron module removed (was empty scaffolding)

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
- Search: `internal/modules/user/adapters/search/`
- Testutil: `internal/shared/testutil/`
- Errors: `internal/shared/errors/domain_error.go`
- Docker: `Dockerfile`, `deploy/docker-compose.yml`, `deploy/docker-compose.dev.yml`
- CI: `.gitlab-ci.yml`
