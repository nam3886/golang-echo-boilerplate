# Phase 1: Project Foundation

**Priority:** P0 | **Effort:** M (2-4h) | **Status:** completed
**Depends on:** none
**Completed:** 2026-03-04

## Context

- [Brainstorm Report](../reports/brainstorm-260304-1636-golang-api-boilerplate.md) — Stack decision, project structure

## Overview

Bootstrap Go 1.26 project with module structure, Uber Fx skeleton, Taskfile, Docker Compose for dev infra, config loading, and basic entrypoint.

## Key Insights

- Go 1.26 requires `go 1.26` in go.mod
- Docker base: `golang:1.26-alpine` (builder), `alpine:3.19` (runtime)
- Uber Fx app in `cmd/server/main.go` — all modules register here
- `caarlos0/env` v11 for 12-factor config
- Taskfile v3 over Makefile for cross-platform + YAML readability

## Files to Create

```
go.mod
go.sum
cmd/server/main.go
internal/shared/config/config.go
Taskfile.yml
.env.example
.gitignore
.air.toml
deploy/docker-compose.dev.yml
```

## Implementation Steps

### 1. Initialize Go module
```bash
go mod init github.com/<org>/<project>
# Set go 1.26 in go.mod
```

### 2. Create config struct
```go
// internal/shared/config/config.go
type Config struct {
    AppEnv      string `env:"APP_ENV" envDefault:"development"`
    Port        int    `env:"PORT" envDefault:"8080"`
    DatabaseURL string `env:"DATABASE_URL,required"`
    RedisURL    string `env:"REDIS_URL,required"`
    RabbitURL   string `env:"RABBITMQ_URL,required"`
    ESURL       string `env:"ELASTICSEARCH_URL" envDefault:"http://localhost:9200"`
    JWTSecret   string `env:"JWT_SECRET,required"`
    JWTAccessTTL  time.Duration `env:"JWT_ACCESS_TTL" envDefault:"15m"`
    JWTRefreshTTL time.Duration `env:"JWT_REFRESH_TTL" envDefault:"168h"`
    LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
    OTLPEndpoint string `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:"http://localhost:4317"`
    SMTPHost    string `env:"SMTP_HOST" envDefault:"localhost"`
    SMTPPort    int    `env:"SMTP_PORT" envDefault:"1025"`
    SMTPFrom    string `env:"SMTP_FROM" envDefault:"noreply@app.local"`
}

func Load() (*Config, error) {
    cfg := &Config{}
    if err := env.Parse(cfg); err != nil {
        return nil, fmt.Errorf("loading config: %w", err)
    }
    return cfg, nil
}
```

### 3. Create Fx app entrypoint
```go
// cmd/server/main.go
func main() {
    fx.New(
        fx.Provide(config.Load),
        // Phase 2: shared infrastructure modules
        // Phase 4: auth module
        // Phase 5: example module
        // Phase 6: events module
        fx.Invoke(startServer),
    ).Run()
}
```

### 4. Create Taskfile.yml
Core tasks: `dev:setup`, `dev`, `generate`, `lint`, `test`, `check`, `build`, `migrate:*`, `seed`.
See brainstorm report Dev Workflow section for full task definitions.

### 5. Create .env.example
All ENV vars with placeholder values. See brainstorm Security section.

### 6. Create .air.toml
Hot reload config: watch `.go`, `.sql`, `.proto` files. Exclude `tmp/`, `gen/`, `node_modules/`.

### 7. Create docker-compose.dev.yml
Services: PostgreSQL 16, Redis 7, RabbitMQ 3 (management), Elasticsearch 8, MailHog.
All with health checks. Volumes for persistence.

### 8. Create .gitignore
```
.env
*.pem
*.key
coverage.out
coverage.html
tmp/
bin/
vendor/
```

## Todo

- [x] Init go module with Go 1.26
- [x] Config struct with caarlos0/env
- [x] Fx app skeleton in cmd/server/main.go
- [x] Taskfile.yml with core tasks
- [x] .env.example with all vars
- [x] .air.toml for hot reload
- [x] docker-compose.dev.yml (Postgres, Redis, RabbitMQ, ES, MailHog)
- [x] .gitignore
- [x] Verify `task dev:deps` starts all containers
- [x] Verify `task dev` starts air with hot reload

## Success Criteria

- `task dev:deps` → all infra containers running + healthy
- `task dev` → Go app starts, logs "server started on :8080"
- Config loads from .env file correctly
- Fx lifecycle hooks log startup/shutdown

## Risk Assessment

- **Low risk:** Standard Go project setup, well-documented tools
- **caarlos0/env v11:** Check API changes from v10 if any

## Next Steps

→ Phase 2: Shared Infrastructure (DB pool, Redis, slog, OTel, errors, BaseModel)
