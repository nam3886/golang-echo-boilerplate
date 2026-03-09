# Project Changelog

All notable changes to GNHA Services are documented here.

## [Unreleased]

### Boilerplate Verified Fixes (2026-03-06)

**Summary:** Implemented 6 critical and important fixes verified through cross-referenced review sessions. Fixed database constraints, architecture violations, domain validation, and soft-delete compatibility.

#### Fixed
- **DB CHECK Constraint** — Added missing `viewer` role to users table constraint (migration 00003)
- **Architecture Violation** — Replaced `appmw.GetClientIP` with `netutil.GetClientIP` in `update_user.go` and `delete_user.go` to eliminate app→adapter dependency violation
- **Misleading ListResult Field** — Removed `Total` field that returned page size instead of actual DB count (not used by any handler)
- **Password Validation** — Added 8-character minimum length check in domain layer (`password.go`)
- **Email Validation** — Added email format validation via `net/mail.ParseAddress` in `NewUser()` with new `ErrInvalidEmail` error type
- **Soft-Delete Index** — Fixed unique index to partial `WHERE deleted_at IS NULL`, allowing email re-registration after soft-delete

#### Files Modified
- `db/migrations/00003_fix_role_constraint.sql` — role constraint + partial unique index
- `internal/shared/auth/password.go` — min password length
- `internal/modules/user/domain/user.go` — email validation
- `internal/modules/user/domain/errors.go` — ErrInvalidEmail
- `internal/modules/user/app/update_user.go` — netutil import
- `internal/modules/user/app/delete_user.go` — netutil import
- `internal/modules/user/adapters/postgres/repository.go` — removed Total field
- `internal/modules/user/domain/repository.go` — removed Total field
- `internal/modules/user/app/list_users.go` — removed Total mapping
- Tests updated in `user_test.go` for new ErrInvalidEmail error

#### Verification
- All 6 fixes verified against actual codebase from independent review sessions
- `go build ./...` passes
- All tests pass including new email validation test

---

### Module Scaffold Generator (2026-03-05)

**Summary:** Implemented CLI tool for scaffolding complete CRUD modules. Automates creation of 27 files (proto, migrations, SQL queries, domain, app, adapters, tests) following hexagonal architecture patterns.

