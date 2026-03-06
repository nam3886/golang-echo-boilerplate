# Go API Boilerplate Trends & Frameworks (2025-2026)

Date: 2026-03-04

---

## 1. Framework Comparison

### HTTP Frameworks (REST)

| Framework | Stars | Adoption | Notes |
|-----------|-------|----------|-------|
| **Gin** | ~81k | 48% devs | De facto standard. Large ecosystem. Non-idiomatic context type. |
| **Fiber** | ~35k | 11% | FastHTTP-based (NOT net/http compat). Fastest raw throughput. Express-like DX. |
| **Echo** | ~31k | 16% | Clean error handling, good OpenAPI integration, idiomatic. Enterprise-friendly. |
| **Chi** | ~18k | ~12% | Fully net/http compatible, composable middleware, stdlib-first teams love it. |
| **gorilla/mux** | ~21k | declining | Archived 2023, still in production but avoid for new projects. |
| **net/http** | stdlib | - | Gaining usage post-Go 1.22 (enhanced routing). Valid for simple services. |

**Verdict:**
- New project, team knows Go well → **Chi** or **Echo** (idiomatic, stdlib-compat)
- Fastest time-to-market → **Gin** (largest community, most tutorials)
- High-throughput, Express familiarity → **Fiber** (but watch net/http incompatibility)
- Go 1.22+ simple services → consider **net/http** directly (native method+path routing)

### Microservice / RPC Frameworks

| Framework | Stars | Origin | Approach |
|-----------|-------|--------|----------|
| **go-kratos** | ~23k | Bilibili | Protobuf-first, HTTP+gRPC, plug-in ORM, clean arch |
| **go-zero** | ~29k | 微服务 | Codegen-heavy (goctl), built-in circuit breaker/rate-limit, batteries-included |
| **GoFrame** | ~11k | - | Full-stack, ORM included, opinionated |
| **Encore** | ~8k | SaaS | Comment-annotation API, auto-infra provisioning, SaaS lock-in risk |

**Verdict for microservices:**
- Proto-first, team control over ORM → **go-kratos**
- Max built-in productivity, codegen workflow → **go-zero**
- Encore = interesting but vendor lock-in; avoid unless fully bought in

### gRPC / Protocol

| Tool | Stars | Role |
|------|-------|------|
| **connect-go** (buf.build) | ~4k | Modern gRPC replacement; works over HTTP/1.1+HTTP/2; compatible with gRPC clients |
| **gRPC-gateway** | ~18k | Proto annotation → REST reverse proxy (separate process) |
| **vanguard-go** | ~700 | In-process REST+gRPC+Connect bridge (no separate proxy) |

**Verdict:**
- New proto-based service → **connect-go** (simpler, debuggable with curl, no proxy)
- Need REST bridge for existing gRPC → **gRPC-gateway**
- Want REST+gRPC in-process without proxy → **vanguard-go**

---

## 2. Go Project Layout (2025 Consensus)

### Official Guidance (go.dev/doc/modules/layout)

```
# Server project (recommended by official docs)
myproject/
  cmd/
    api/      main.go
    worker/   main.go
  internal/
    user/       # domain-organized, NOT layer-organized
    payment/
    middleware/
  api/          # OpenAPI specs, proto files
  config/       # config files
  scripts/
  go.mod
  go.sum
```

### Key 2025 Shifts

- **`internal/` by domain, not by layer**: `internal/user/` not `internal/handlers/`
- **`pkg/` is contested**: Official docs don't recommend it; use only for genuinely public-importable libs
- **`util/`, `common/`, `helpers/`**: Code smell — name packages by what they do, not what they are
- **Start flat, evolve**: Begin with just `main.go` + `go.mod`; add dirs when justified
- **golang-standards/project-layout**: Community-created, NOT official. Useful reference, not law

### Monorepo with go.work (Go 1.18+)

```
monorepo/
  go.work
  services/
    api/        go.mod
    worker/     go.mod
    scheduler/  go.mod
  pkg/
    shared/     go.mod   # shared libs across services
  proto/                 # shared proto definitions
```

`go.work` coordinates local module resolution without publish cycles. Use for multi-service repos where services share internal packages actively in development.

---

## 3. Popular Boilerplate Projects (GitHub)

