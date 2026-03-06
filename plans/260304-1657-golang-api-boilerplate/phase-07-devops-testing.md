# Phase 7: DevOps & Testing

**Priority:** P1 | **Effort:** L (4-8h) | **Status:** completed
**Depends on:** Phase 5
**Completed:** 2026-03-04

## Context

- [Brainstorm Report](../reports/brainstorm-260304-1636-golang-api-boilerplate.md) — DevOps, Testing, SigNoz sections

## Overview

Production-ready Dockerfile, GitLab CI/CD pipeline, Docker Compose for production (Traefik), SigNoz monitoring setup, and testing infrastructure (testcontainers, test helpers, golden files, example tests).

## Files to Create

```
# DevOps
Dockerfile
deploy/docker-compose.yml              # Production (app + Traefik + infra)
deploy/docker-compose.monitor.yml      # SigNoz
deploy/traefik/traefik.yml             # Traefik static config
.gitlab-ci.yml

# Testing
internal/shared/testutil/db.go         # Testcontainers Postgres helper
internal/shared/testutil/redis.go      # Testcontainers Redis helper
internal/shared/testutil/rabbitmq.go   # Testcontainers RabbitMQ helper
internal/shared/testutil/fixtures.go   # Test data factory
internal/shared/testutil/golden.go     # Golden file assertion
internal/modules/user/adapters/postgres/repository_test.go  # Integration test
internal/modules/user/app/create_user_test.go               # Unit test
internal/modules/user/adapters/grpc/handler_test.go          # E2E API test

# Seeder
cmd/seed/main.go
```

## Implementation Steps

### 1. Dockerfile — multi-stage
```dockerfile
# Stage 1: Build
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /server ./cmd/server

# Stage 2: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata curl
COPY --from=builder /server /server
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
    CMD curl -f http://localhost:8080/healthz || exit 1
ENTRYPOINT ["/server"]
```

### 2. Production Docker Compose
```yaml
# deploy/docker-compose.yml
services:
  app:
    image: ${CI_REGISTRY_IMAGE:-myapp}:${TAG:-latest}
    restart: unless-stopped
    env_file: .env
    depends_on:
      postgres: { condition: service_healthy }
      redis: { condition: service_healthy }
      rabbitmq: { condition: service_healthy }
    deploy:
      replicas: 2
      update_config:
        parallelism: 1
        delay: 10s
        order: start-first
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.app.rule=Host(`${APP_DOMAIN}`)"
      - "traefik.http.routers.app.tls.certresolver=letsencrypt"
      - "traefik.http.services.app.loadbalancer.server.port=8080"
      - "traefik.http.services.app.loadbalancer.healthcheck.path=/healthz"

  traefik:
    image: traefik:v3.0
    restart: unless-stopped
    ports: ["80:80", "443:443"]
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./traefik:/etc/traefik
      - traefik-certs:/certs

  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    volumes: [pgdata:/var/lib/postgresql/data]
    environment:
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: ${DB_NAME}
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER}"]
      interval: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]

  rabbitmq:
    image: rabbitmq:3-management-alpine
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "rabbitmq-diagnostics", "check_running"]

volumes:
  pgdata:
  traefik-certs:
```

### 3. SigNoz Docker Compose
```yaml
# deploy/docker-compose.monitor.yml
# Use official SigNoz docker-compose
# https://signoz.io/docs/install/docker/
# Clone signoz/deploy into deploy/signoz/ or reference directly
# App config: OTEL_EXPORTER_OTLP_ENDPOINT=http://signoz-otel-collector:4317
```
Taskfile task:
```yaml
monitor:up:
  desc: Start SigNoz monitoring
  cmds:
    - docker compose -f deploy/docker-compose.monitor.yml up -d
monitor:down:
  cmds:
    - docker compose -f deploy/docker-compose.monitor.yml down
```

### 4. GitLab CI/CD
```yaml
# .gitlab-ci.yml
stages: [quality, test, build, deploy]

variables:
  DOCKER_IMAGE: $CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA
  POSTGRES_DB: test_db
  POSTGRES_USER: test
  POSTGRES_PASSWORD: test
  DATABASE_URL: "postgres://test:test@postgres:5432/test_db?sslmode=disable"
  REDIS_URL: "redis://redis:6379/0"
  RABBITMQ_URL: "amqp://guest:guest@rabbitmq:5672/"

lint:
  stage: quality
  image: golangci/golangci-lint:v1.62
  script: [golangci-lint run ./...]
  rules: [{ if: $CI_MERGE_REQUEST_IID }]

generated-check:
  stage: quality
  image: golang:1.26-alpine
  before_script:
    - go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
    - go install github.com/bufbuild/buf/cmd/buf@latest
  script:
    - buf generate && sqlc generate
    - git diff --exit-code gen/
  rules: [{ if: $CI_MERGE_REQUEST_IID }]

unit-test:
  stage: test
  image: golang:1.26-alpine
  script:
    - go test -race -count=1 -coverprofile=coverage.out ./internal/...
    - go tool cover -func=coverage.out
  coverage: '/total:\s+\(statements\)\s+(\d+\.\d+)%/'
  rules:
    - if: $CI_MERGE_REQUEST_IID
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

integration-test:
  stage: test
  image: golang:1.26-alpine
  services:
    - postgres:16-alpine
    - redis:7-alpine
    - rabbitmq:3-alpine
  script:
    - go test -race -count=1 -tags=integration ./...
  rules:
    - if: $CI_MERGE_REQUEST_IID
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

build:
  stage: build
  image: docker:24
  services: [docker:24-dind]
  script:
    - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
    - docker build -t $DOCKER_IMAGE .
    - docker tag $DOCKER_IMAGE $CI_REGISTRY_IMAGE:latest
    - docker push $DOCKER_IMAGE
    - docker push $CI_REGISTRY_IMAGE:latest
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

deploy-staging:
  stage: deploy
  image: alpine:3.19
  before_script:
    - apk add --no-cache openssh-client
    - eval $(ssh-agent -s) && echo "$SSH_PRIVATE_KEY" | ssh-add -
  script:
    - ssh -o StrictHostKeyChecking=no deploy@$STAGING_HOST
        "cd /app && docker compose pull && docker compose up -d --remove-orphans"
  environment: { name: staging, url: "https://staging.$APP_DOMAIN" }
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

deploy-production:
  stage: deploy
  extends: deploy-staging
  script:
    - ssh -o StrictHostKeyChecking=no deploy@$PROD_HOST
        "cd /app && docker compose pull && docker compose up -d --remove-orphans"
  environment: { name: production, url: "https://$APP_DOMAIN" }
  rules:
    - if: $CI_COMMIT_TAG
  when: manual
```

