# Scaffold Template Audit

**Date:** 2026-03-06
**Scope:** cmd/scaffold/templates/ (19 templates) vs internal/modules/user/ (13 actual files)
**Verdict:** Generated code WILL NOT COMPILE as-is for any module that mirrors the user module's real patterns.

---

## Issue Table

| Template | Issue | Severity | Fix |
|---|---|---|---|
| `app_create.tmpl` | Missing `EventBus`, `PasswordHasher` dependencies — constructor only takes `repo`. Actual `CreateUserHandler` takes `(repo, hasher, bus)`. Generated code compiles but is structurally wrong. | Critical | Add `bus *events.EventBus` field + constructor param; add TODO comment for hasher if module doesn't need passwords |
| `app_create.tmpl` | No event publish after `repo.Create`. Actual code publishes `user.created` with `auth.UserFromContext(ctx)` actor extraction, `slog.ErrorContext` on failure. Template has only `// TODO: publish {{.Name}}.created event` but no skeleton. | Critical | Provide actual publish call skeleton with `auth.UserFromContext`, `slog.ErrorContext` — matching real pattern |
| `app_create.tmpl` | Missing email-uniqueness pre-check (`GetByEmail` + `ErrEmailTaken`). User-specific but the pattern (lookup-before-create) should be called out as a TODO. Missing imports: `errors`, `log/slog`, `time`, `auth`, `sharederr`, `events`. | Critical | Imports will cause compile error since `"fmt"` is the only extra import but event code references `slog`, `time`, `auth`, `events` |
| `app_create.tmpl` | `NewCreate{{.NameTitle}}Handler(mockRepo)` call in test won't match generated constructor signature once bus/hasher are added. | Critical | Constructor signature mismatch |
| `app_update.tmpl` | Missing `EventBus` dependency — constructor only takes `repo`. Actual `UpdateUserHandler` takes `(repo, bus)`. Generated code compiles but misses event publishing entirely. | Critical | Add `bus *events.EventBus` field + constructor param |
| `app_update.tmpl` | No event publish after update. Missing imports `log/slog`, `time`, `auth`, `events`. | Critical | Same pattern as create: publish skeleton + imports |
| `app_delete.tmpl` | Missing `EventBus` dependency — constructor only takes `repo`. Actual `DeleteUserHandler` takes `(repo, bus)`. | Critical | Add `bus *events.EventBus` field + constructor param |
| `app_delete.tmpl` | No event publish after soft-delete. Missing imports `log/slog`, `time`, `auth`, `events`. | Critical | Same pattern |
| `adapter_postgres.tmpl` | `Create` method uses `parseProductID` + explicit UUID param, but actual user `Create` uses `uuid.MustParse` directly (no helper call) and also passes `Email`, `Password`, `Role` fields that don't exist in template. This is acceptable as customization, BUT the template calls `parse{{.NameTitle}}ID(entity.ID())` then passes `ID: uid` — correct pattern. The real user repo skips the helper for Create and uses `uuid.MustParse`. Minor divergence but both compile. | Minor | Add comment: `// TODO: add module-specific fields to CreateXxxParams` |
| `adapter_postgres.tmpl` | Missing `pgconn` import. Actual user repo imports `"github.com/jackc/pgx/v5/pgconn"` for unique-constraint detection (`pgErr.Code == "23505"`). Template has no duplicate-key handling at all. Generated code compiles but silently returns a generic error on unique violations. | Important | Add `pgconn` import + constraint-check skeleton with TODO comment |
| `adapter_postgres.tmpl` | `toDomain` signature is `toDomain(row sqlcgen.{{.NameTitle}})` — correct. BUT it only maps `id`, `name`, `createdAt`, `updatedAt`, `deletedAt`. Actual user `toDomain` maps `email`, `password`, `role` too. The template cannot know domain-specific fields; however the comment `// TODO: customize` is absent. | Minor | Add `// TODO: add module-specific fields from row` comment inside `Reconstitute()` call |
| `adapter_postgres.tmpl` | `Update` method only extracts `name` from entity and passes `Name: pgtype.Text{...}`. Actual user update also extracts `role` and passes `Role: pgtype.Text{...}`. No comment indicating customization needed. | Minor | Add TODO comment after `name := entity.Name()` |
| `adapter_postgres.tmpl` | `GetByEmail` method is completely absent. The actual `UserRepository` interface has `GetByEmail`. Template `domain_repository.tmpl` does NOT include `GetByEmail` either — consistent, but a generated module won't compile if the developer adds `GetByEmail` to domain without adding it to the postgres adapter. | Minor | N/A — intentional omission since this is user-specific, but doc comment on repository template should note this |
| `adapter_grpc_handler.tmpl` | `Create{{.NameTitle}}` handler only passes `Name` from request: `app.Create{{.NameTitle}}Cmd{Name: req.Msg.Name}`. Actual user handler passes `Email`, `Name`, `Password`, `Role`. The template only matches a name-only entity which won't compile once the developer adds fields to the proto and Cmd. | Minor | Add `// TODO: map additional request fields` comment in Create handler |
| `adapter_grpc_handler.tmpl` | `Update{{.NameTitle}}` handler only handles `req.Msg.Name` optional field. Actual user handler also handles `req.Msg.Role`. No comment indicating more fields may need wiring. | Minor | Add TODO comment |
| `adapter_grpc_handler.tmpl` | `List{{.NamePluralTitle}}` handler iterates `result.Items` but actual user handler iterates `result.Users` (the field is `Users`, not `Items`). **This is a compile error** if the developer uses the list template for app layer unchanged — `result.Items` does not exist on `ListUsersResult`. | Critical | Template `app_list.tmpl` uses `Items []*domain.{{.NameTitle}}` but actual `list_users.go` uses `Users []*domain.User`. The template is internally self-consistent but diverges from the user module. The handler template correctly uses `result.Items` matching the template's struct field. No compile error IF both templates are used together. Inconsistency vs user module: document that `Items` is the scaffold convention. |
| `adapter_grpc_mapper.tmpl` | `toProto` only maps `Id`, `Name`, `CreatedAt`, `UpdatedAt`. Actual user mapper maps `Email`, `Role` too. No TODO comment. | Minor | Add `// TODO: add module-specific proto fields` comment |
| `adapter_grpc_routes.tmpl` | Template is correct and matches actual routes.go exactly. No issues. | — | — |
| `module.tmpl` | Does NOT provide `EventBus` to the Fx container. Since app handlers (create/update/delete) will need `*events.EventBus`, Fx will fail to resolve the dependency at runtime. The user `module.go` does not provide it either — it relies on the shared events module providing it globally. This is correct and consistent. But with the current app templates (no bus param), generated code won't need it anyway — until the developer adds it. | Minor | Add comment: `// EventBus is provided by shared/events.Module — ensure it is registered in main.go` |
| `app_create_test.tmpl` | `NewCreate{{.NameTitle}}Handler(mockRepo)` — constructor will mismatch if handler is updated to take bus+hasher. Test won't compile. | Critical | Scaffold test matches scaffold handler. Issue is the handler template is wrong, so fixing the handler template fixes this. |
| `app_create_test.tmpl` | Test does NOT mock `GetByEmail` call — once email uniqueness check is added to handler, test will fail with "unexpected call". | Important | Add `mockRepo.EXPECT().GetByEmail(...)` expectation once handler template is corrected |
| `adapter_postgres_test.tmpl` | `createTest{{.NameTitle}}` helper calls `domain.New{{.NameTitle}}(name)` with only one arg. If the domain entity requires more args (like email+password+role for user), the test won't compile. No comment warning about this. | Important | Add `// TODO: update createTestXxx() args to match New{{.NameTitle}}() signature` |
| `domain_test.tmpl` | `New{{.NameTitle}}("Test Name")` — same issue: called with single arg. The domain template `New{{.NameTitle}}` also takes a single arg, so this is internally consistent. But diverges from user module. Template is self-consistent; just note it for the developer. | Minor | Add `// TODO: update test args if you add more constructor params` comment |
| `domain_entity.tmpl` | `Reconstitute` function name is not prefixed — conflicts if multiple modules are in same package. This is fine since each module has its own `domain` package. No issue. | — | — |
| `domain_errors.tmpl` | Only defines `Err{{.NameTitle}}NotFound` and `ErrNameRequired`. Actual user errors include `ErrEmailRequired`, `ErrInvalidRole`, `ErrEmailTaken`. This is expected — module-specific. But no comment `// TODO: add module-specific errors`. | Minor | Add comment |
| `migration.tmpl` | Uses `DEFAULT gen_random_uuid()` for ID. But scaffold's `adapter_postgres.tmpl` `Create` passes an explicit `ID: uid` from `parseProductID(entity.ID())`. These conflict: DB generates UUID, but code also sends one. If the SQL is `INSERT INTO products (id, name) VALUES ($1, $2)` it works. If omitted, DB generates — which breaks the domain ID contract. The query template `Create{{.NameTitle}}` explicitly inserts `id`, so the `DEFAULT gen_random_uuid()` is just a fallback — no conflict. However, the user module uses `uuid.MustParse` (not the parse helper) and the known issue C-2 (stale entity ID) exists in the actual user module. Scaffold avoids this by passing explicit ID. Good. | — | — |
| `queries.tmpl` | `List{{.NamePluralTitle}}` query uses `(created_at, id) < (cursor_created_at, cursor_id)` for keyset. Actual user query is identical. Matches. | — | — |
| `queries.tmpl` | `Update{{.NameTitle}}` only patches `name`. User's actual query also patches `role`. No TODO comment. | Minor | Add `-- TODO: add module-specific columns` comment |
| `proto.tmpl` | Proto `{{.NameTitle}}` message only has `id`, `name`, `created_at`, `updated_at`. No `email`, `role`. Expected — module-specific. No TODO comment on message fields. | Minor | Add `// TODO: add module-specific fields` comment |
| `domain_repository.tmpl` | Missing `GetByEmail` (user-specific). Interface matches a name-only entity correctly. Consistent. But no TODO for lookup-by-alternate-key pattern. | Minor | Add `// TODO: add lookup methods (e.g. GetByEmail) if module needs alternate lookups` |

