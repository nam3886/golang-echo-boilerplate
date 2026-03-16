# Testing Strategy

Testing conventions and patterns for Golang Echo Boilerplate.

## Test Types

| Type | Tag | Infra | Runner |
|------|-----|-------|--------|
| Unit | (none) | No — mocks only | `task test` |
| Integration | `//go:build integration` | Testcontainers (real Postgres/Redis/RabbitMQ) | `task test:integration` |

## When to Use What

### Unit Tests

Test **logic** without infrastructure.

- Domain entity constructors and validation (`domain/user_test.go`)
- App handlers with mock repositories (`app/create_user_test.go`)
- Business rules, edge cases, error paths
- Pure functions and utilities

**Pattern:** gomock for repository interfaces, stub for simple dependencies (hasher, publisher).

```go
func TestCreateUserHandler_Success(t *testing.T) {
    ctrl := gomock.NewController(t)
    mockRepo := mocks.NewMockUserRepository(ctrl)
    mockRepo.EXPECT().GetByEmail(gomock.Any(), "new@example.com").Return(nil, domain.ErrUserNotFound())
    mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

    bus := events.NewEventBus(&testutil.NoopPublisher{})
    handler := app.NewCreateUserHandler(mockRepo, &testutil.StubHasher{}, bus)
    user, err := handler.Handle(ctx, cmd)
    // assert...
}
```

### Integration Tests

Test **infrastructure boundaries** with real services.

- Repository implementations (Postgres queries, transactions, constraints)
- SQL query correctness (joins, pagination, soft-delete filters)
- Event publishing/subscribing with RabbitMQ
- Redis caching and rate limiting

**Pattern:** testcontainers setup in test helper, real DB with migrations applied.

```go
//go:build integration

func TestPgUserRepository_Create(t *testing.T) {
    pool := testutil.NewTestPostgres(t)
    testutil.RunMigrations(t, pool)
    repo := NewPgUserRepository(pool)

    user := createTestUser(t, "test@example.com")
    err := repo.Create(ctx, user)
    // assert...
}
```

## Test Organization

```
internal/modules/user/
├── domain/
│   └── user_test.go              # Unit: entity logic
├── app/
│   └── create_user_test.go       # Unit: handler + mocks
└── adapters/postgres/
    └── repository_test.go        # Integration: real DB
```

**Rule:** Test file lives next to the code it tests. No separate `tests/` directory for unit tests.

## Mock Generation

Repository interfaces include `//go:generate` directives:

```go
//go:generate mockgen -source=repository.go -destination=../../../shared/mocks/mock_user_repository.go -package=mocks
```

Run: `task generate:mocks` (or `go generate ./...`).

Output: `internal/shared/mocks/mock_*.go` — committed to repo.

## Testcontainers Helpers

| Helper | Package | Container |
|--------|---------|-----------|
| `NewTestPostgres(t)` | `testutil` | postgres:16-alpine |
| `NewTestRedis(t)` | `testutil` | redis:7-alpine |
| `NewTestRabbitMQ(t)` | `testutil` | rabbitmq:3-management-alpine |
| `RunMigrations(t, pool)` | `testutil` | Applies goose migrations on test DB |

All containers auto-cleanup via `t.Cleanup`.

## Test Utilities Reference

### Stubs (Lightweight replacements)

| Stub | Package | Purpose |
|------|---------|---------|
| `StubHasher` | `testutil` | Password hashing that returns input as-is (no crypto) |
| `FailHasher` | `testutil` | Password hashing that always returns an error |
| `NoopPublisher` | `testutil` | Event publisher that does nothing (silent drop) |
| `CapturingPublisher` | `testutil` | Event publisher that records published events for assertion |
| `FailPublisher` | `testutil` | Event publisher that always returns an error |

### Testcontainers (Real infrastructure)

| Helper | Package | Container | Cleanup |
|--------|---------|-----------|---------|
| `NewTestPostgres(t)` | `testutil` | postgres:16-alpine | Auto via `t.Cleanup` |
| `NewTestRedis(t)` | `testutil` | redis:7-alpine | Auto via `t.Cleanup` |
| `NewTestRabbitMQ(t)` | `testutil` | rabbitmq:3-management-alpine | Auto via `t.Cleanup` |
| `NewTestElasticsearch(t)` | `testutil` | elasticsearch:8.17.0 | Auto via `t.Cleanup` |

### Helpers (Utilities)

| Helper | Package | Usage |
|--------|---------|-------|
| `Ptr[T](v)` | `testutil` | Generic helper: `Ptr(42)` returns `*int` |
| `RunMigrations(t, pool)` | `testutil` | Apply goose migrations on test DB |

## Running Tests

```bash
task test              # Unit tests with -race and coverage
task test:integration  # Integration tests (Docker required)
task test:coverage     # HTML coverage report
task check             # Lint + unit tests (pre-merge gate)
```

## CI Pipeline

| Job | Stage | Trigger | What |
|-----|-------|---------|------|
| `unit-test` | test | MR + main | `-race -count=1 -coverprofile` |
| `integration-test` | test | main + tags | Real Postgres/Redis/RabbitMQ services |

Coverage extracted via regex in CI pipeline (GitHub Actions or equivalent).

## Git Hooks

- **Pre-push:** `go test -race -count=1 ./internal/...` — must pass before push
- **Pre-commit:** Lint + generated code check (no test run — too slow for commits)

## Guidelines

1. **No mocks for infrastructure** — use testcontainers for DB/cache/messaging
2. **Mock only domain interfaces** — repository, event publisher
3. **Table-driven tests** for validation and edge cases
4. **One assertion per test** when possible — clearer failure messages
5. **Test error paths** — not just happy path
6. **`-race` always** — detect data races early
7. **`-count=1`** — disable test caching for fresh runs
