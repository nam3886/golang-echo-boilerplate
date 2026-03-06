# Go API Framework Ecosystem Research (2025-2026)
**Target:** Production-ready modular monolith — REST + gRPC + RabbitMQ events, PostgreSQL + Redis + Elasticsearch
**Date:** 2026-03-04

---

## TL;DR Recommendation

**Stack: Echo v4 (or Chi v5) + Connect RPC + Watermill + sqlc + Uber Fx**

- No framework forces an architectural style — clean architecture discipline matters more than framework choice
- Avoid go-zero and go-kratos for a modular monolith; they're built around microservice topology
- Connect RPC over plain gRPC for HTTP/1.1 browser + gRPC dual support out of box
- sqlc is the clear winner for PostgreSQL-heavy teams; bun is runner-up
- Uber Fx wins DI for anything beyond trivial scale; Wire is fine for small static graphs

---

## 1. Framework Comparison

### 1.1 go-kratos v2

| | |
|---|---|
| **GitHub** | 25.5k stars, v2.9.2 (Dec 2025), actively maintained |
| **Design** | Protocol Buffer first: HTTP + gRPC from one Protobuf definition |
| **Code gen** | `kratos` CLI generates service skeletons, Swagger docs auto-generated |
| **DI** | No built-in; works with Wire or Fx |
| **ORM** | None bundled — your choice |

**Pros:**
- Best-in-class HTTP+gRPC hybrid from single codebase (one handler serves both)
- Strong observability defaults (OpenTelemetry, Prometheus, recovery middleware)
- Bilibili production-tested; clean layered architecture
- No ORM opinion = team retains full choice

**Cons:**
- Primarily a microservices framework — modular monolith is a second-class citizen
- Smaller Western community (Chinese-origin project; docs and issues partly in Chinese)
- Medium learning curve: must understand Protobuf-centric design before productivity
- Ecosystem thinner than Spring Boot analogs; fewer third-party plugins
- Service registry / service discovery are core concepts that add cognitive overhead you don't need for a monolith

**Verdict for modular monolith:** Overkill. The service mesh primitives (registry, load balancing) are noise. Use it only if microservices split is planned within 6 months.

---

### 1.2 go-zero

| | |
|---|---|
| **GitHub** | 32.7k stars, v1.10.0 (Feb 2026), CNCF landscape |
| **Design** | `.api` file DSL → goctl generates entire service skeleton |
| **Code gen** | Extremely powerful: Go, iOS, Android, Kotlin, Dart, TS, JS |
| **Resilience** | Built-in circuit breaker, rate limit, adaptive load shedding |

**Pros:**
- goctl is genuinely the fastest scaffolding tool in the Go ecosystem
- AI-native tooling (MCP, Cursor integration) — future-proof DX direction
- Serious resilience engineering built-in for high-traffic scenarios
- Recent and active (Feb 2026 release)

**Cons:**
- **Most opinionated framework in this list** — goctl-generated code locks you in
- Generated code is "magical": debugging non-obvious issues is painful
- Services are either pure HTTP OR pure gRPC — no hybrid in single service instance (unlike Kratos)
- `.api` DSL is another language to learn on top of Go
- Heavy: not suited for rapid iteration or teams that deviate from the framework's opinion
- Primarily Chinese community; English documentation quality lags

**Verdict for modular monolith:** No. The DSL and code generation lock-in creates friction. Teams familiar with gin/echo will find it disorienting.

---

### 1.3 Fiber v3

| | |
|---|---|
| **GitHub** | ~35k stars, actively maintained |
| **Design** | Express.js-inspired, built on Fasthttp (NOT net/http) |
| **gRPC** | Client-side only via recipe; cannot natively serve gRPC |
| **Go version** | Requires Go 1.25+ |

**Pros:**
- Fastest HTTP throughput in benchmarks (Fasthttp engine)
- Lowest learning curve for anyone from Node.js/Express background
- Good middleware ecosystem
- Docker + Testcontainers integration blog from Docker (Dec 2024)

**Cons:**
- **Fasthttp is incompatible with net/http** — breaks most middleware, interceptors, and standard libraries
- Cannot serve gRPC natively — must run a separate gRPC server process
- Ecosystem isolation: tools expecting `http.Handler` don't work
- Community friction: `http.Handler` compatibility gap is a real maintenance burden
- v3 requires Go 1.25 — bleeding edge requirement

**Verdict for modular monolith:** Hard no for REST+gRPC requirement. Fasthttp incompatibility with net/http is a fundamental blocker for a unified service.

---

### 1.4 Chi v5 + Echo v4 (Custom Stack)

| | |
|---|---|
| **Chi** | ~18k stars, zero external deps, pure stdlib |
| **Echo** | ~30k stars, v4 stable (security patches until Dec 2026), v5 in progress |
| **gRPC** | Run standard `google.golang.org/grpc` alongside — separate port |