---

## Compile-Blocking Issues Summary

These will prevent `go build` of the generated module:

1. **`app_create.tmpl`** — if developer adds `bus` to constructor (following real pattern), test `NewCreate{{.NameTitle}}Handler(mockRepo)` breaks.
2. **`app_create.tmpl` / `app_update.tmpl` / `app_delete.tmpl`** — `// TODO: publish event` comments contain no imports for `log/slog`, `time`, `events`, `auth`. When developer fills in the TODO using the user module as reference, they add imports that are not present, requiring manual import editing. Not a compile error in the template as-is, but a trap.
3. **`adapter_postgres_test.tmpl`** — `createTest{{.NameTitle}}(t, "name")` will break if module entity has multiple constructor args.

## Structural Divergences (Not Compile Errors)

| Area | Template Convention | User Module Reality |
|---|---|---|
| List result field | `Items []*domain.{{.NameTitle}}` | `Users []*domain.User` |
| Create handler deps | `(repo)` | `(repo, hasher, bus)` |
| Update handler deps | `(repo)` | `(repo, bus)` |
| Delete handler deps | `(repo)` | `(repo, bus)` |
| Postgres Create | uses `parseID` helper + explicit UUID | uses `uuid.MustParse` inline |
| Unique constraint | not handled | `pgconn.PgError` code `23505` check |
| Actor extraction | absent | `auth.UserFromContext(ctx)` pattern in all mutating handlers |

