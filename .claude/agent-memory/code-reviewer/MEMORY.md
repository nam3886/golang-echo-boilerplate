# Code Reviewer Memory - gnha-services

## Project Structure
- Go 1.26.0 modular monolith using Fx DI, Echo HTTP, Connect RPC, pgx+sqlc, Watermill+RabbitMQ
- Hexagonal architecture: `domain/ -> app/ -> adapters/{postgres,grpc}`
- Generated code in `gen/sqlc/` and `gen/proto/`
- Migrations in `db/migrations/`
- CI/CD: `.gitlab-ci.yml` (4 stages: quality, test, build, deploy)
- Task runner: `Taskfile.yml` (go-task)
- ~3,513 Go LOC in internal/ + cmd/ (44 hand-written files, excludes gen/)
- No auth proto exists; no login endpoint

## Key Patterns
- Domain entities use unexported fields + getters + `Reconstitute()` for persistence hydration
- Sentinel domain errors (`*DomainError`) mapped to HTTP status + Connect RPC codes
- Event bus publishes to RabbitMQ; audit + notification modules subscribe
- Cursor-based pagination with base64-encoded JSON cursors (keyset: created_at DESC, id DESC)
- Auth middleware on route groups, not global; RBAC applied (PermUserRead) but too coarse for write/delete
- Closure-based `Update(ctx, id, func(*User) error)` for transactional UoW in repos
- Event handlers registered via Fx `group:"event_handlers"` tag

## Known Issues (updated 2026-03-06 FINAL review -- all 10 fixes verified)
### CRITICAL
- None
### HIGH
- None (H-1 RBAC partially fixed -- RequirePermission applied but too coarse, see Medium)
### MEDIUM
- N-2: RBAC applies only PermUserRead to all endpoints; write/delete need PermUserWrite/PermUserDelete
- M-2: Audit ActorID falls back to EntityID (system ops) -- by design
- M-4: ListUsers returns password hashes through call chain (proto mapper strips them)
- M-5: Cron scheduler starts with zero registered jobs (by-design for boilerplate)
- create_user_test.go missing error-path tests (invalid role, hash failure, repo failure)
- os.Exit(1) in Echo start goroutine bypasses Fx shutdown
- mp.Shutdown error silently discarded in shared/module.go
- Audit module creates its own sqlcgen.Queries (could conflict with other modules)
- No login endpoint -- boilerplate cannot be tested end-to-end
### LOW
- SanitizeHeader in request_log.go is dead code
- repository.go at 222 lines (slightly over 200-line guideline)
- Cron Stop() doesn't wait for running jobs (should use context-aware stop)
- PII (email, name) persisted in audit trail via event payload -- GDPR consideration
### RESOLVED (all 10 requested fixes confirmed)
- RBAC on routes: RequirePermission(PermUserRead) added
- AMQP shutdown: registerAMQPShutdown closes publisher+subscriber
- X-Request-ID: >128 chars rejected
- Swagger XSS: html.EscapeString on all interpolated values
- Watermill router: cancellable context via WithCancel
- Cron AddJob: returns error from cron.AddFunc
- OTel WithInsecure: conditional on IsDevelopment()
- DB shutdown: pool.Close() + rdb.Close() on Fx OnStop
- Readiness probe: /readyz checks DB + Redis
- Safe UUID: zero MustParse in internal/

## Overall Score: 8.3/10 (as of 2026-03-06 FINAL)
- Architecture: 9/10, Security: 8/10, Code Quality: 8.5/10, DX: 8/10
- Ship-ready as boilerplate. Main gap: RBAC granularity for write/delete ops.

## File Locations
- Entry: `cmd/server/main.go`
- Auth: `internal/shared/auth/{jwt,password,context}.go`
- Middleware: `internal/shared/middleware/`
- User module: `internal/modules/user/`
- Events: `internal/shared/events/`
- Config: `internal/shared/config/config.go`
- Audit: `internal/modules/audit/`
- Notification: `internal/modules/notification/`
- Cron: `internal/shared/cron/`
- Testutil: `internal/shared/testutil/`