| Project | Stars | Stack | Status |
|---------|-------|-------|--------|
| **evrone/go-clean-template** | ~7k | Gin, Clean Arch | Actively maintained |
| **bxcodec/go-clean-arch** | ~9k | Echo/Gin, Clean Arch | Widely referenced |
| **qiangxue/go-rest-api** | ~3.5k | Echo, SOLID, ozzo-validation | Active |
| **diygoapi** | ~611 | net/http, no framework | Maintained 2025 |
| **codoworks/go-boilerplate** | ~300 | Echo, Postgres/SQLite | Active |
| **syahidfrd/go-boilerplate** | ~46 | Gin, testcontainers, Redis | 2024-2025 |
| **barekit/golang-boilerplate** | small | Gin, GORM, Uber Fx, JWT | Recent |

**Patterns common across top boilerplates:**
- Clean Architecture / Hexagonal layers
- Dependency injection (manual or Uber Fx / Wire)
- Docker + Makefile
- JWT auth
- Postgres + migration tool
- Swagger docs

---

## 4. DX Toolchain

### Hot Reload
| Tool | Notes |
|------|-------|
| **air** (air-verse/air) | Standard choice, `.air.toml` config, most used |
| **wgo** | Simpler, supports arbitrary commands, good for custom workflows |

### Code Generation
| Tool | Use Case |
|------|----------|
| **sqlc** | SQL → type-safe Go code. Uses pgx v5. Pairs with golang-migrate |
| **buf + protoc-gen-go** | Protobuf codegen, lint, breaking change detection |
| **oapi-codegen** | OpenAPI spec → Go server/client stubs |
| **wire** (google) | Compile-time DI via codegen |
| **mockery** | Interface → mock gen for testing |

### Linting
- **golangci-lint**: Industry standard aggregator. v2.x merged staticcheck into it.
- Recommended linters: `staticcheck`, `gosec`, `errcheck`, `govet`, `revive`, `gocyclo`, `misspell`
- Config via `.golangci.yml`

### Testing
- `testing` (stdlib) + **testify** (27% usage) for assertions
- **testcontainers-go**: Real DB/Redis in tests, no mocking infra
- **gomock** / **mockery**: Interface mocking

---

## 5. Batteries-Included Library Stack (2025 Recommended)

### Logging
| Library | Notes |
|---------|-------|
| **slog** (stdlib, Go 1.21+) | First choice for new projects. No deps. JSON + text handlers. |
| **zerolog** | Zero-alloc, chain API, high perf. Good for high-throughput services. |
| **zap** (Uber) | Mature, structured, widely used in enterprise. Slightly more boilerplate. |
| Logrus | Legacy; use slog instead |

### Config
| Library | Notes |
|---------|-------|
| **viper** | Most popular, supports env/file/remote. Heavy but feature-rich. |
| **cleanenv** | Lighter alternative. Struct tags + env/yaml. Recommended for smaller services. |
| **envconfig** | Env-only, minimal. Good for 12-factor apps. |

### Database
| Library | Notes |
|---------|-------|
| **pgx/v5** | Preferred Postgres driver (lib/pq in maintenance mode) |
| **sqlc** | SQL-first, type-safe. Best for teams comfortable with SQL. |
| **GORM** | ORM, developer-friendly, slower for complex queries |
| **sqlx** | Thin wrapper over database/sql, good middle ground |
| **ent** | Schema-first ORM, good for complex relations |

### Migrations
| Tool | Notes |
|------|-------|
| **golang-migrate** | Simple, widely used, SQL up/down files, supports pgx |
| **goose** | Supports Go-based migrations. More flexible. |
| **Atlas** | Declarative schema, HCL/SQL, integrates with GORM. More complex but powerful. |

**Practical recommendation:** golang-migrate + sqlc + pgx for SQL-first; goose + GORM for ORM-first

### Auth
| Library | Notes |
|---------|-------|
| **golang-jwt/jwt v5** | Standard JWT. Most used. |
| **o1egl/paseto** | More secure token format, fewer algorithm choices (safe by design). |
| **go-chi/jwtauth** | JWT middleware for Chi |

### Validation
- **go-playground/validator v10**: Struct tag-based, ~16k stars, most adopted. Use `WithRequiredStructEnabled()` for v11-compat behavior.
- **ozzo-validation**: Fluent code-based API (no struct tags), cleaner for complex rules