## Positive Observations

- Import paths using `{{.GoModule}}` are correct and will resolve properly from go.mod.
- `connectrpc.com/validate` interceptor wired in routes — matches actual.
- Cursor pagination implementation in postgres adapter is identical to actual and correct.
- `SELECT FOR UPDATE` in Update transaction — matches actual.
- `SoftDelete` returns `ErrNotFound` when `rows == 0` — matches actual.
- `domain_errors.tmpl` uses `sharederr.New(...)` (not `errors.New`) — correct pattern.
- Interface compliance guard `var _ XxxServiceHandler = (*XxxServiceHandler)(nil)` — present and correct.
- Fx `fx.Annotate` with `fx.As(new(domain.XxxRepository))` — matches actual.
- `//go:generate mockgen` directive on repository — correct path pattern.

## Recommended Fixes (Priority Order)

1. **app_create.tmpl** — Add `bus *events.EventBus` to struct+constructor+imports (`log/slog`, `time`, `auth`, `events`). Add full publish skeleton with `auth.UserFromContext`. Add hasher TODO comment.
2. **app_update.tmpl** — Same bus wiring + publish skeleton.
3. **app_delete.tmpl** — Same bus wiring + publish skeleton.
4. **app_create_test.tmpl** — Update constructor call to include mock bus. Add `GetByEmail` EXPECT for uniqueness check path.
5. **adapter_postgres.tmpl** — Add `pgconn` import + constraint error check skeleton in `Create`. Add TODO comments in `toDomain`, `Update`, `Create` for module-specific fields.
6. **adapter_postgres_test.tmpl** — Add TODO comment on `createTest{{.NameTitle}}` for multi-arg constructors.
7. **queries.tmpl** — Add `-- TODO: add columns` comment in Update query.
8. **proto.tmpl** — Add TODO comment in message block.
9. **module.tmpl** — Add comment about EventBus being provided by shared module.
10. **domain_errors.tmpl** — Add TODO comment for additional errors.

## Unresolved Questions

- Should scaffold templates include the `PasswordHasher` dependency pattern at all, or document it as user-module-specific?
- Should the `List` result field be renamed to `Items` everywhere (including user module) to standardize the convention, or keep `Users`/`Products` as domain-named fields?
- Should event topic constants be auto-generated into `events/topics.go`, or remain a manual step (currently listed in step 6 of scaffold next-steps)?