**Chi strengths:**
- Closest to stdlib; every `http.Handler` middleware works
- Zero external dependencies is a real operational advantage
- Composable sub-routers — natural fit for modular monolith domain boundaries
- Used in production by Cloudflare, Heroku, 99Designs

**Echo strengths:**
- More batteries included than Chi (binder, validator, OpenAPI)
- Familiar to teams coming from Gin (similar API surface)
- Context-first design

**Custom stack approach:**
```
HTTP router:    Chi v5 or Echo v4
gRPC:          google.golang.org/grpc (separate listener, same process)
Events:        Watermill + RabbitMQ
DB:            sqlc + pgx v5
Cache:         go-redis
Search:        olivere/elastic or elastic/go-elasticsearch
DI:            Uber Fx
Config:        viper or envconfig
Observability: OpenTelemetry
Hot reload:    air
```

**Cons:**
- No code generation — all boilerplate is manual
- No built-in service config, health check, graceful shutdown (must wire manually, though Fx helps)
- More initial setup time vs Kratos

**Verdict for modular monolith:** Best fit. Maximum control, zero lock-in, composable boundaries per domain module. The explicit wiring is also the documentation.

---

### 1.5 Connect RPC (buf.build)

| | |
|---|---|
| **GitHub** | 3.8k stars (connectrpc/connect-go), v1.19.1 (Oct 2025), stable |
| **Protocol support** | gRPC, gRPC-Web, Connect (HTTP/1.1 + HTTP/2) — all three simultaneously |
| **Ecosystem** | Buf CLI, Protovalidate, grpchealth, grpcreflect, connect-es (TypeScript) |

**What it is:** A replacement for the standard `google.golang.org/grpc` library that works on top of `net/http`, enabling gRPC and REST-like JSON calls on the same port.

**Pros:**
- **Works on standard `net/http`** — plugs into Chi, Echo, Mux natively with no conflict
- Single port serves gRPC, gRPC-Web (browser), and cURL-friendly JSON simultaneously
- No HTTP/2 requirement for clients — HTTP/1.1 works
- Protovalidate integration for request validation
- `buf` CLI replaces `protoc` with better ergonomics, linting, breaking change detection
- Stable 1.x, semantic versioning, no planned breaking changes
- OpenTelemetry interceptors available

**Cons:**
- 3.8k stars = smaller community than plain gRPC
- Teams must learn Protobuf + buf workflow (same overhead as plain gRPC)
- Not a web framework — must combine with Chi/Echo for REST routes that aren't RPC

**Verdict:** Strong recommendation to use Connect RPC INSTEAD of plain gRPC for the gRPC layer. The net/http compatibility is a game-changer for monolith architectures. Pair with Chi/Echo for non-RPC REST endpoints.

---

### 1.6 go-kit

| | |
|---|---|
| **GitHub** | 27.6k stars, v0.13.0 (Aug 2023) — last release 2+ years ago |
| **Design** | Toolkit, not framework; endpoint/transport/service layers |

**Pros:**
- Extremely principled layering (transport → endpoint → service)
- Unopinionated; integrates with anything
- Battle-tested in enterprise Go services

**Cons:**
- **No release since August 2023** — effectively in maintenance mode
- Verbose boilerplate per endpoint (request/response structs, encoders, decoders)
- The abstraction overhead is high for teams that just want to ship
- Modern alternatives (Connect RPC + Chi) achieve the same result with less ceremony
- Not suited for rapid iteration

**Verdict:** No. go-kit had its moment (2016-2021). Maintenance mode + high boilerplate + better alternatives make it a legacy choice in 2026.

---

## 2. ORM / Query Builder

| Tool | Stars | Type safety | Migrations | Performance | Best for |
|------|-------|------------|------------|-------------|---------|
| **sqlc** | ~14k | Compile-time | External (goose/atlas) | Raw SQL | PostgreSQL-first teams who know SQL |
| **bun** | ~4k | Medium-high | bun/migrate bundled | ~raw SQL | SQL-shaped queries with moderate ORM convenience |
| **GORM** | ~37k | Runtime (reflection) | Auto-migrate (caution) | Moderate | Prototyping, rapid CRUD |
| **ent** | ~15k | Compile-time | Generated | Moderate | Complex graph-like schemas |
| **sqlx** | ~16k | Medium | External | Near-raw | Drop-in upgrade from database/sql |

### sqlc (recommended)
- Write SQL, get type-safe Go — the philosophy is correct for PostgreSQL work
- `pgx/v5` driver gives best PostgreSQL performance
- No runtime magic; queries visible and reviewable as SQL
- Migrations via `goose` or `atlas` (separate concern, cleanly separated)
- **Limitation:** PostgreSQL only (MySQL experimental); schema changes require regenerating

