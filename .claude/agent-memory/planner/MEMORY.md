# Planner Agent Memory

## Project: golang-echo-boilerplate
- **Architecture:** Hexagonal modular monolith (Go 1.26, Echo, Connect RPC, Fx DI)
- **DB:** Postgres (pgx + sqlc), Redis, RabbitMQ (Watermill), Elasticsearch (optional)
- **Testing:** gomock for mocks, testcontainers for integration, `//go:build integration` tag convention
- **Events:** Watermill pub/sub via `events.EventPublisher` interface wrapping `message.Publisher`

## Key Patterns
- Domain entities: unexported fields, constructor validation, `Reconstitute` for persistence rebuild
- Error handling: `DomainError` with `ErrorCode`, sentinel constructors (`ErrNotFound()` returns fresh pointer)
- `DomainError.Is()` compares by `ErrorCode`, not pointer -- so `errors.Is(err, ErrNotFound())` works
- Event publishing: after successful persistence, log-but-don't-fail pattern
- RBAC: Echo middleware (`RequirePermission`) for route groups + Connect RPC interceptor (`RBACInterceptor`) for procedures
- File limit: 200 lines per file per code standards
- sqlc queries in `db/queries/`, generated code in `gen/sqlc/`

## Planning Conventions
- Parallel phases need strict file ownership matrix (no overlapping edits)
- Sequential phases can share files with prior phases
- Always include code snippets showing exact fix in phase files
- Plan.md max 80 lines, YAML frontmatter required

## Known Issues (Active -- boilerplate-fix plan)
- User module domain errors still use `var` (mutable pointers) -- must convert to constructor functions
- Scaffold templates at ~55% alignment with user module (golden reference)
- 12 packages at 0% test coverage (shared/errors, grpc adapter, middleware, config, etc.)
- Audit module imports user domain directly (violates no cross-module rule)
- `mail.ParseAddress` result `.Address` field not extracted

## Scaffold Templates
- Location: `cmd/scaffold/templates/*.tmpl` (19 files)
- Main gaps vs user module: error pattern, List return type, cursor handling, event publishing, ID guards

## Doc Files That Frequently Need Sync
- `docs/code-standards.md` (679 lines) -- event patterns, error patterns, List signature
- `docs/testing-strategy.md` (141 lines) -- error pattern, test stubs
- `docs/adding-a-module.md` (423 lines) -- event topic location
