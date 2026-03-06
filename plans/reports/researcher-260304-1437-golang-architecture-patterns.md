# Go Architecture Patterns & Best Practices for Production Modular Monolith

**Stack**: Echo + Connect RPC + Watermill + sqlc + Uber Fx + PostgreSQL + Redis + RabbitMQ + Elasticsearch
**Date**: 2026-03-04
**Focus**: Practical, Go-idiomatic, YAGNI-compliant patterns

---

## 1. Architecture Patterns

### Clean vs Hexagonal vs Onion in Go

**Verdict: Use a simplified Hexagonal (Ports & Adapters) approach.**

All three patterns share the same core principle: dependencies point inward, business logic has zero knowledge of infrastructure. In Go, the distinction between them is mostly academic. Go's implicit interfaces naturally create the "ports" layer without extra ceremony.

**Practical Go layout:**
```
internal/
  modules/
    order/
      domain/         # Entities, value objects, domain errors, repository interfaces (ports)
      app/            # Command/query handlers (application services)
      adapters/
        postgres/     # sqlc-generated code + repository implementations
        redis/        # Cache adapter
        http/         # Echo handlers (inbound port)
        grpc/         # Connect RPC handlers (inbound port)
      module.go       # Uber Fx module definition
    user/
      ...
  shared/
    events/           # Shared event definitions
    middleware/       # Cross-cutting middleware
```

