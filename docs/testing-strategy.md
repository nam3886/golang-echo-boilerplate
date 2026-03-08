# Testing Strategy

Testing conventions and patterns for GNHA Services.

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
    mockRepo.EXPECT().GetByEmail(gomock.Any(), "new@example.com").Return(nil, sharederr.ErrNotFound())
    mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

    handler := NewCreateUserHandler(mockRepo, &testutil.StubHasher{}, &testutil.NoopPublisher{})
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

## Test Fixtures

`internal/shared/testutil/fixtures.go` provides predefined test data:

```go
testutil.DefaultUserFixture()  // member role
testutil.AdminUserFixture()    // admin role
testutil.ViewerUserFixture()   // viewer role
```

## Testcontainers Helpers

| Helper | Package | Container |
|--------|---------|-----------|
| `NewTestPostgres(t)` | `testutil` | postgres:16-alpine |
| `NewTestRedis(t)` | `testutil` | redis:7-alpine |
| `NewTestRabbitMQ(t)` | `testutil` | rabbitmq:3-management-alpine |
| `RunMigrations(t, pool)` | `testutil` | Applies goose migrations on test DB |

All containers auto-cleanup via `t.Cleanup`.

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

Coverage extracted via regex, reported as Cobertura artifact in GitLab.

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
