# Code Review: Architecture & Structure Consistency
**Date:** 2026-03-06
**Scope:** All 3 modules (user, audit, notification) + shared + docs
**Files read:** 40+

---

## Overall Assessment

The codebase is structurally sound with clear hexagonal architecture. The `user` module is the reference implementation and is internally consistent. The `audit` and `notification` modules follow the correct event-subscriber pattern. However, there are **14 concrete inconsistencies** spanning import ordering, logging, DI patterns, docs vs. code mismatches, and dead artifacts that a new developer would stumble on.

---

## Findings Table

| # | File(s) | Issue | Severity | Fix |
|---|---------|-------|----------|-----|
| 1 | `audit/subscriber.go` | Import group ordering wrong: internal pkg (`gnha-services/...`) appears BEFORE third-party (`ThreeDotsLabs/...`). Standard Go import order: stdlib → third-party → internal. | Important | Move `github.com/ThreeDotsLabs/watermill/message` above `github.com/gnha/gnha-services/...` |
| 2 | `notification/subscriber.go` | Same import order violation: internal import listed before `ThreeDotsLabs/watermill/message`. | Important | Same fix as #1 |
| 3 | `user/adapters/grpc/mapper.go` | Same import order violation: `connectrpc.com/connect` and `google.golang.org/protobuf/...` (third-party) appear AFTER `github.com/gnha/...` (internal). | Important | Reorder: stdlib → third-party (connect, google) → internal (gnha) |
| 4 | `notification/subscriber.go` line 41 | `slog.Error(...)` used after `ctx := msg.Context()` is in scope (line 39). All other handlers with `ctx` use `slog.ErrorContext(ctx, ...)`. Inconsistent — OTel trace ID is lost in this log. | Important | Change line 41 to `slog.ErrorContext(ctx, ...)` and line 45 to `slog.InfoContext(ctx, ...)` |
| 5 | `audit/module.go` line 12-14 | Anonymous `func(pool *pgxpool.Pool) *sqlcgen.Queries` provider is inlined in the module — it's the ONLY place in the codebase where an anonymous func is used as an `fx.Provide` argument. All other providers are named constructors. | Important | Extract to a named function `NewAuditQueries(pool *pgxpool.Pool) *sqlcgen.Queries` in `subscriber.go` or create `queries.go`. Also: if the user module ever adds a direct `*sqlcgen.Queries` provide, this will cause an Fx duplicate-provider panic. |
| 6 | `user/adapters/postgres/repository.go` line 101 | `uuid.MustParse(string(user.ID()))` — panics if `user.ID()` is not a valid UUID. Every other UUID parse in the same file uses `uuid.Parse(...)` with proper error handling (see `parseUserID` helper). | Important | Use `parseUserID(user.ID())` or `uuid.Parse(string(user.ID()))` with error return |
| 7 | `docs/code-standards.md` (PostgreSQL Adapter section, ~line 393) | Shows `type Repository struct { q *sqlc.Queries }` with `q` stored as a field (constructed once). Actual code calls `sqlcgen.New(r.pool)` inside **every** method (6 separate allocations per request chain). Pattern divergence will confuse new developers. | Important | Either update docs to show the per-method pattern, or refactor the actual repository to store `q` as a field (preferred — also removes the 6 allocations). |
| 8 | `docs/code-standards.md` (Module Registration section, ~line 651) | Shows all `fx.Provide(...)` calls grouped inside a **single** `fx.Provide(...)` block. Actual `user/module.go` uses **separate** `fx.Provide(...)` per constructor. The `adding-a-module.md` template matches the actual code. Contradictory guidance. | Important | Update `code-standards.md` to use separate `fx.Provide()` calls, matching both the actual code and `adding-a-module.md`. |
| 9 | `docs/code-standards.md` (Module Registration, ~line 651) | Shows `pgadapter.NewRepository` but actual code uses `postgres.NewPgUserRepository`. The `adding-a-module.md` correctly uses `postgres.NewPgProductRepository`. Naming convention clash. | Important | Update `code-standards.md` example to use `postgres.NewPgUserRepository` to match actual. |
| 10 | `audit/subscriber.go` vs `notification/subscriber.go` | Audit logs success implicitly (no log on success). Notification logs `slog.Info("welcome email sent", ...)` on success. Inconsistent — either both should log success or neither should. | Minor | Add `slog.InfoContext(ctx, "audit log written", "entity_id", entityID)` in audit handlers, OR remove the success log from notification. Prefer consistency — add to audit. |
| 11 | `docs/architecture.md` line 19 | Lists `model/` as a subdirectory of `shared/` ("Shared base models"). This directory **does not exist** in the codebase. `model.BaseModel` was previously identified as dead code that was removed. | Minor | Remove the `model/` line from the architecture diagram. |
| 12 | `notification/templates/` | Empty directory committed to git. The welcome template is a hardcoded constant in `notification/subscriber.go`. The empty directory implies templates will be moved there — but currently it's dead. | Minor | Either add a `.gitkeep` with a comment, or delete the directory and embed when needed. |
| 13 | `shared/config/config.go` line 26 | `ESURL string` (Elasticsearch URL) is present in config but is referenced **nowhere** in the codebase — no client, no module, no adapter uses it. | Minor | Remove `ESURL` from Config until Elasticsearch is actually used (YAGNI). |
| 14 | `audit/subscriber.go` vs `notification/subscriber.go` | Audit prefixes log messages with `"audit: ..."` consistently. Notification prefixes with `"notification: ..."` for errors but the success log `"welcome email sent"` has no prefix. If logs from both modules go to the same output, the success log is ambiguous. | Minor | Change to `"notification: welcome email sent"` for consistency. |