### 5. Testcontainers helpers
```go
// internal/shared/testutil/db.go
func NewTestPostgres(t *testing.T) *pgxpool.Pool {
    t.Helper()
    ctx := context.Background()
    pg, err := postgres.Run(ctx, "postgres:16-alpine",
        postgres.WithDatabase("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("ready to accept connections").WithOccurrence(2),
        ),
    )
    t.Cleanup(func() { pg.Terminate(ctx) })
    connStr, _ := pg.ConnectionString(ctx, "sslmode=disable")
    pool, _ := pgxpool.New(ctx, connStr)
    // Run migrations
    runMigrations(t, connStr)
    return pool
}
```

### 6. Test examples
```go
// Unit test — mock repository
// internal/modules/user/app/create_user_test.go
func TestCreateUser_Success(t *testing.T) {
    repo := &mockUserRepo{users: map[string]*domain.User{}}
    handler := app.NewCreateUserHandler(repo, &mockHasher{})
    user, err := handler.Handle(ctx, app.CreateUserCmd{
        Email: "test@test.com", Name: "Test", Password: "12345678", Role: "member",
    })
    assert.NoError(t, err)
    assert.Equal(t, "test@test.com", user.Email())
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
    // ... assert ErrAlreadyExists
}

// Integration test — real DB
// internal/modules/user/adapters/postgres/repository_test.go
//go:build integration
func TestPgUserRepository_Create(t *testing.T) {
    pool := testutil.NewTestPostgres(t)
    repo := postgres.NewPgUserRepository(pool)
    // ... test with real DB
}

// E2E API test — Connect httptest
// internal/modules/user/adapters/grpc/handler_test.go
func TestUserService_CreateUser(t *testing.T) {
    pool := testutil.NewTestPostgres(t)
    // Wire full handler chain
    handler := grpc.NewUserServiceHandler(...)
    _, h := userv1connect.NewUserServiceHandler(handler)
    server := httptest.NewServer(h)
    defer server.Close()

    client := userv1connect.NewUserServiceClient(http.DefaultClient, server.URL)
    resp, err := client.CreateUser(ctx, connect.NewRequest(&userv1.CreateUserRequest{
        Email: "test@test.com", Name: "Test", Password: "12345678", Role: "member",
    }))
    assert.NoError(t, err)
    assert.Equal(t, "test@test.com", resp.Msg.Email)
}
```

### 7. Database seeder
```go
// cmd/seed/main.go
func main() {
    fx.New(
        shared.Module,
        user.Module,
        fx.Invoke(runSeed),
    ).Run()
}

func runSeed(repo domain.UserRepository, hasher auth.PasswordHasher) {
    seeds := []struct{ email, name, password, role string }{
        {"admin@app.local", "Admin", "admin123", "admin"},
        {"member@app.local", "Member", "member123", "member"},
        {"viewer@app.local", "Viewer", "viewer123", "viewer"},
    }
    for _, s := range seeds {
        // Check exists, skip if already seeded (idempotent)
        // Hash password, create user
    }
}
```

## Todo

- [x] Dockerfile (multi-stage, ~15MB image, healthcheck)
- [x] Production docker-compose.yml (app + Traefik + infra)
- [x] Traefik config (auto SSL, health-check routing)
- [x] SigNoz docker-compose.monitor.yml
- [x] .gitlab-ci.yml (lint, generated-check, unit-test, integration-test, build, deploy)
- [x] Testcontainers helper: Postgres (with migrations)
- [x] Testcontainers helper: Redis
- [x] Testcontainers helper: RabbitMQ
- [x] Test fixtures factory
- [x] Golden file assertion helper
- [x] Unit test example (CreateUser handler)
- [x] Integration test example (PgUserRepository)
- [x] E2E API test example (Connect httptest)
- [x] Event handler test (Watermill GoChannel)
- [x] Database seeder (cmd/seed/main.go)
- [x] Taskfile tasks: test, test:integration, test:coverage, seed, monitor:up
- [x] Verify: `task test` passes, `task test:integration` passes
- [x] Verify: Docker build produces working image
- [x] Verify: GitLab CI pipeline runs (dry-run)

## Success Criteria

- Docker image builds, <20MB, healthcheck works
- `task test` runs unit tests, `task test:integration` runs with real DB
- CI pipeline: lint → test → build → deploy stages working
- SigNoz receives traces/logs/metrics from app
- Testcontainers start/stop cleanly, no dangling containers
- Seeder is idempotent (run multiple times without error)
- Zero-downtime deploy: Traefik routes to healthy instance during update

## Next Steps

→ Phase 8: Docs & DX Polish (README, lefthook, golangci-lint config)