**Key insight from [Three Dots Labs](https://threedots.tech/post/ddd-cqrs-clean-architecture-combined/):** Top-level organizing principle = bounded contexts (modules), NOT technical layers. Each module owns its full vertical slice.

**Go-specific advantage:** Go's package system enforces boundaries at compile time. Unexported types in `domain/` cannot be accessed from outside. No need for framework-level enforcement.

### Three Dots Labs Wild Workouts Structure

[Wild Workouts](https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example) demonstrates DDD + CQRS + Clean Architecture combined:
- **Domain layer**: Entities with encapsulated fields, constructors validate invariants
- **App layer**: Thin command/query handlers that orchestrate domain operations
- **Ports layer**: HTTP/gRPC handlers delegate to app layer
- **Adapters layer**: DB implementations behind repository interfaces

**Critical takeaway:** Handlers stay thin. Complex logic lives in domain entities. If your handler has >20 lines of business logic, extract it to domain.

### CQRS with Watermill

[Watermill CQRS component](https://watermill.io/docs/cqrs/) provides:
- **CommandBus** — publishes commands to topics, exactly one handler per command
- **EventBus** — publishes events, multiple handlers can subscribe
- **EventGroupProcessor** — multiple handlers sharing one subscriber for ordered processing
- Marshaler converts Go structs <-> Watermill messages

**When to use CQRS:**
- YES: Different read/write models (e.g., denormalized Elasticsearch read model, normalized Postgres write model)
- YES: Event-driven side effects (send email after order created)
- NO: Simple CRUD with identical read/write shapes — adds unnecessary complexity

**Implementation pattern:**
```go
// Command handler
type CreateOrderHandler struct {
    repo order.Repository
    bus  *cqrs.EventBus
}

func (h *CreateOrderHandler) Handle(ctx context.Context, cmd *CreateOrder) error {
    order, err := domain.NewOrder(cmd.UserID, cmd.Items)
    if err != nil { return err }
    if err := h.repo.Create(ctx, order); err != nil { return err }
    return h.bus.Publish(ctx, &OrderCreated{OrderID: order.ID})
}
```

### Event Sourcing

**Skip it for typical API projects.** Event sourcing adds significant complexity (event store, projections, eventual consistency, snapshots). Use it only when:
- Audit trail is a first-class business requirement
- You need temporal queries ("what was the state at time X?")
- Domain experts think in events naturally

**YAGNI approach:** Start with CQRS without event sourcing. Publish events from command handlers after persisting state in Postgres. Add event sourcing later IF needed for specific aggregates.

### Package Boundaries

Go enforces boundaries through:
1. **Unexported identifiers** — lowercase = package-private
2. **Internal packages** — `internal/` prevents external imports
3. **Interface segregation** — define interfaces where they're consumed, not where implemented
4. **Module boundaries** — each module is a self-contained package tree

**Rule:** Module A never imports module B's `adapters/` or `domain/` directly. Cross-module communication happens through events (Watermill) or shared interfaces.

---

## 2. Design Patterns That Matter

### Repository Pattern

**Idiomatic in Go when done right.** [Three Dots Labs argues convincingly](https://threedots.tech/post/repository-pattern-in-go/) for repositories in Go. The key is:

- One repository per **aggregate**, NOT per table
- Define interface in domain package (consumer side)
- Keep domain types separate from DB types

```go
// domain/repository.go — port definition
type OrderRepository interface {
    GetByID(ctx context.Context, id OrderID) (*Order, error)
    Save(ctx context.Context, order *Order) error
    Update(ctx context.Context, id OrderID, fn func(*Order) error) error
}
```

**Anti-patterns to avoid:**
- Don't create generic `Repository[T]` — different aggregates have different access patterns
- Don't pass `*sql.Tx` through repository interface — encapsulate transactions inside
- Don't share tables across repositories

### Unit of Work with sqlc

**Use the closure-based approach instead of classic Unit of Work.** [Database transactions in Go](https://threedots.tech/post/database-transactions-in-go/) recommends:

```go
func (r *PgOrderRepo) Update(ctx context.Context, id OrderID, fn func(*Order) error) error {
    tx, _ := r.pool.Begin(ctx)
    defer tx.Rollback(ctx)

    queries := db.New(tx) // sqlc queries bound to tx
    row, _ := queries.GetOrderForUpdate(ctx, id) // SELECT ... FOR UPDATE
    order := toDomain(row)

    if err := fn(order); err != nil { return err }

    _ = queries.UpdateOrder(ctx, toParams(order))
    return tx.Commit(ctx)
}
```

For cross-aggregate transactions (rare — question your design first), use a transaction provider:
```go
type TxProvider interface {
    Transact(ctx context.Context, fn func(adapters Adapters) error) error
}
```

### Middleware Chain (Echo + Connect)

**Echo middleware:** Standard `func(next echo.HandlerFunc) echo.HandlerFunc` — chain with `e.Use()`.

**Connect interceptors:** Similar concept but protocol-aware. [Connect interceptors](https://connectrpc.com/docs/go/interceptors/) wrap `UnaryFunc`:
```go
func LoggingInterceptor() connect.UnaryInterceptorFunc {
    return func(next connect.UnaryFunc) connect.UnaryFunc {
        return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
            start := time.Now()
            resp, err := next(ctx, req)
            slog.Info("rpc", "procedure", req.Spec().Procedure, "dur", time.Since(start))
            return resp, err
        }
    }
}
```

**Pattern:** Use Echo middleware for HTTP-level concerns (CORS, rate limiting, request ID). Use Connect interceptors for RPC-level concerns (auth, logging, tracing).

### Strategy Pattern for Swappable Implementations

Go interfaces ARE the strategy pattern. No need for named "strategies":
```go
type NotificationSender interface {
    Send(ctx context.Context, to UserID, msg Message) error
}
// Implementations: EmailSender, SMSSender, PushSender
// Swap via Uber Fx provider based on config
```

### Factory Pattern with Uber Fx

[Uber Fx](https://uber-go.github.io/fx/index.html) replaces manual factories with DI providers:
```go
var Module = fx.Module("order",
    fx.Provide(NewOrderRepository),    // constructor injection
    fx.Provide(NewCreateOrderHandler),
    fx.Provide(
        fx.Annotate(
            NewOrderRepository,
            fx.As(new(domain.OrderRepository)), // bind interface
        ),
    ),
)
```

**Fx best practices:**
- Group related providers into `fx.Module` per domain module
- Use `fx.Annotate` with `fx.As` to bind implementations to interfaces
- Use `fx.Invoke` sparingly — only for side-effect setup (starting HTTP server, registering routes)
- Leverage `fx.Lifecycle` for startup/shutdown hooks

### Functional Options Pattern

Use for configurable constructors with many optional parameters:
```go
type ServerOption func(*serverConfig)

func WithTimeout(d time.Duration) ServerOption {
    return func(c *serverConfig) { c.timeout = d }
}

func NewServer(addr string, opts ...ServerOption) *Server {
    cfg := defaultConfig()
    for _, opt := range opts { opt(&cfg) }
    // ...
}
```

**When to use:** Library-style APIs with 3+ optional configs. **Skip for:** Internal services where a simple config struct suffices.

### Error Type Patterns

```go
// Domain error with code
type DomainError struct {
    Code    ErrorCode
    Message string
    Err     error // wrapped original
}

func (e *DomainError) Error() string { return e.Message }
func (e *DomainError) Unwrap() error { return e.Err }

// Sentinel errors for common cases
var (
    ErrNotFound      = &DomainError{Code: CodeNotFound, Message: "not found"}
    ErrAlreadyExists = &DomainError{Code: CodeAlreadyExists, Message: "already exists"}
    ErrForbidden     = &DomainError{Code: CodeForbidden, Message: "forbidden"}
)

// Check with errors.Is / errors.As
if errors.Is(err, ErrNotFound) { ... }
```

---

## 3. Error Handling Best Practices

### Structured Errors with Codes

**Yes, use gRPC-style error codes even for REST.** [Connect RPC](https://connectrpc.com/docs/go/errors/) uses 16 standard codes that map cleanly to HTTP status codes. Adopt same codes for REST:

| Code | HTTP | Use |
|------|------|-----|
| `InvalidArgument` | 400 | Validation failures |
| `NotFound` | 404 | Resource missing |
| `AlreadyExists` | 409 | Duplicate creation |
| `PermissionDenied` | 403 | Authorization failure |
| `Unauthenticated` | 401 | Missing/invalid auth |
| `FailedPrecondition` | 412 | Business rule violation |
| `Internal` | 500 | Unexpected server error |
| `Unavailable` | 503 | Downstream dependency down |

### Error Wrapping

```go
// Wrap at boundaries with context
return fmt.Errorf("creating order for user %s: %w", userID, err)

// Don't wrap when it would leak implementation details
// BAD: return fmt.Errorf("postgres insert failed: %w", err) — from domain layer
// GOOD: return fmt.Errorf("saving order: %w", ErrAlreadyExists) — translated to domain error
```

### Domain vs Infrastructure Errors

- **Domain errors** — business rule violations, defined in `domain/` package
- **Infrastructure errors** — DB timeouts, network failures, connection refused
- **Translation layer** — adapters catch infra errors and translate to domain errors when possible (e.g., unique constraint violation -> `ErrAlreadyExists`)

### Connect RPC Error Handling

```go
// Creating errors with details
err := connect.NewError(connect.CodeInvalidArgument, errors.New("invalid email"))
detail, _ := connect.NewErrorDetail(&errdetails.BadRequest{
    FieldViolations: []*errdetails.BadRequest_FieldViolation{
        {Field: "email", Description: "must be valid email address"},
    },
})
err.AddDetail(detail)
```

### Centralized Error Middleware

```go
// Echo error handler that translates domain errors to HTTP responses
func ErrorHandler(err error, c echo.Context) {
    var domErr *domain.DomainError
    if errors.As(err, &domErr) {
        c.JSON(domErr.Code.HTTPStatus(), ErrorResponse{
            Code:    string(domErr.Code),
            Message: domErr.Message,
        })
        return
    }
    // Log unexpected errors, return generic 500
    slog.Error("unhandled error", "err", err, "path", c.Path())
    c.JSON(500, ErrorResponse{Code: "INTERNAL", Message: "internal server error"})
}
```

---

## 4. API Design Best Practices

### Versioning

**Use URL prefix versioning (`/v1/`, `/v2/`).** It's the most practical:
- Explicit and visible in logs/monitoring
- Easy to route at load balancer level
- No content negotiation complexity
- Header-based versioning adds friction for API consumers

**When to introduce v2:** Only when you have backward-incompatible changes. Don't preemptively version.

### Pagination

**Use cursor-based for production APIs.** Offset pagination breaks with concurrent writes and degrades at scale.

```go
type PageRequest struct {
    Cursor string `query:"cursor"`
    Limit  int    `query:"limit" validate:"min=1,max=100"`
}

type PageResponse[T any] struct {
    Items      []T    `json:"items"`
    NextCursor string `json:"next_cursor,omitempty"`
    HasMore    bool   `json:"has_more"`
}
```

**Cursor implementation:** Base64-encode `(sort_column_value, id)` tuple. Decode on server, use `WHERE (created_at, id) < ($1, $2) ORDER BY created_at DESC, id DESC LIMIT $3`.

**When offset is OK:** Admin dashboards, internal tools, small datasets (<10K rows).

### Filtering/Sorting/Searching

Keep it simple:
```
GET /v1/orders?status=pending&sort=-created_at&q=search+term
```
- Filtering: query params matching field names
- Sorting: field name, prefix `-` for descending
- Searching: `q` param for full-text (delegate to Elasticsearch)
- Validate allowed filter/sort fields server-side to prevent SQL injection

### Request Validation

**Validate in handler layer, before calling service.** Use a validation library (e.g., `go-playground/validator`):
```go
type CreateOrderRequest struct {
    Items []OrderItem `json:"items" validate:"required,min=1,dive"`
}
// Validate in handler, return 400 if invalid
// Domain layer validates business rules, returns FailedPrecondition
```

**Three validation layers:**
1. **Transport** — request shape, types, required fields (handler)
2. **Business** — domain invariants, business rules (domain entity constructors)
3. **Infrastructure** — DB constraints (last line of defense)

### Response Format

**Use flat responses, not envelopes.** Envelopes (`{data: ..., status: "ok"}`) add noise. HTTP status codes already convey success/failure.

```json
// Success: 200 with body
{"id": "123", "status": "pending", "items": [...]}

// Error: 4xx/5xx with error body
{"code": "INVALID_ARGUMENT", "message": "email is required"}

// List: 200 with pagination
{"items": [...], "next_cursor": "abc", "has_more": true}
```

### Idempotency Keys

Implement for all non-idempotent write operations (POST, PATCH):
```go
// Middleware approach
func IdempotencyMiddleware(store IdempotencyStore) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            key := c.Request().Header.Get("Idempotency-Key")
            if key == "" { return next(c) }

            if cached, ok := store.Get(ctx, key); ok {
                c.Response().Header().Set("X-Idempotent-Replay", "true")
                return c.JSONBlob(cached.Status, cached.Body)
            }
            // Execute, store response with TTL (24h), return
        }
    }
}
```
Store in Redis with TTL. Validate request body hash matches original to prevent key reuse with different payloads.

---

## 5. Security Patterns

### JWT + Refresh Token Flow

```
1. Login -> POST /v1/auth/login {email, password}
   Response: {access_token (15min), refresh_token (HTTP-only cookie, 7d)}

2. API calls -> Authorization: Bearer <access_token>

3. Refresh -> POST /v1/auth/refresh (cookie auto-sent)
   Response: {access_token (new)}
   Rotate refresh token on each use (detect reuse = revoke all)

4. Logout -> POST /v1/auth/logout
   Blacklist access_token jti in Redis (TTL = remaining token life)
```

**Key decisions:**
- Access token: short-lived (15min), stateless JWT, contains user_id + roles
- Refresh token: stored in Redis with `jti`, HTTP-only Secure SameSite=Strict cookie
- Use `github.com/golang-jwt/jwt/v5` for JWT operations
- Store refresh token family for rotation detection

### RBAC Implementation

```go
// Middleware-based
type Permission string
const (
    PermOrderCreate Permission = "order:create"
    PermOrderRead   Permission = "order:read"
    PermUserManage  Permission = "user:manage"
)

func RequirePermission(perms ...Permission) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            user := auth.UserFromContext(c.Request().Context())
            for _, p := range perms {
                if !user.HasPermission(p) {
                    return connect.NewError(connect.CodePermissionDenied, ...)
                }
            }
            return next(c)
        }
    }
}
```

**Store roles/permissions in DB.** Cache in JWT claims for hot-path checks. Refresh from DB on token refresh.

### Rate Limiting

**Use Redis-backed sliding window for production:**
- Per-user: `rate:user:{user_id}:{endpoint}` — 100 req/min
- Per-IP (unauthenticated): `rate:ip:{ip}:{endpoint}` — 20 req/min
- Global: circuit breaker for downstream protection

Libraries: `github.com/go-chi/httprate` or `github.com/ulule/limiter` (both work with Echo via adapter).

**Echo's built-in rate limiter caveat:** Not suitable for >16K unique identifiers due to Go map performance. Use Redis-backed solution for production.

### Input Sanitization

- Use parameterized queries (sqlc handles this automatically)
- Validate and bound all string lengths
- Strip HTML from user-generated content if rendering in web
- Use `bluemonday` for HTML sanitization if needed

### CORS

```go
e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example.com"},
    AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
    AllowHeaders:     []string{"Authorization", "Content-Type", "Connect-Protocol-Version"},
    AllowCredentials: true,
    MaxAge:           86400,
}))
```

**Never use `AllowOrigins: ["*"]` with `AllowCredentials: true`.** Browsers reject this. Be explicit about allowed origins.

---

## 6. Testing Strategy

### Table-Driven Tests

Standard Go idiom. Use for any function with multiple input/output combinations:
```go
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid", "user@example.com", false},
        {"no @", "userexample.com", true},
        {"empty", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateEmail(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
            }
        })
    }
}
```

### Integration Testing with Testcontainers

```go
func TestOrderRepository(t *testing.T) {
    ctx := context.Background()
    pgContainer, _ := postgres.Run(ctx, "postgres:16",
        postgres.WithDatabase("test"),
        testcontainers.WithWaitStrategy(wait.ForLog("ready to accept connections")),
    )
    t.Cleanup(func() { pgContainer.Terminate(ctx) })

    connStr, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
    pool, _ := pgxpool.New(ctx, connStr)
    // Run migrations, create repo, test...
}
```

**Tip:** Use `TestMain(m *testing.M)` to start containers once per package, not per test.

### Testing Watermill Event Handlers

Use Watermill's `GoChannel` Pub/Sub for in-memory testing:
```go
pubSub := gochannel.NewGoChannel(gochannel.Config{}, nil)
// Publish test event, assert handler produced expected side effects
```

### Testing Connect Services

```go
func TestCreateOrder(t *testing.T) {
    svc := NewOrderService(mockRepo)
    _, handler := orderv1connect.NewOrderServiceHandler(svc)
    server := httptest.NewServer(handler)
    defer server.Close()

    client := orderv1connect.NewOrderServiceClient(http.DefaultClient, server.URL)
    resp, err := client.CreateOrder(ctx, connect.NewRequest(&orderv1.CreateOrderRequest{...}))
    // Assert response
}
```

### Mocking Strategy

**Use interfaces, not mocking frameworks.** Go interfaces are small enough to implement test doubles manually:
```go
type mockOrderRepo struct {
    orders map[string]*Order
}
func (m *mockOrderRepo) GetByID(ctx context.Context, id OrderID) (*Order, error) {
    o, ok := m.orders[string(id)]
    if !ok { return nil, ErrNotFound }
    return o, nil
}
```

**When to use mocking frameworks:** Only when interfaces have 5+ methods and you only care about 1-2 in a specific test. `github.com/stretchr/testify/mock` or `go.uber.org/mock` (gomock).

### Golden File Testing

For API response shape stability:
```go
// testdata/golden/create_order_response.json
golden.Assert(t, responseBody, "testdata/golden/create_order_response.json")
```
Use `go test -update` flag to regenerate golden files.

---

## 7. Performance Patterns

### Connection Pooling

**pgx pool:**
```go
config, _ := pgxpool.ParseConfig(connStr)
config.MaxConns = 25              // Match expected concurrency
config.MinConns = 5               // Keep warm connections
config.MaxConnLifetime = 1 * time.Hour
config.MaxConnIdleTime = 30 * time.Minute
config.HealthCheckPeriod = 1 * time.Minute
pool, _ := pgxpool.NewWithConfig(ctx, config)
```

**Redis pool (go-redis):**
```go
rdb := redis.NewClient(&redis.Options{
    PoolSize:     10 * runtime.NumCPU(), // Default: 10 per CPU
    MinIdleConns: 5,
    PoolTimeout:  30 * time.Second,
})
```

### Caching Strategies

**Start with Cache-Aside (Lazy Loading):**
```go
func (s *OrderService) GetByID(ctx context.Context, id string) (*Order, error) {
    // 1. Check cache
    if cached, err := s.cache.Get(ctx, "order:"+id); err == nil {
        return cached, nil
    }
    // 2. Load from DB
    order, err := s.repo.GetByID(ctx, id)
    if err != nil { return nil, err }
    // 3. Populate cache with TTL
    s.cache.Set(ctx, "order:"+id, order, 5*time.Minute)
    return order, nil
}
```

**Cache invalidation:** Invalidate on write. For Watermill: publish event -> cache invalidation handler subscribes and deletes keys.

**When to skip caching:** If your DB can handle the read load. Premature caching adds complexity. Measure first.

### N+1 Prevention with sqlc

sqlc encourages writing explicit SQL, which naturally prevents N+1:
```sql
-- name: GetOrdersWithItems :many
SELECT o.*, oi.product_id, oi.quantity, oi.price
FROM orders o
JOIN order_items oi ON o.id = oi.order_id
WHERE o.user_id = $1
ORDER BY o.created_at DESC;
```

For batch loading: use `ANY(@ids::uuid[])` pattern:
```sql
-- name: GetOrdersByIDs :many
SELECT * FROM orders WHERE id = ANY(@ids::uuid[]);
```

### Bulk Operations

Use PostgreSQL's array unnesting for batch inserts via sqlc:
```sql
-- name: BulkCreateItems :copyfrom
INSERT INTO items (name, price) VALUES ($1, $2);
```
sqlc generates `CopyFrom` using pgx's `CopyFrom` protocol — fastest for large batches.

### Context Timeout Propagation

```go
// Set at HTTP/gRPC entry point
ctx, cancel := context.WithTimeout(c.Request().Context(), 30*time.Second)
defer cancel()
// Pass through all layers — DB, cache, external calls all respect the deadline
```

### Graceful Degradation

```go
// Fallback when cache is down
order, err := s.cache.Get(ctx, key)
if err != nil {
    slog.Warn("cache unavailable, falling back to DB", "err", err)
    return s.repo.GetByID(ctx, id) // Degrade, don't fail
}
```

Use circuit breakers (`github.com/sony/gobreaker`) for external service calls.

---

## 8. Observability Patterns

### Structured Logging

**Recommendation: `log/slog` (stdlib).** Since Go 1.21, slog covers most needs without external deps.

| Library | Perf | Zero-alloc | Stdlib | Best For |
|---------|------|-----------|--------|----------|
| slog | Good | ~1 alloc/op | Yes | Most projects |
| zerolog | Best | Yes | No | High-throughput, JSON-heavy |
| zap | Great | Near-zero | No | Uber ecosystem integration |

```go
// slog with JSON handler for production
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
slog.SetDefault(logger)

// Contextual logging
slog.InfoContext(ctx, "order created",
    "order_id", order.ID,
    "user_id", user.ID,
    "total", order.Total,
)
```

**Use zerolog only if** benchmarks prove slog is a bottleneck (unlikely for most services).

### Distributed Tracing (OpenTelemetry)

**Context propagation across protocols:**

- **HTTP (Echo):** `otelecho` middleware auto-extracts/injects W3C `traceparent` header
- **gRPC (Connect):** `otelconnect` interceptor handles gRPC metadata
- **RabbitMQ (Watermill):** Manual propagation via message headers:

```go
// Publisher: inject trace context into message metadata
otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(msg.Metadata))

// Subscriber: extract trace context from message metadata
ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(msg.Metadata))
```

### Metrics

- **Technical:** request latency, error rates, DB pool utilization, cache hit ratio
- **Business:** orders created/hour, revenue processed, user signups
- Use Prometheus client (`github.com/prometheus/client_golang`) with OpenTelemetry bridge

### Health Checks

```go
// /healthz — liveness: is the process alive? KEEP SIMPLE.
e.GET("/healthz", func(c echo.Context) error {
    return c.JSON(200, map[string]string{"status": "ok"})
})

// /readyz — readiness: can it handle traffic?
e.GET("/readyz", func(c echo.Context) error {
    if err := db.Ping(ctx); err != nil { return c.JSON(503, ...) }
    if err := rdb.Ping(ctx).Err(); err != nil { return c.JSON(503, ...) }
    return c.JSON(200, map[string]string{"status": "ready"})
})
```

**Liveness:** Never check dependencies. Only detect deadlocked/crashed process.
**Readiness:** Check critical dependencies (DB, cache). Used by load balancers for traffic routing.

---

## 9. Configuration Management

### 12-Factor Config

```go
type Config struct {
    Port        int    `env:"PORT" envDefault:"8080"`
    DatabaseURL string `env:"DATABASE_URL,required"`
    RedisURL    string `env:"REDIS_URL,required"`
    RabbitURL   string `env:"RABBITMQ_URL,required"`
    JWTSecret   string `env:"JWT_SECRET,required"`
    LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
}
// Use github.com/caarlos0/env/v11
```

**Priority chain:** ENV vars > config file > defaults. Never hardcode secrets.

### Environment-Specific Configs

Use ENV vars, not separate config files per environment. The binary is identical across dev/staging/prod — only env vars change.

### Feature Flags

Start simple: ENV var booleans or DB-backed flags. Adopt [go-feature-flag](https://gofeatureflag.org/) when you need:
- Percentage rollouts
- User targeting
- A/B testing

**YAGNI:** Don't add feature flag infrastructure until you have >3 flags.

### Secret Management

- **Local dev:** `.env` file (gitignored), loaded by `godotenv`
- **Production:** Cloud provider secret manager (AWS SSM, GCP Secret Manager, Vault)
- **Never** commit secrets. Use `.env.example` with placeholder values.

---

## 10. Go-Specific Idioms

### Context Propagation

**Pass `context.Context` as first parameter to EVERY function that does I/O or may be cancelled.** No exceptions.

```go
// Good
func (s *Service) CreateOrder(ctx context.Context, req CreateOrderReq) (*Order, error)

// Bad — context in struct field
type Service struct { ctx context.Context } // DON'T
```

### Goroutine Lifecycle Management

```go
// errgroup for coordinated goroutines with error propagation
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(10) // bounded parallelism

for _, item := range items {
    item := item
    g.Go(func() error {
        return processItem(ctx, item)
    })
}
if err := g.Wait(); err != nil { ... }
```

**Rules:**
- Every goroutine must have a clear shutdown path
- Always use `context.Context` for cancellation
- Use `errgroup` over raw `go` + `sync.WaitGroup` when errors matter
- Recover panics in goroutines to prevent process crash

### Worker Pool Pattern

```go
func ProcessBatch(ctx context.Context, jobs <-chan Job, workers int) error {
    g, ctx := errgroup.WithContext(ctx)
    for i := 0; i < workers; i++ {
        g.Go(func() error {
            for {
                select {
                case job, ok := <-jobs:
                    if !ok { return nil }
                    if err := process(ctx, job); err != nil {
                        slog.Error("job failed", "err", err)
                    }
                case <-ctx.Done():
                    return ctx.Err()
                }
            }
        })
    }
    return g.Wait()
}
```

### Graceful Shutdown Orchestration

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
    defer stop()

    // Uber Fx handles this automatically with fx.Lifecycle:
    app := fx.New(
        fx.Provide(...),
        fx.Invoke(func(lc fx.Lifecycle, srv *echo.Echo) {
            lc.Append(fx.Hook{
                OnStart: func(ctx context.Context) error {
                    go srv.Start(":8080")
                    return nil
                },
                OnStop: func(ctx context.Context) error {
                    return srv.Shutdown(ctx) // 15s default timeout
                },
            })
        }),
    )
    app.Run() // Blocks until signal, then runs OnStop hooks in reverse order
}
```

**Fx handles shutdown ordering automatically** — OnStop hooks run in reverse registration order, matching the "reverse initialization" pattern.

### Go 1.22+ Features to Leverage

**Range over integers (1.22):**
```go
for i := range 10 { fmt.Println(i) } // 0..9
```

**Range over functions / iterators (1.23):**
```go
func All[T any](items []T) iter.Seq[T] {
    return func(yield func(T) bool) {
        for _, item := range items {
            if !yield(item) { return }
        }
    }
}
```

**Enhanced ServeMux routing (1.22):** Not relevant since we use Echo + Connect, but good to know for lightweight internal tooling.

**Loop variable fix (1.22):** Loop variables are per-iteration by default. No more `v := v` copies needed inside closures.

---

## Summary: What to Implement First (YAGNI Priority)

| Priority | Pattern | Rationale |
|----------|---------|-----------|
| P0 | Modular package structure | Foundation for everything else |
| P0 | Uber Fx DI modules | Wire everything together cleanly |
| P0 | Repository pattern (per aggregate) | Data access abstraction |
| P0 | Structured error types + middleware | Consistent error responses |
| P0 | JWT auth + RBAC middleware | Security baseline |
| P0 | Structured logging (slog) | Observability baseline |
| P0 | Graceful shutdown (via Fx lifecycle) | Production reliability |
| P1 | CQRS commands (Watermill) | Event-driven side effects |
| P1 | Request validation layer | Input safety |
| P1 | Cursor pagination | Scalable list endpoints |
| P1 | Health checks (/healthz, /readyz) | Kubernetes readiness |
| P1 | OpenTelemetry tracing | Distributed debugging |
| P1 | Redis caching (cache-aside) | Performance for hot paths |
| P2 | Idempotency keys | Safe retries for clients |
| P2 | Rate limiting (Redis-backed) | Abuse protection |
| P2 | Feature flags | Controlled rollouts |
| P2 | CQRS read models (Elasticsearch) | Optimized queries |
| P3 | Event sourcing | Only if audit trail is required |
| P3 | Worker pools | Background batch processing |
| P3 | Circuit breakers | External service resilience |

---

## Unresolved Questions

1. **Echo v5 vs v4:** Echo v5 is in development with breaking changes. Should the boilerplate target v4 (stable) or v5 (future-proof)? Need to check v5 release timeline.
2. **sqlc emit_interface:** Should we use sqlc's `emit_interface` option to generate interfaces for testability, or wrap sqlc-generated code in manual repository interfaces?
3. **Watermill + sqlc transaction coordination:** When a command handler needs both Watermill event publishing and sqlc DB writes in one transaction, should we use Watermill's outbox pattern or pgx transaction + manual publish-after-commit?
4. **Connect RPC + Echo coexistence:** Best pattern for mounting Connect handlers alongside Echo routes on same port vs separate ports?
5. **Elasticsearch sync strategy:** Use Watermill events to sync write model (Postgres) to read model (Elasticsearch)? Or use PostgreSQL logical replication / CDC (Debezium)?

---

## Key Sources

- [Three Dots Labs — DDD + CQRS + Clean Architecture Combined](https://threedots.tech/post/ddd-cqrs-clean-architecture-combined/)
- [Three Dots Labs — Repository Pattern in Go](https://threedots.tech/post/repository-pattern-in-go/)
- [Three Dots Labs — Database Transactions in Go](https://threedots.tech/post/database-transactions-in-go/)
- [Three Dots Labs — Common Anti-Patterns in Go Web Apps](https://threedots.tech/post/common-anti-patterns-in-go-web-applications/)
- [Wild Workouts DDD Example](https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example)
- [Watermill CQRS Docs](https://watermill.io/docs/cqrs/)
- [Connect RPC — Errors](https://connectrpc.com/docs/go/errors/)
- [Connect RPC — Interceptors](https://connectrpc.com/docs/go/interceptors/)
- [Uber Fx Docs](https://uber-go.github.io/fx/index.html)
- [sqlc + pgx Production Guide (Brandur)](https://brandur.org/sqlc)
- [VictoriaMetrics — Go Graceful Shutdown](https://victoriametrics.com/blog/go-graceful-shutdown/)
- [Testcontainers for Go](https://golang.testcontainers.org/)
- [Go Feature Flag](https://gofeatureflag.org/)
- [Better Stack — Go Logging Libraries](https://betterstack.com/community/guides/logging/best-golang-logging-libraries/)
- [OpenTelemetry Context Propagation](https://opentelemetry.io/docs/concepts/context-propagation/)