---

## Structural Differences Between Modules (Intentional vs. Inconsistent)

These differences are **intentional** and do not need fixing — documenting here to prevent false alarms:

| Aspect | user | audit | notification | Verdict |
|--------|------|-------|--------------|---------|
| Has `domain/` layer | Yes | No | No | Correct — audit/notification are subscriber-only, no domain entities |
| Has `adapters/` layer | Yes (postgres + grpc) | No | No | Correct — same reason |
| Has `app/` layer | Yes | No | No | Correct — same reason |
| Handler name | `UserServiceHandler` | `Handler` | `Handler` | Acceptable — different packages. `Handler` is idiomatic for single-handler packages |
| Module structure | CRUD + event publish | Event subscribe | Event subscribe | Correct bifurcation |
| `provideHandlers` pattern | No (no subscribers) | Yes | Yes | Correct — only subscriber modules need the `group:"event_handlers"` tag |

---

## Critical Issues (from previous audits, still open per MEMORY.md)

Not in scope of this consistency review but listed for completeness:
- **C-1**: AuthService proto has no Go implementation
- **C-2**: CreateUser returns stale entity (domain UUID vs DB-generated UUID is actually moot since `Create` passes the domain UUID to DB via `ID:` param — confirm no regression)
- **C-3**: `/healthz` and `/readyz` are dummy stubs

---

## Positive Observations

- Import aliasing is consistent: `sqlcgen "gen/sqlc"`, `sharederr "shared/errors"`, `appmw "shared/middleware"` — applied uniformly everywhere.
- Sentinel error variables follow the same `ErrXxx = sharederr.New(...)` pattern in every domain errors file.
- Event handler names `HandleUserCreated`, `HandleUserUpdated`, `HandleUserDeleted` are perfectly symmetric between audit and notification.
- `//go:generate mockgen` directive is correctly placed in `domain/repository.go`.
- Test files use stdlib `testing` (not testify), consistent throughout.
- `ctx := msg.Context()` extracted from Watermill message in all subscriber handlers — correct pattern.
- All `fx.Module` declarations include a descriptive name string as the first arg.

---

## Recommended Actions (Prioritized)

1. **Fix import ordering** in `audit/subscriber.go`, `notification/subscriber.go`, `user/adapters/grpc/mapper.go` — run `goimports` to autofix (Issues 1–3)
2. **Fix `uuid.MustParse`** in `repository.go:101` — potential panic in production (Issue 6)
3. **Extract anonymous Fx provider** in `audit/module.go` to a named function (Issue 5)
4. **Fix `slog.ErrorContext`** in `notification/subscriber.go:41,45` (Issue 4)
5. **Update `docs/code-standards.md`** — fix the Repository pattern example and module registration example (Issues 7, 8, 9)
6. **Remove dead `ESURL` config field** (Issue 13)
7. **Remove `model/` from architecture.md** (Issue 11)
8. **Delete empty `notification/templates/`** (Issue 12)
9. **Standardize success logging** between audit and notification subscribers (Issues 10, 14)

---

## Metrics

- Import ordering violations: 3 files
- Docs vs. code mismatches: 3 distinct examples
- Panic risk (MustParse): 1 location
- Dead config fields: 1 (`ESURL`)
- Empty committed directories: 1 (`notification/templates/`)
- Logging inconsistencies: 3 locations

---

## Unresolved Questions

1. Is the per-method `sqlcgen.New(r.pool)` pattern intentional (to support transaction passing via `q := sqlcgen.New(tx)`)? If so, `code-standards.md` must document that the stored-`q` pattern is incompatible with the transaction use case, which is why it's not used.
2. The `audit` module registers `*sqlcgen.Queries` as an Fx singleton. If a future module also provides `*sqlcgen.Queries`, Fx will panic at startup. Is there a plan to namespace these (e.g., using `fx.Tag`)?
3. `notification/subscriber.go` handles only `UserCreatedEvent`. Is the absence of `HandleUserUpdated` / `HandleUserDeleted` intentional (notifications only on create) or an oversight? No doc explains this asymmetry.
