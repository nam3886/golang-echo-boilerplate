# Boilerplate DX Fixes

status: pending
created: 2026-03-06
estimated: 45 min

## Context

Review report: `plans/reports/review-260306-0923-boilerplate-dx-comprehensive.md`
Findings verified against actual source code. C1 (queries.tmpl) was false alarm — template already has GetByIDForUpdate.
I9 (scaffold underscore) works correctly — underscore in Go package names is valid.

## Verified Fix List

| # | Item | File(s) | Effort |
|---|------|---------|--------|
| C3 | uuid.MustParse → parseUserID | `user/adapters/postgres/repository.go:101` | 2 min |
| I1 | Import ordering | `audit/subscriber.go`, `notification/subscriber.go` | 2 min |
| I2 | slog.Error → slog.ErrorContext | `notification/subscriber.go:41,45` | 2 min |
| I4 | code-standards.md examples wrong | `docs/code-standards.md` lines 392-414, 649-658, 628-632 | 15 min |
| I6-I7 | README prerequisites + services | `README.md` | 10 min |
| I12 | Stale model/ in architecture | `docs/architecture.md:19` | 1 min |
| I13 | Missing error codes in doc | `docs/error-codes.md` | 2 min |

## Phases

### Phase 1: Code Fixes (5 min)

#### C3 — Fix uuid.MustParse panic

File: `internal/modules/user/adapters/postgres/repository.go:101`

```go
// BEFORE (line 101):
ID: uuid.MustParse(string(user.ID())),

// AFTER:
uid, err := parseUserID(user.ID())
if err != nil {
    return err
}
// then use uid in CreateUserParams
```

Must restructure Create method to parse ID first (like all other methods do).

#### I1 — Fix import ordering

**audit/subscriber.go** — move `github.com/ThreeDotsLabs/watermill/message` before `github.com/gnha/...`:

```go
import (
    "encoding/json"
    "log/slog"

    "github.com/ThreeDotsLabs/watermill/message"
    "github.com/google/uuid"
    sqlcgen "github.com/gnha/gnha-services/gen/sqlc"
    "github.com/gnha/gnha-services/internal/shared/events"
)
```

**notification/subscriber.go** — same fix:

```go
import (
    "bytes"
    "encoding/json"
    "html/template"
    "log/slog"

    "github.com/ThreeDotsLabs/watermill/message"
    "github.com/gnha/gnha-services/internal/shared/events"
)
```

#### I2 — Fix notification slog context

File: `internal/modules/notification/subscriber.go`

Lines 41 and 45 — ctx is available (extracted at line 39). Change:
```go
// Line 41:
slog.ErrorContext(ctx, "notification: failed to send email", "err", err, "to", event.Email)
// Line 45:
slog.InfoContext(ctx, "notification: welcome email sent", "to", event.Email)
```

Note: Lines 29 and 35 are BEFORE ctx extraction — leave as `slog.Error` (correct).

Also fixes M1 (missing module prefix in success log).

### Phase 2: Doc Fixes (15 min)

#### I4 — code-standards.md

**Fix 1 (lines 392-414):** Replace stored-q pattern with per-method pattern matching actual code:

```go
type PgUserRepository struct {
    pool *pgxpool.Pool
}

func NewPgUserRepository(pool *pgxpool.Pool) *PgUserRepository {
    return &PgUserRepository{pool: pool}
}

func (r *PgUserRepository) Create(ctx context.Context, user *domain.User) error {
    uid, err := parseUserID(user.ID())
    if err != nil {
        return err
    }
    q := sqlcgen.New(r.pool)
    _, err = q.CreateUser(ctx, sqlcgen.CreateUserParams{
        ID:       uid,
        Email:    user.Email(),
        Name:     user.Name(),
        Password: user.Password(),
        Role:     string(user.Role()),
    })
    // Handle unique constraint violation...
```

**Fix 2 (lines 649-658):** fx.Module — use separate `fx.Provide()` calls:

```go
var Module = fx.Module("user",
    fx.Provide(
        fx.Annotate(
            postgres.NewPgUserRepository,
            fx.As(new(domain.UserRepository)),
        ),
    ),
    fx.Provide(app.NewCreateUserHandler),
    fx.Provide(app.NewGetUserHandler),
    // ...
    fx.Provide(grpc.NewUserServiceHandler),
    fx.Invoke(grpc.RegisterRoutes),
)
```

**Fix 3 (line 588):** Unit test — replace `events.NewEventBus()` with stub:

```go
type noopPublisher struct{}
func (n *noopPublisher) Publish(topic string, messages ...*message.Message) error {
    return nil
}

hasher := &mockHasher{}
handler := app.NewCreateUserHandler(repo, hasher, &noopPublisher{})
```

**Fix 4 (lines 628-631):** testutil function names:

```go
// BEFORE:
pool := testutil.NewTestDB(t, ctx)
defer pool.Close()
repo := postgres.NewRepository(pool)

// AFTER:
pool := testutil.NewTestPostgres(t)
repo := postgres.NewPgUserRepository(pool)
```

#### I12 — architecture.md line 19

Delete `model/              # Shared base models` line.

#### I13 — error-codes.md

Add 2 missing codes:

```
| FAILED_PRECONDITION | 412 | Precondition not met |
| UNAVAILABLE | 503 | Service temporarily unavailable |
```

### Phase 3: README Enhancement (10 min)

#### I6-I7 — Add Prerequisites + Dev Services

Add after "Quick Start" section:

```markdown
## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26+ | [go.dev](https://go.dev/dl/) |
| Docker | 24+ | [docker.com](https://docs.docker.com/get-docker/) |
| Task | 3+ | `go install github.com/go-task/task/v3/cmd/task@latest` |

All other tools (buf, sqlc, air, lefthook, goose, mockgen) are installed automatically by `task dev:setup`.

## Dev Services

| Service | Port | UI |
|---------|------|----|
| App | :8080 | http://localhost:8080/swagger/ |
| PostgreSQL | :5432 | — |
| Redis | :6379 | — |
| RabbitMQ | :5672 | http://localhost:15672 (guest/guest) |
| Elasticsearch | :9200 | — |
| MailHog | :1025 | http://localhost:8025 |
```

## Success Criteria

- [ ] `go build ./...` passes
- [ ] No uuid.MustParse in codebase
- [ ] Import order: stdlib → third-party → internal in all files
- [ ] code-standards.md examples match actual code
- [ ] README has prerequisites + dev services
- [ ] error-codes.md has all 8 codes
- [ ] architecture.md no stale references

## Dropped Items (verified not issues)

- **C1** (queries.tmpl GetByIDForUpdate): Already exists — reviewer error
- **I9** (scaffold underscore names): `order_item` → `order_itemv1connect` is valid Go
- **C2** (EventBus in scaffold): Kept as TODO — YAGNI, not all modules need events
- **C4** (monitor compose missing): Skip until monitoring needed