### bun (strong alternative)
- Thin over `database/sql`, SQL-first query builder feel
- Bundles migrations (`bun/migrate`)
- Batch operations and large result sets handled well
- OpenTelemetry instrumentation built-in

### GORM — avoid for production PostgreSQL
- Reflection overhead adds up at scale
- Auto-migrate dangerous in production
- N+1 query issues common
- Acceptable for rapid prototyping or internal tooling

### ent — use only if graph traversal is core requirement
- Excellent type safety and complex relationship handling
- **Real drawbacks:** Long compile times, "magical" queries in debug mode, code bloat from generator
- Auto migration struggles with date/time edge cases
- Overkill for standard CRUD-heavy services

---

## 3. Dependency Injection

### Uber Fx (recommended for modular monolith)
- Lifecycle-aware: `OnStart`/`OnStop` hooks handle graceful shutdown cleanly
- Reflection-based runtime DI — dynamic module composition
- Module pattern maps cleanly to bounded contexts
- Used by Uber in production at massive scale
- Learning curve: 1-2 days to internalize `fx.Provide`, `fx.Invoke`, `fx.Module`
- **Best for:** Services with complex initialization order, graceful shutdown requirements

### Google Wire
- Compile-time code generation — zero runtime overhead
- Simple static dependency graphs: clearly the right choice
- Generates readable Go code (you can read what it wires)
- **Best for:** Simpler services, CLI tools, when you want to see the wiring

### Manual DI
- Best for small services (<5 components)
- Zero overhead, maximum clarity
- Becomes unmaintainable past ~15 components

**Recommendation:** Fx for modular monolith with multiple domain modules. The lifecycle hooks alone justify it.

---

## 4. Event-Driven: Watermill

| | |
|---|---|
| **GitHub** | 9.6k stars, v1.5.1 (Sep 2025), MIT |
| **RabbitMQ support** | `watermill-amqp` (AMQP pub/sub) |
| **Other backends** | Kafka, Redis Streams, NATS, PostgreSQL, Google PubSub, SQS/SNS |

**Why Watermill:**
- Unified abstraction across 12 pub/sub backends — swap RabbitMQ → Kafka with one line
- Production-hardened: stress-tested 20x parallel with race detector
- CQRS/Event Sourcing patterns built-in
- Clean publisher/subscriber interface; works well with Clean Architecture
- `Router` concept handles middleware, retry, poison queue, logging uniformly

**Alternatives considered:**
- Direct AMQP (`streadway/amqp` or `rabbitmq/amqp091-go`): lower abstraction, more control, higher boilerplate. Use if you need custom AMQP topology control.
- `ThreeDotsLabs/wild-workdays` patterns (CQRS): works on top of Watermill

**Verdict:** Still the best Go event-driven abstraction library. No serious competitor at same abstraction level. v1.5.1 (Sep 2025) shows active maintenance.

---

## 5. Project Structure for Modular Monolith

The golang-standards/project-layout is NOT an official Go standard — it's community convention. For a modular monolith, prefer:

```
/cmd/
    server/main.go          # entrypoint only
/internal/
    /module-name/           # one dir per bounded context
        handler.go          # HTTP handlers (Connect RPC or REST)
        service.go          # domain logic
        repository.go       # DB queries (sqlc generated)
        events.go           # Watermill publishers/subscribers
    /shared/
        middleware/
        config/
        database/
        observability/
/db/
    /queries/               # sqlc .sql files
    /migrations/            # goose migration files
/proto/
    /module-name/           # .proto files per module
/gen/
    /sqlc/                  # generated by sqlc
    /proto/                 # generated by buf
```

**Key principles:**
- Bounded contexts = Go packages under `/internal/`
- No cross-module DB joins — access other module data through its service interface
- Each module owns its SQL files and migrations conceptually
- `internal/` enforces compile-time encapsulation — external packages cannot import

**Three Dots Labs** (creators of Watermill) have the canonical "microservices or monolith is just a detail" post: the architecture discipline (Clean Architecture layers) matters more than whether you're in one process or many. This is the right mental model.

---

## 6. Key Tooling Recommendations

| Concern | Tool | Notes |
|---------|------|-------|
| Hot reload | `air` | Standard, works with any net/http app |
| Proto codegen | `buf` CLI | Replaces protoc; linting, breaking change detection |
| DB migrations | `goose` | Simple, widely used; or `atlas` for schema diffing |
| Linting | `golangci-lint` | 70%+ adoption per JetBrains survey |
| Testing | `testify` + `testcontainers-go` | Real DB/Redis/RabbitMQ in tests |
| Observability | OpenTelemetry | All major frameworks have interceptors |
| Config | `viper` or `envconfig` | envconfig simpler for 12-factor apps |