### API Docs / OpenAPI
| Tool | Notes |
|------|-------|
| **swaggo/swag** | Most popular. Annotation comments → Swagger 2.0/OpenAPI 3.0. Supports Gin/Echo/Fiber. |
| **oapi-codegen** | Generate server from OpenAPI spec (contract-first) |
| **fuego** | Framework that auto-generates OpenAPI from Go types (emerging) |
| **huma** | Framework-agnostic, OpenAPI 3.1 from Go types |

### Error Handling
- Pattern: custom error types with `errors.Is`/`errors.As`; structured error response DTOs
- **pkg/errors** is legacy — use stdlib `fmt.Errorf` with `%w` wrapping
- Centralize HTTP error mapping in middleware

### Middleware (common set)
- Recovery (panic handler)
- Request ID (`google/uuid`)
- Structured logging with request context
- Rate limiting (`golang.org/x/time/rate` or `ulule/limiter`)
- CORS (`rs/cors`)
- Auth (JWT middleware)
- Timeout

---

## 6. Practical Recommendations by Scale

### Small Service / Prototype
```
net/http (Go 1.22+) or Chi
slog
cleanenv
pgx + golang-migrate
golang-jwt/jwt v5
go-playground/validator v10
air (hot reload)
golangci-lint
```

### Medium API (Team of 2-5)
```
Gin or Echo
slog or zerolog
viper
sqlc + pgx + golang-migrate
go-playground/validator v10
swaggo/swag
Makefile + air
golangci-lint + testify + testcontainers
Wire or Uber Fx (DI)
```

### Microservices
```
go-kratos (proto-first) or go-zero (codegen-heavy)
buf + connect-go or gRPC-gateway
zerolog or zap
Atlas (schema migrations)
go.work monorepo
OpenTelemetry (tracing/metrics)
Prometheus metrics
```

---

## 7. What's Trending / Shifting

- **slog over zap/logrus** for new projects (stdlib, no dep)
- **sqlc over GORM** preference rising among experienced Go devs (explicit SQL)
- **pgx/v5 over lib/pq** (lib/pq maintenance mode)
- **connect-go gaining** over traditional gRPC for new proto-based APIs
- **testcontainers-go** replacing in-memory mocks for integration tests
- **Go 1.22 net/http** enhanced routing reducing need for chi/gorilla for simple cases
- **Atlas** growing as migration tool, especially ORM-integrated teams
- **golangci-lint v2** consolidating staticcheck, gosimple into one linter

---

## Unresolved Questions

1. Exact Sponge framework adoption — only appears in Chinese-language community; unclear Western usage.
2. connect-go production war stories beyond Buf's own marketing — limited external case studies.
3. go-zero community sustainability: heavily Chinese-market-driven; English docs/community less active.
4. Huma vs fuego vs oapi-codegen for contract-first API generation — no clear winner yet; all maturing.
5. Wire vs Uber Fx: Wire deprecated in 2024 by Google internally — Uber Fx or manual DI preferred now.

---

## Sources

- [JetBrains Go Ecosystem 2025](https://blog.jetbrains.com/go/2025/11/10/go-language-trends-ecosystem-2025/)
- [Encore: Best Go Backend Frameworks 2026](https://encore.dev/articles/best-go-backend-frameworks)
- [LogRocket: Top Go Frameworks 2025](https://blog.logrocket.com/top-go-frameworks-2025/)
- [Official Go Module Layout Docs](https://go.dev/doc/modules/layout)
- [No-Nonsense Go Package Layout](https://laurentsv.com/blog/2024/10/19/no-nonsense-go-package-layout.html)
- [Go Project Structure 2025](https://www.glukhov.org/post/2025/12/go-project-structure/)
- [Connect: A Better gRPC](https://buf.build/blog/connect-a-better-grpc)
- [ConnectRPC/connect-go](https://github.com/connectrpc/connect-go)
- [air-verse/air](https://github.com/air-verse/air)
- [sqlc docs](https://docs.sqlc.dev/)
- [Atlas vs golang-migrate](https://atlasgo.io/blog/2025/04/06/atlas-and-golang-migrate)
- [golangci-lint linters](https://golangci-lint.run/docs/linters/)
- [Kratos vs go-zero comparison](https://medium.com/@g.zhufuyi/hardcore-multi-dimensional-comparison-kratos-vs-go-zero-vs-goframe-vs-sponge-which-go-8c168d75fe36)