#### Added
- **cmd/scaffold/main.go** — CLI tool for module scaffolding with `-name` and optional `-plural` flags
- **cmd/scaffold/templates/** — 19 Go templates covering all module layers:
  - Proto: service, messages, enums
  - Database: migration, SQL queries
  - Domain: entity, repository interface, errors, events
  - Application: create/get/list/update/delete handlers
  - Adapters: Postgres repository, gRPC handler, routes, mappers
  - Module: fx.Module definition
  - Tests: unit test scaffold with mockgen directives
- **Taskfile.yml** — `task module:create name=<name>` command for single-step scaffolding
- **docs/adding-a-module.md** — Quick Start section showing generator usage (previously manual steps only)

#### Features
- Generates 27 files matching user module patterns
- Custom plural naming support: `task module:create name=category plural=categories`
- Auto-runs code generation after scaffold (buf + sqlc)
- Proper mockgen directives in repository interfaces
- Complete pagination, error handling, and event publishing templates
- Full test boilerplate with real infrastructure (testcontainers)

#### Closes Gap
- **P0 from Boilerplate Review:** Module scaffold script now available
- Reduced module creation time from 30+ min (manual) to <10 sec (scaffold + customize)
- Ensures all new modules follow hexagonal architecture patterns consistently

---

### Boilerplate YAGNI Fixes (2026-03-05)

**Summary:** Removed half-implemented auth scaffold (proto, migrations, generated code, apikey.go). Fixed CreateUser UUID mismatch between domain and database. Added mockgen testing infrastructure and integration test scaffolding.

#### Added
- **Mock Generation Infrastructure** — Added `mockgen` tool setup to dev environment; created `task generate:mocks` for automatic mock generation via `//go:generate` directives
- **Testing Conventions Documentation** — Added mockgen usage patterns and mock generation examples to code-standards.md
- **Unit Test Scaffolding** — Created test structure with gomock Controller and mock repository examples
- **Integration Test Framework** — Prepared integration test base classes and testcontainers setup

#### Removed
- **Half-Implemented Auth Service** — Removed proto definitions, migrations, SQL queries, generated code, and apikey.go that were never integrated
- **Unused Base Model Auth Fields** — Cleaned up base model that had unused auth-related schema

#### Fixed
- **CreateUser UUID Mismatch** — Fixed domain UUID not being passed to database INSERT statement; now correctly generates UUID in domain and persists to DB
- **Module Pattern Consistency** — Verified all modules follow hexagonal architecture pattern; updated adding-a-module.md with correct implementation examples

#### Changed
- **Taskfile Tools** — Added `go install github.com/golang/mock/mockgen@latest` to dev:tools task
- **Code Generation Task** — Enhanced `generate` task to run `generate:mocks` automatically

#### Documentation Updates
- Updated `docs/code-standards.md` with mockgen setup, mock generation examples, and corrected test patterns
- Verified `docs/adding-a-module.md` reflects actual implementation (includes mockgen directives, correct UUID patterns)
- All documentation examples now match actual codebase patterns

---

### Boilerplate Review (2026-03-05)

**Score: 16/20 đạt/vượt | 2 đạt một phần | 2 thiếu**

| # | Tiêu chí | Status | Ghi chú |
|---|----------|--------|---------|
| 1 | Convention over Configuration | STRONG | Fx DI, buf.validate, sqlc codegen, module structure chuẩn |
| 2 | Module Template (CLI/script) | **COMPLETE** | `task module:create name=X` scaffolds 27 files in <10 sec |
| 3 | Structure rõ ràng | STRONG | cmd/, internal/shared/, internal/modules/, db/, proto/, gen/, deploy/ |
| 4 | Enforced Architecture | STRONG | domain→app→adapters enforce qua Go packages + interfaces |
| 5 | Standard Error System | STRONG | DomainError + ErrorCode enum + HTTP mapping + module errors |
| 6 | Standard Response Format | PARTIAL | Protobuf response (Connect RPC) — không có REST wrapper `{data,error,meta}` |
| 7 | Middleware sẵn | EXCELLENT | 10 middleware: recovery, request-id, logger, body-limit, gzip, security-headers, CORS, timeout, rate-limit, auth+RBAC |
| 8 | Logging chuẩn | STRONG | slog stdlib, JSON prod / text dev, structured fields |
| 9 | Validation | STRONG | buf/validate declarative, Connect interceptor — thiếu custom business validator |
| 10 | Testing pattern | STRONG | testcontainers real infra, mock repos, fixtures — thiếu test template |
| 11 | Dev commands | STRONG | Taskfile: dev, test, lint, migrate, build, seed, check |
| 12 | Local development | EXCELLENT | docker-compose: PG16, Redis7, RabbitMQ, ES8, Mailpit + air hot reload |
| 13 | Code generation | STRONG | buf + sqlc + CI verify + module scaffold CLI |
| 14 | Documentation | STRONG | architecture.md, code-standards.md (633L), adding-a-module.md, error-codes.md |
| 15 | Example module | EXCELLENT | user (full CRUD) + audit (subscriber) + notification (subscriber) |
| 16 | Guardrails | STRONG | golangci-lint (11 linters), lefthook pre-commit/push, GitLab CI pipeline |
| 17 | Performance-safe defaults | STRONG | pgx pool, 30s timeout, Redis rate-limit, gzip, body-limit, distributed cron lock |
| 18 | Observability | STRONG | /healthz, /readyz, OpenTelemetry traces+metrics, SigNoz |
| 19 | API versioning | STRONG | proto/user/v1/, Connect RPC path versioning |
| 20 | Scalable module design | STRONG | Modular monolith, Fx modules, event-driven, zero coupling |

**Top gaps cần bổ sung:**
1. **P1** — Test template cho module mới (unit + integration boilerplate) — partially done via scaffold
2. **P2** — OpenAPI/Swagger serving cho frontend team

**Điểm mạnh:** Hexagonal architecture thực sự enforce, event-driven sẵn sàng, type-safe toàn bộ (protobuf + sqlc), security-first (Argon2id, JWT blacklist, RBAC), real testing (testcontainers)

---

### Added
- **Event System Enhancement** — All domain events (UserCreatedEvent, UserUpdatedEvent, UserDeletedEvent) now include ActorID field for complete audit trail correlation
- **Pagination Support** — User listing endpoint implements cursor-based pagination with hasMore indicator for efficient data retrieval
- **Input Validation** — protovalidate interceptor integrated into Connect RPC handler stack for declarative request validation

### Changed
- **Repository Pagination** — List method signature updated to return (users, nextCursor, hasMore, error) with internal limit+1 probing for efficient page boundary detection
- **Error Handling** — SoftDelete now returns ErrNotFound when user doesn't exist (previously silent no-op)
- **Event Publishing** — Update and Delete handlers now publish UserUpdatedEvent and UserDeletedEvent respectively, with ActorID extracted from authentication context
- **Repository Constraint Handling** — Create handler catches Postgres 23505 (unique violation) and maps to domain.ErrEmailTaken for clean error semantics

### Fixed
- **Pagination Probe Efficiency** — Repository internally fetches limit+1 records to detect page boundaries, reducing extra database queries
- **Soft Delete Idempotency** — SoftDelete properly signals non-existence vs. internal errors for correct client-side error handling

---

## Release History

> Previous releases documented as project matures. Initial implementation focused on core user module with hexagonal architecture foundation.

---

## Notes

- All changes maintain backward compatibility with existing domain interfaces
- Event publishing uses graceful error logging to prevent handler failures
- ActorID extraction follows auth.UserFromContext() pattern across all mutation handlers