---

## 7. Community/Ecosystem Signal (2025)

From JetBrains Go Developer Survey 2025:
- Gin: 48% adoption (dominant but not growing)
- Echo: 16%
- Fiber: 11% (growing from 0 in 2020)
- `net/http` stdlib: most common routing foundation
- GORM + ent for heavy ORM; pgx for PostgreSQL-native; sqlx for multi-DB

---

## 8. Final Stack Recommendation

**For a team familiar with gin/echo/kratos building a modular monolith:**

```
HTTP layer:       Echo v4 (familiar API, solid middleware)
gRPC layer:       Connect RPC (connectrpc/connect-go) via buf
Event layer:      Watermill + watermill-amqp
DB:               sqlc + pgx/v5 + goose (migrations)
Cache:            go-redis v9
Search:           elastic/go-elasticsearch (official)
DI/Lifecycle:     Uber Fx
Config:           envconfig (simple) or viper (complex configs)
Observability:    OpenTelemetry SDK
Hot reload:       air
Linting:          golangci-lint
Proto tooling:    buf CLI
Testing:          testify + testcontainers-go
```

**Why not Kratos:** Microservices primitives add noise. Protobuf-first forces all endpoints through proto, limiting REST flexibility.

**Why not go-zero:** DSL lock-in and pure-HTTP vs pure-gRPC split are fundamental blockers.

**Why not Fiber:** Fasthttp incompatibility with net/http is disqualifying for REST+gRPC on same process.

**Why not go-kit:** Maintenance mode since 2023; high boilerplate for same outcome.

---

## Unresolved Questions

1. **Elasticsearch client choice:** `olivere/elastic` (community) vs `elastic/go-elasticsearch` (official) — need to evaluate query DSL ergonomics for the team's search patterns.
2. **Proto-first vs REST-first:** If most endpoints are REST with selective gRPC (internal), does the Connect RPC overhead (learning Protobuf, buf workflow) justify it vs a simple `net/http` gRPC server alongside Echo?
3. **sqlc vs bun for complex dynamic queries:** sqlc struggles with dynamic WHERE clauses — if the codebase needs dynamic filtering (e.g. Elasticsearch-style search), bun may complement sqlc rather than replace it.
4. **Redis usage pattern:** Is Redis used for caching only, or also pub/sub, queues, distributed locks? This affects whether Watermill's Redis Streams backend overlaps with direct go-redis usage.
5. **Migration tooling:** `goose` vs `atlas` — atlas provides schema diffing from actual DB state but adds operational complexity. Worth evaluating for team size.

---

## Sources

- [go-kratos/kratos GitHub](https://github.com/go-kratos/kratos)
- [zeromicro/go-zero GitHub](https://github.com/zeromicro/go-zero)
- [connectrpc/connect-go GitHub](https://github.com/connectrpc/connect-go)
- [go-kit/kit GitHub](https://github.com/go-kit/kit)
- [ThreeDotsLabs/watermill GitHub](https://github.com/ThreeDotsLabs/watermill)
- [Why Connect RPC is a great choice - wolfe.id.au (Dec 2025)](https://www.wolfe.id.au/2025/12/02/why-connect-rpc-is-a-great-choice-for-building-apis/)
- [Hardcore Comparison: Kratos vs go-zero vs GoFrame - Medium (Jul 2025)](https://medium.com/@g.zhufuyi/hardcore-multi-dimensional-comparison-kratos-vs-go-zero-vs-goframe-vs-sponge-which-go-8c168d75fe36)
- [Comparing Go ORMs: GORM vs Ent vs Bun vs sqlc - glukhov.org (Sep 2025)](https://www.glukhov.org/post/2025/09/comparing-go-orms-gorm-ent-bun-sqlc/)
- [Comparing best Go ORMs - Encore Cloud (2026)](https://encore.cloud/resources/go-orms)
- [Stop using entgo - DEV Community](https://dev.to/shandoncodes/stop-using-entgoplease-5gm5)
- [Go DI: Wire vs Fx vs Pure DI - Medium](https://medium.com/@geisonfgfg/dependency-injection-in-go-fx-vs-wire-vs-pure-di-structuring-maintainable-testable-applications-61c13939fd66)
- [When using Microservices or Monolith can be just a detail - Three Dots Labs](https://threedots.tech/post/microservices-or-monolith-its-detail/)
- [Go Ecosystem in 2025 - JetBrains GoLand Blog](https://blog.jetbrains.com/go/2025/11/10/go-language-trends-ecosystem-2025/)
- [golang-standards/project-layout GitHub](https://github.com/golang-standards/project-layout)
- [Fiber v3 What's New](https://docs.gofiber.io/next/whats_new/)
- [Connect RPC Getting Started](https://connectrpc.com/docs/go/getting-started/)
