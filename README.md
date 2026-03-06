# GNHA Services

Production-ready Go API boilerplate — modular monolith.

## Quick Start

```bash
# 1. Clone & setup
git clone <repo> && cd gnha-services
cp .env.example .env
task dev:setup    # Install tools, start infra, migrate, seed

# 2. Run
task dev          # Hot reload on :8080

# 3. Test
task test                # Unit tests
task test:integration    # Integration (testcontainers)
task check               # Lint + test
```

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26+ | [go.dev](https://go.dev/dl/) |
| Docker | 24+ | [docker.com](https://docs.docker.com/get-docker/) |
| Task | 3+ | `go install github.com/go-task/task/v3/cmd/task@latest` |

All other tools (buf, sqlc, air, lefthook, goose, mockgen) are installed automatically by `task dev:setup`.

## Stack

Go 1.26 | Echo | Connect RPC | Watermill | PostgreSQL | Redis | RabbitMQ | SigNoz

## Architecture

Simplified Hexagonal (modular monolith) — see [docs/architecture.md](docs/architecture.md)

```
cmd/server/          # Entrypoint
internal/
  shared/            # Cross-cutting: config, DB, auth, middleware, events, cron
  modules/
    user/            # Example module
      domain/        # Entity, repository interface, errors
      app/           # Command/query handlers
      adapters/      # Postgres (sqlc), gRPC (Connect)
    audit/           # Audit trail subscriber
    notification/    # Email notification subscriber
proto/               # Protobuf definitions
db/                  # Migrations + SQL queries
gen/                 # Generated code (proto, sqlc)
deploy/              # Docker Compose, Traefik
```

## Code Gen

```bash
task generate          # Proto (buf) + SQL (sqlc)
task generate:proto    # Protobuf only
task generate:sqlc     # SQL only
```

## Adding a Module

See [docs/adding-a-module.md](docs/adding-a-module.md)

## Dev Services

| Service | Port | UI |
|---------|------|----|
| App | :8080 | http://localhost:8080/swagger/ |
| PostgreSQL | :5432 | — |
| Redis | :6379 | — |
| RabbitMQ | :5672 | http://localhost:15672 (guest/guest) |
| Elasticsearch | :9200 | — |
| MailHog | :1025 | http://localhost:8025 |

## API

- Connect RPC on :8080
- Swagger UI: http://localhost:8080/swagger/ (dev only)
- Proto definitions: `proto/<module>/v1/*.proto`

## Monitoring

```bash
task monitor:up    # Start SigNoz → http://localhost:3301
```

## Deploy

```bash
task docker:build    # Build image
# GitLab CI handles staging/production deploy
```
