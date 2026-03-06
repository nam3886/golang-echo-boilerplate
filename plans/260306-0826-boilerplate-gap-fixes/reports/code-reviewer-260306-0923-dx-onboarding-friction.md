# DX / Onboarding Friction Review — GNHA Services

**Date:** 2026-03-06
**Reviewer:** code-reviewer agent
**Scope:** All onboarding-surface files reviewed from a new-developer perspective

---

## Summary

The project has a solid structural foundation. The scaffold generator, Taskfile, and docs cover a wide surface area. However, there are ~20 distinct friction points spread across docs, config, tooling, and templates that would cause a new developer to stop and ask questions. Most are fixable with targeted doc edits or minor template changes. None are architectural.

---

## Friction Table

| # | Area | Friction Point | Impact | Suggestion |
|---|------|----------------|--------|------------|
| 1 | **README — Quick Start** | `task dev:setup` runs `sleep 5` and then immediately calls `migrate:up`, but Postgres/RabbitMQ/ES may not be healthy yet (ES takes 20-30 s cold). New dev gets a confusing migration connection error on first run. | High | Replace `sleep 5` with a healthcheck poll loop (`docker compose wait` or a shell loop checking `pg_isready`). Document that ES starts slowly. |
| 2 | **README — Quick Start** | `task dev:setup` calls `task seed` which needs `DATABASE_URL` in `.env`, but the `cp -n .env.example .env` runs just before `migrate:up` in the same task — no mention that the user must edit `.env` first (JWT_SECRET is `change-me...` which is < 32 chars by default and will fail `config.Load()`). | High | The default `JWT_SECRET` value in `.env.example` is only 30 characters (`change-me-in-production-use-a-strong-secret` — 42 chars actually, but dev should verify). Add a warning comment in `.env.example` above `JWT_SECRET`: `# MUST be ≥ 32 chars`. Also add a README note: "Edit .env before running task dev:setup". |
| 3 | **README — Missing info** | No mention of: (a) required Go version, (b) Docker minimum version, (c) `task` (go-task) installation. A new dev who runs `task` and gets `command not found` has no guidance. | High | Add Prerequisites section: Go 1.26+, Docker 24+, go-task v3 (`brew install go-task` / `go install github.com/go-task/task/v3/cmd/task@latest`). |
| 4 | **README — No dev UI info** | `docker-compose.dev.yml` starts MailHog on `:8025` and RabbitMQ management on `:15672`, but README says nothing about these. A new dev sending test emails has no idea where to look. | Medium | Add Dev Services table: Postgres `:5432`, Redis `:6379`, RabbitMQ management `:15672`, MailHog `:8025`. |
| 5 | **README — monitor:up references non-existent file** | `task monitor:up` references `deploy/docker-compose.monitor.yml` which does not exist. Running it gives a Docker compose error with no context. | High | Either create `docker-compose.monitor.yml` or remove `monitor:up`/`monitor:down` from Taskfile and README until it exists. |
| 6 | **README — No CONTRIBUTING.md** | No `CONTRIBUTING.md` or equivalent. New dev has no guidance on: PR process, branch naming, commit format, how to run pre-commit hooks (lefthook), or how to get code reviewed. | Medium | Add `CONTRIBUTING.md` covering: branch naming, conventional commits, `task check` before PR, lefthook setup. |
| 7 | **docs/adding-a-module.md — Quick Start step mismatch** | Quick Start lists 8 steps but step 5 says "Update domain entity, handlers, and adapters to match new fields." This is vague — a new dev doesn't know which specific symbols to update (the template generates a `Name`-only entity; real fields require changes to `Create*Params`, `toDomain()`, `toProto()`, proto messages, and SQL queries). The manual reference sections below don't explicitly call out which symbols change together. | High | Add a "What to customize after scaffold" mini-checklist that names the exact symbols: `sqlcgen.Create{{Entity}}Params`, `toDomain()` function, `toProto()` function, and proto message fields. |
| 8 | **docs/adding-a-module.md — `toDomain` uses generic struct field** | The manual example's `toDomain` is not shown in the manual sections — the reader must infer it from the postgres template. The Quick Start says "Update adapters to match new fields" without showing what that looks like for a two-field entity. | Medium | Add a short `toDomain` example in the manual section that shows adding a second field (e.g., `Price`) end-to-end through migration → SQL → sqlcgen params → toDomain → toProto. |
| 9 | **docs/adding-a-module.md — scaffold generates `GetByIDForUpdate` but it's not in the SQL template** | The `adapter_postgres.tmpl` calls `q.Get{{Name}}ByIDForUpdate(ctx, uid)` in the `Update` method. The `queries.tmpl` does NOT include a `GetProductByIDForUpdate :one SELECT ... FOR UPDATE` query. After scaffold + `task generate`, the code will NOT compile. | Critical | Add the missing `-- name: Get{{Name}}ByIDForUpdate :one SELECT ... FOR UPDATE` query to `queries.tmpl`, or update `adapter_postgres.tmpl` to use `GetByID` within the transaction (less correct but compiles). |
| 10 | **docs/code-standards.md — Postgres adapter example uses string ID** | The `Create` example in the PostgreSQL Repository section passes `ID: string(user.ID())` but the real sqlc-generated type for UUID fields is `pgtype.UUID` or `[16]byte`, not `string`. A new dev copy-pasting this will get a compile error. | High | Update the example to use `uuid.MustParse(string(user.ID()))` (as shown in `adding-a-module.md`) or the pgtype pattern. |
| 11 | **docs/code-standards.md — Test example references `events.NewEventBus()`** | Line 589: `bus := events.NewEventBus()` is used in the unit test example, but `EventBus` requires RabbitMQ connection (it's a Watermill publisher, not an in-memory stub). A new dev running this test without infra gets a panic or nil pointer. | High | Replace with `&noopPublisher{}` stub or explicitly document that unit tests must use a noop publisher (as the testing-strategy.md correctly does on line 32). |
| 12 | **docs/architecture.md — Request flow diagram is wrong** | The request flow shows `Auth Middleware → RBAC Middleware` in the middle of the Connect RPC handler chain, but in reality Auth+RBAC are Echo route-group middleware applied *before* the Connect handler, not inside it. Also, RBAC middleware exists but is never invoked in any `RegisterRoutes` — the diagram implies it is active. | Medium | Fix the flow to show Auth/RBAC at the Echo layer before the Connect handler. Add a note that RBAC is currently not wired (known gap). |
| 13 | **docs/error-codes.md — Missing codes** | The real `domain_error.go` defines `CodeFailedPrecondition` and `CodeUnavailable` which are absent from `error-codes.md`. A dev implementing a precondition check won't find the right code in the docs. | Medium | Add the two missing codes to the table: `FAILED_PRECONDITION → 412`, `UNAVAILABLE → 503`. |
| 14 | **docs/testing-strategy.md — Testutil helper names don't match** | `testing-strategy.md` line 104 lists `NewTestPostgres(t)` but the actual testutil function (referenced in code-standards.md line 630) is `NewTestDB(t, ctx)`. A dev following the docs will get a compile error. | High | Audit testutil helpers and align both documents to the real function signatures. |
| 15 | **Taskfile.yml — `dev:deps` uses bare `sleep 5`** | As noted in #1, `sleep 5` is not reliable for ES or RabbitMQ readiness. Additionally `dev:deps` has no `desc` line for `--list` output — wait, it does (`desc: Start infrastructure containers`). But the `sleep 5` is the core issue. | High | Same fix as #1: poll for readiness. Alternatively, add `depends_on: condition: service_healthy` and use `docker compose wait`. |
| 16 | **Taskfile.yml — Missing `dev:down` task** | There's no `task dev:down` to stop and remove dev containers. A new dev must know the raw `docker compose -f deploy/docker-compose.dev.yml down` command. | Medium | Add `dev:down` task mirroring `dev:deps` but running `down`. |
| 17 | **Taskfile.yml — `migrate:create` docs** | `task migrate:create` uses `{{.CLI_ARGS}}` but nowhere in the README or Taskfile desc does it show the invocation syntax (`task migrate:create -- add_email_index`). A new dev will run `task migrate:create` with no args and get an unnamed migration file. | Low | Update `desc` to: `Create a new migration — usage: task migrate:create -- <name>`. |
| 18 | **`.env.example` — Elasticsearch not explained** | `ELASTICSEARCH_URL` appears in `.env.example` and `config.go`, but Elasticsearch is never referenced in any doc. A new dev wonders: is it required? What uses it? The `ESURL` field is in Config but nothing in the codebase reads it (no search code exists). | Medium | Either remove `ELASTICSEARCH_URL` from `.env.example`/Config if unused, or add a comment `# Used by search module (not yet implemented)`. |
| 19 | **`cmd/scaffold/main.go` — Next-step message gap** | The scaffold output says `5. Update generated code: toDomain(), Create/UpdateParams, toProto()` but doesn't tell the dev *where* those are located. A new dev has 19 new files and doesn't know which file contains each symbol. | Medium | Expand step 5 to include file paths: `adapters/postgres/repository.go → toDomain(), Create*Params`, `adapters/grpc/mapper.go → toProto()`. |
| 20 | **`cmd/scaffold/main.go` — Multi-word names not handled** | `validateIdentifier` allows underscores (e.g., `order_item`) and `toTitle` converts them to PascalCase (`OrderItem`), but `NameSnake` is set to `*name` directly (e.g., `order_item`) while `NamePlural` defaults to `order_items`. The generated package directory would be `internal/modules/order_item/` which is valid Go but unconventional. More critically, the import alias `order_itemv1connect` (generated) is not a valid Go identifier. | High | Either: document that only single-word names are supported, or add a validation guard rejecting underscore names, or properly handle the package alias generation for multi-word names. |
| 21 | **`cmd/scaffold/templates/app_create.tmpl` — No event publishing** | The scaffold generates `// TODO: publish {{.Name}}.created event` as a comment stub, which means scaffolded modules silently omit events unless the dev notices and wires them. The `code-standards.md` mandates event publishing after every mutation. | Medium | Wire the event bus into the scaffold template (matching the pattern in `app_create.go` for the user module), with the `ActorID` extraction. Mark it optional with a comment only if the bus is explicitly excluded. |
| 22 | **Generated files — Header present but inconsistent** | `gen/sqlc/user.sql.go` has `// Code generated by sqlc. DO NOT EDIT.`. Proto generated files have the same. Good. However, the scaffold-generated files in `internal/modules/{name}/` have NO "do not edit" or "generated by scaffold" header comment — confusing for devs who don't know which files are templates vs. hand-written. | Low | Add a `// Code scaffolded by cmd/scaffold. Customize as needed.` header to all scaffold templates, so devs immediately know the file origin. |
| 23 | **docker-compose.dev.yml — No MailHog health check** | All services have `healthcheck:` except `mailhog`. The `dev:deps` sleep waits for all services, but if MailHog (or any healthcheck-less service) is still starting, there's no detection. | Low | Add `healthcheck` to MailHog or document that MailHog may take a few seconds to respond after infra starts. |
| 24 | **No `docs/development-roadmap.md`** | `documentation-management.md` rules reference `./docs/development-roadmap.md` as a required living document, but it doesn't exist. Any developer asked to update the roadmap will find nothing. | Medium | Create a minimal `docs/development-roadmap.md` or remove the reference from `documentation-management.md`. |
| 25 | **Auth middleware — API key support undocumented** | `auth/` directory has `jwt.go`, `password.go`, `context.go` but no `apikey.go` despite the README saying "JWT/API key validation" and the architecture doc listing "JWT/API key". A developer implementing API key auth has no template to follow. | Medium | Either add `auth/apikey.go` or remove API key references from docs until implemented. |

---

## Critical Issues (blocking first run or compilation)

1. **#9 — Missing `GetByIDForUpdate` SQL query in scaffold template**: Scaffolded modules will not compile. This is the highest-priority fix.
2. **#1/#5 — `task dev:setup` will fail** on cold start due to unreliable sleep and missing `docker-compose.monitor.yml`.
3. **#11 — Unit test example uses non-stub EventBus**: Causes test failures or panics without infra.

---

## Positive Observations

- Scaffold generates 19 correct files with proper template substitution — an exceptionally strong onboarding accelerator once the SQL gap is fixed.
- `cmd/scaffold/main.go` validates identifier names, checks for reserved words, and checks file conflicts before writing — excellent defensive design.
- Generated code has proper `// Code generated by ...` headers.
- `Taskfile.yml` descriptions are clear and `task --list` is immediately useful.
- `.env.example` covers every required config variable with sensible defaults.
- `docs/code-standards.md` contains real, non-trivial examples across every layer — better than most boilerplates.
- `docker-compose.dev.yml` uses healthchecks on all infra services (except MailHog).
- `cmd/seed/main.go` is idempotent (skips existing users) — safe to re-run.
- `domain_error.go` is well-structured with HTTP mapping and sentinel errors — clear pattern for new devs.

---

## Recommended Fix Priority

1. Fix `queries.tmpl` to include `GetByIDForUpdate` (Critical — compile blocker)
2. Fix `task dev:setup` healthcheck wait (High — first-run blocker)
3. Create or remove `docker-compose.monitor.yml` (High — task:monitor is broken)
4. Add Prerequisites section to README (High — new dev blocker)
5. Fix `events.NewEventBus()` in code-standards.md test example (High — misleading)
6. Fix Postgres `Create` example ID type in code-standards.md (High — compile error if copied)
7. Align testutil helper names across docs (High — compile error if copied)
8. Fix architecture.md request flow diagram (Medium)
9. Add missing error codes to error-codes.md (Medium)
10. Add `dev:down` Taskfile task (Medium)
11. Add Dev Services table to README (Medium)
12. Expand scaffold next-step message with file paths (Medium)
13. Guard or document multi-word module name limitation (High — silent breakage)
14. Wire events into scaffold app_create template (Medium)
15. Add CONTRIBUTING.md (Medium)
16. Remove or explain `ELASTICSEARCH_URL` (Medium)

---

## Unresolved Questions

- Does `auth/apikey.go` exist elsewhere or is API key auth genuinely unimplemented?
- Is `ELASTICSEARCH_URL` used by any planned module, or is it dead config?
- Is `docs/development-roadmap.md` intentionally omitted (team uses another tracker)?
- Should `testutil.NewTestDB(t, ctx)` or `testutil.NewTestPostgres(t)` be the canonical name — one of the docs is wrong, which?
