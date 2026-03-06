# Boilerplate DX Comprehensive Review

Date: 2026-03-06 | 3 parallel reviewers: architecture, DX/onboarding, scaffold templates

## Critical (compile/runtime blockers)

| # | Area | Issue | Fix |
|---|------|-------|-----|
| C1 | `queries.tmpl` | **Missing `GetByIDForUpdate` query.** `adapter_postgres.tmpl` calls `q.Get{{Name}}ByIDForUpdate()` in Update — query doesn't exist in template. Every scaffolded module fails compilation after `task generate`. | Add `-- name: Get{{.NameTitle}}ByIDForUpdate :one` SELECT...FOR UPDATE to queries.tmpl |
| C2 | Scaffold templates | **Event bus missing from app handlers.** `app_create/update/delete.tmpl` have `// TODO: publish event` but no `bus *events.EventBus` in struct/constructor. When dev follows user module pattern, build breaks (missing imports: slog, events, auth). | Wire EventBus into handler struct + constructor + publish skeleton in all mutation templates |
| C3 | `user/.../repository.go:101` | **`uuid.MustParse()` panics** if domain entity holds malformed UUID. Every other UUID parse in same file uses `parseUserID()` with error return. | Replace with `parseUserID(user.ID())` |
| C4 | Taskfile `monitor:up` | References `deploy/docker-compose.monitor.yml` which **doesn't exist**. Fails immediately. | Create file or remove tasks until needed |

## Important (DX friction / inconsistency)

| # | Area | Issue | Fix |
|---|------|-------|-----|
| I1 | Import ordering | audit/subscriber.go, notification/subscriber.go, grpc/mapper.go — internal imports before third-party. | Run `goimports ./...` |
| I2 | notification/subscriber.go | `slog.Error()` instead of `slog.ErrorContext(ctx)` — OTel trace ID lost in logs. | Use `slog.ErrorContext(ctx, ...)` |
| I3 | audit/module.go | Anonymous inline `func` in `fx.Provide`. Every other provider is a named constructor. Also risks Fx duplicate-provider panic if second module provides `*sqlcgen.Queries`. | Extract to named `NewAuditQueries()` |
| I4 | code-standards.md | 3 mismatches with actual code: (a) stored `q` field vs per-method `sqlcgen.New(r.pool)`, (b) single `fx.Provide()` block vs separate calls, (c) wrong constructor name `pgadapter.NewRepository`. | Update docs to match actual code |
| I5 | code-standards.md | Unit test example uses `events.NewEventBus()` (requires live RabbitMQ). Should be `&noopPublisher{}` stub. | Fix example |
| I6 | README.md | No Prerequisites section (Go version, Docker version, go-task install). New dev gets `command not found`. | Add Prerequisites |
| I7 | README.md | No Dev Services table (ports for MailHog :8025, RabbitMQ management :15672). | Add table |
| I8 | adding-a-module.md | Step 5 "Update domain entity, handlers, adapters" too vague. Doesn't name specific symbols to change. | Name exact symbols + add end-to-end field-addition example |
| I9 | Scaffold | Multi-word names (`order_item`) generate invalid Go import alias `order_itemv1connect`. | Reject underscore in names or handle concatenation |
| I10 | Scaffold next-steps | "Update toDomain(), Create/UpdateParams, toProto()" without file paths. Dev has 19 new files. | Include file paths in output |
| I11 | Scaffold templates | No unique-constraint handling in `adapter_postgres.tmpl` Create. Actual pattern uses `pgconn` error check for 23505. | Add skeleton + TODO |
| I12 | architecture.md | Lists `model/` directory that doesn't exist. Shows Auth/RBAC inside Connect handler chain (actually at Echo layer). | Fix diagram |
| I13 | error-codes.md | Missing `CodeFailedPrecondition` (412) and `CodeUnavailable` (503). | Add both |
| I14 | Config | `ESURL` (Elasticsearch) configured but never used anywhere. | Remove (YAGNI) |
| I15 | Taskfile `dev:setup` | `sleep 5` unreliable on cold start. ES takes 20-30s. | Replace with readiness poll or `docker compose wait` |
| I16 | Taskfile | No `dev:down` task. Dev must know raw docker compose command. | Add `dev:down` |
| I17 | Docs | `testing-strategy.md` says `NewTestPostgres(t)`, `code-standards.md` says `NewTestDB(t, ctx)`. One is wrong. | Reconcile to actual testutil signature |

## Minor

| # | Area | Issue |
|---|------|-------|
| M1 | notification/subscriber.go | Success log `"welcome email sent"` missing module prefix (audit uses `"audit: ..."`) |
| M2 | notification/templates/ | Empty directory committed to git — YAGNI |
| M3 | Scaffold templates | No header comment identifying files as scaffold-generated |
| M4 | Scaffold templates | No TODO comments at customization points (proto fields, query columns, mapper fields) |
| M5 | docker-compose.dev.yml | MailHog has no healthcheck (all other services do) |
| M6 | No CONTRIBUTING.md | No branch naming, PR process, conventional commits guidance |

## Positive (What's Already Excellent)

- Hexagonal architecture boundaries genuinely enforced — domain layer has zero framework imports
- Sentinel errors follow identical pattern everywhere
- fx.Module plug-and-play works as advertised
- Seed script fully idempotent
- Testcontainers with auto-cleanup — real infra testing
- Generated code has proper "DO NOT EDIT" headers
- Lefthook pre-commit/pre-push gates solid
- CI pipeline comprehensive (lint → generated-check → unit → integration → build → deploy)
- Scaffold core engineering is sound (conflict detection, reserved-word validation)

## Priority Actions

### P0 — Fix Now (blocks scaffold usage)
1. Fix queries.tmpl: add GetByIDForUpdate query
2. Wire EventBus into scaffold mutation templates
3. Fix uuid.MustParse panic in user repository

### P1 — Fix Before Onboarding New Dev
4. Run `goimports ./...` (fixes I1 automatically)
5. Fix notification slog.ErrorContext (I2)
6. Update code-standards.md examples (I4, I5)
7. Add README prerequisites + dev services table (I6, I7)
8. Fix adding-a-module.md with specific symbols (I8)
9. Handle multi-word scaffold names (I9)
10. Reconcile testutil docs (I17)

### P2 — Polish
11. Remove dead ESURL config, missing monitor compose, empty templates dir
12. Add CONTRIBUTING.md
13. Add TODO comments to scaffold templates
14. Fix architecture.md diagram
15. Add dev:down task

## Unresolved Questions

1. Is `sqlcgen.New(r.pool)` per-method intentional (enables `sqlcgen.New(tx)` in transactions)? If yes, document why in code-standards.md
2. Audit module provides `*sqlcgen.Queries` globally via Fx — will panic if second module does same. Use `fx.Tag` to namespace?
3. Notification only handles UserCreatedEvent. Intentional (welcome-only) or oversight?
4. API key auth referenced in docs but not implemented. Planned or stale docs?
5. `docs/development-roadmap.md` referenced in rules but doesn't exist. External tracker?
