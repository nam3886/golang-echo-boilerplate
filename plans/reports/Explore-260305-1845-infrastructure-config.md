# GNHA Services Infrastructure & Configuration Analysis

**Date:** March 5, 2026  
**Project:** gnha-services  
**Focus:** Connection pooling, timeouts, rate limiting, CORS, API versioning, observability, auth, and database patterns

---

## 1. CONNECTION POOL SETTINGS

### PostgreSQL Pool Configuration
**File:** `internal/shared/database/postgres.go` (lines 13-40)

```go
poolCfg.MaxConns = 25
poolCfg.MinConns = 5
poolCfg.MaxConnLifetime = 1 * time.Hour
poolCfg.MaxConnIdleTime = 30 * time.Minute
```

**Details:**
- Uses `github.com/jackc/pgx/v5/pgxpool`
- Max connections: **25**
- Min connections: **5**
- Lifetime: **1 hour** (connections recycled)
- Idle timeout: **30 minutes**
- Retry logic: **10 attempts** with exponential backoff (1s, 2s, 3s... intervals)

### Redis Pool Configuration
**File:** `internal/shared/database/redis.go` (lines 15-22)

```go
opt.PoolSize = 10 * runtime.NumCPU()
opt.MinIdleConns = 5
```

**Details:**
- Uses `github.com/redis/go-redis/v9`
- Pool size: **10 × CPU cores** (scales with system)
- Min idle connections: **5**
- Retry logic: **10 attempts** with exponential backoff

---

## 2. TIMEOUT SETTINGS

### Global Request Timeout
**File:** `internal/shared/middleware/chain.go` (lines 37-40)

```go
e.Use(echomw.ContextTimeoutWithConfig(echomw.ContextTimeoutConfig{
    Timeout: 30 * time.Second,
}))
```

**Details:**
- Global HTTP request timeout: **30 seconds**
- Applied to all routes via middleware chain

### JWT Configuration
**File:** `internal/shared/config/config.go` (lines 28-31)

```go
JWTAccessTTL  time.Duration `env:"JWT_ACCESS_TTL" envDefault:"15m"`
JWTRefreshTTL time.Duration `env:"JWT_REFRESH_TTL" envDefault:"168h"`
```

**Details:**
- Access token TTL: **15 minutes** (configurable via `JWT_ACCESS_TTL`)
- Refresh token TTL: **168 hours / 7 days** (configurable via `JWT_REFRESH_TTL`)

### Connection-Level Timeouts
- All database connections include retry logic with progressive delays
- Redis connections include retry logic with progressive delays

---

## 3. RATE LIMITING IMPLEMENTATION

**File:** `internal/shared/middleware/rate_limit.go`

```go
func RateLimit(rdb *redis.Client, limit int, window time.Duration) echo.MiddlewareFunc {
    // Redis sliding window rate limiter
}
```

**Algorithm:** Redis-based sliding window counter
- **Default limit:** 100 requests per minute
- **Window:** 1-minute sliding window
- **Key structure:**
  - Authenticated users: `ratelimit:user:{userID}`
  - Anonymous requests: `ratelimit:ip:{clientIP}`
- **Response headers:** Includes `Retry-After` header (in seconds)
- **Failure mode:** Fail-open (allows request if Redis unavailable)
- **Implementation:** Uses Redis sorted sets with timestamp-based scoring

**Code snippet:**
```go
func rateLimitKey(c echo.Context) string {
    if user := auth.UserFromContext(c.Request().Context()); user != nil {
        return "ratelimit:user:" + user.UserID
    }
    return "ratelimit:ip:" + c.RealIP()
}
```

---

## 4. CORS CONFIGURATION

**File:** `internal/shared/middleware/chain.go` (lines 27-36)

```go
e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
    AllowOrigins: cfg.CORSOrigins,
    AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
    AllowHeaders: []string{
        "Accept", "Authorization", "Content-Type",
        "X-Request-ID", "Connect-Protocol-Version",
    },
    AllowCredentials: true,
    MaxAge:          3600,
}))
```

**Config source:** `internal/shared/config/config.go` (line 45)
```go
CORSOrigins []string `env:"CORS_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000"`
```

**Details:**
- **Allowed origins:** Configured via environment variable `CORS_ORIGINS` (comma-separated)
- **Default origin:** `http://localhost:3000`
- **Allowed methods:** GET, POST, PUT, PATCH, DELETE, OPTIONS
- **Allowed headers:** Accept, Authorization, Content-Type, X-Request-ID, Connect-Protocol-Version
- **Credentials:** Enabled (`AllowCredentials: true`)
- **Cache duration:** 3600 seconds (1 hour)

---

## 5. API VERSIONING APPROACH

**File:** `proto/user/v1/user.proto`

**Strategy:** Package-based versioning using Protocol Buffers

```proto
syntax = "proto3";
package user.v1;
option go_package = "github.com/gnha/gnha-services/gen/proto/user/v1;userv1";
```

**Details:**
- **Versioning method:** Semantic versioning in proto package names (e.g., `user.v1`)
- **Code generation:** Generates to `gen/proto/user/v1/`
- **Service definition:** Services defined within versioned packages
- **Service example:**
  ```proto
  service UserService {
    rpc GetUser(GetUserRequest) returns (GetUserResponse);
    rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
    rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
    rpc UpdateUser(UpdateUserRequest) returns (UpdateUserResponse);
    rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse);
  }
  ```
- **Validation:** Uses buf.validate for message field validation with rules like:
  - Email validation: `[(buf.validate.field).string.email = true]`
  - Length constraints: `[(buf.validate.field).string = {min_len: 1, max_len: 255}]`
  - Enum validation: `[(buf.validate.field).string = {in: ["admin", "member", "viewer"]}]`
  - UUID validation: `[(buf.validate.field).string.uuid = true]`

---

## 6. EVENT/MESSAGING SYSTEM

### Watermill + RabbitMQ
**Files:** 
- `internal/shared/events/bus.go`
- `internal/shared/events/subscriber.go`
- `internal/shared/events/topics.go`
- `internal/shared/events/module.go`

**Event Bus Implementation:**
```go
type EventBus struct {
    publisher message.Publisher
}

func (b *EventBus) Publish(ctx context.Context, topic string, event any) error {
    payload, err := json.Marshal(event)
    msg := message.NewMessage(uuid.NewString(), payload)
    // Propagate trace context into message metadata
    otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(msg.Metadata))
    msg.Metadata.Set("event_type", topic)
    return b.publisher.Publish(topic, msg)
}
```

**Configuration:**
```go
amqpCfg := amqp.NewDurableQueueConfig(cfg.RabbitURL)
pub, err := amqp.NewPublisher(amqpCfg, watermill.NewSlogLogger(slog.Default()))
```

**Topics Defined:**
```go
const (
    TopicUserCreated = "user.created"
    TopicUserUpdated = "user.updated"
    TopicUserDeleted = "user.deleted"
)
```

**Event Types:**
- `UserCreatedEvent` (user_id, actor_id, email, name, role, timestamp)
- `UserUpdatedEvent` (user_id, actor_id, timestamp)
- `UserDeletedEvent` (user_id, actor_id, timestamp)

**Router Configuration:**
- **Middleware:** Recoverer, Retry (3 max retries, 1s initial interval)
- **Delivery:** Handler registration pattern with Fx dependency injection
- **Trace propagation:** Automatic OTel context injection into message metadata

**Dependency Management:**
```go
require (
    github.com/ThreeDotsLabs/watermill v1.5.1
    github.com/ThreeDotsLabs/watermill-amqp/v3 v3.0.2
)
```

---

## 7. OPENTELEMETRY / OBSERVABILITY SETUP

### Tracer Provider
**File:** `internal/shared/observability/tracer.go`

```go
func NewTracerProvider(cfg *config.Config) (*sdktrace.TracerProvider, error) {
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
        otlptracegrpc.WithInsecure(),
    )
    // ...
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName(cfg.AppName),
            semconv.ServiceVersion("0.1.0"),
            semconv.DeploymentEnvironment(cfg.AppEnv),
        )),
    )
}
```

**Details:**
- **Exporter:** OTLP gRPC (configurable endpoint)
- **Default endpoint:** `http://localhost:4317` (env: `OTEL_EXPORTER_OTLP_ENDPOINT`)
- **Propagation:** TraceContext + Baggage
- **Resource attributes:** Service name, version, deployment environment

### Meter Provider (Metrics)
**File:** `internal/shared/observability/metrics.go`

```go
func NewMeterProvider(cfg *config.Config) (*sdkmetric.MeterProvider, error) {
    exporter, err := otlpmetricgrpc.New(ctx,
        otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
        otlpmetricgrpc.WithInsecure(),
    )
    // ...
    mp := sdkmetric.NewMeterProvider(
        sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
        // ...
    )
}
```

**Details:**
- **Metric exporter:** OTLP gRPC
- **Reader:** Periodic reader (default 60s interval)
- **Resource attributes:** Service name, version

### Logger
**File:** `internal/shared/observability/logger.go`

```go
func NewLogger(cfg *config.Config) *slog.Logger {
    level := parseLevel(cfg.LogLevel)
    if cfg.IsDevelopment() {
        handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            Level:     level,
            AddSource: true,
        })
    } else {
        handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
            Level: level,
        })
    }
}
```

**Details:**
- **Logger:** Go 1.26 `slog` (structured logging)
- **Format:** Text (dev) / JSON (prod)
- **Level:** Configurable via `LOG_LEVEL` env (debug, info, warn, error)
- **Source tracking:** Enabled in development

**Dependency:**
```
go.opentelemetry.io/otel v1.41.0
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.41.0
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.41.0
go.opentelemetry.io/otel/sdk v1.41.0
```

---

## 8. GOLANGCI-LINT CONFIG

**File:** `.golangci.yml`

```yaml
run:
  timeout: 5m

linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gocritic
    - misspell
    - revive
    - unconvert
    - unparam

linters-settings:
  gocritic:
    enabled-tags: [diagnostic, style, performance]
  revive:
    rules:
      - name: unexported-return
        disabled: true

issues:
  exclude-dirs: [gen, tmp, vendor]
  max-issues-per-linter: 50
  max-same-issues: 5
```

**Details:**
- **Run timeout:** 5 minutes
- **Active linters:** errcheck, gosimple, govet, ineffassign, staticcheck, unused, gocritic, misspell, revive, unconvert, unparam
- **Excluded directories:** gen, tmp, vendor
- **Gocritic tags:** Diagnostic, style, performance
- **Issue limits:** 50 per linter, 5 duplicate issues max
- **Disabled rule:** unexported-return (from revive)

---

## 9. GO.MOD - KEY DEPENDENCIES

**File:** `go.mod` (Go 1.26.0)

### Core Dependencies:
- **Web Framework:** `github.com/labstack/echo/v4 v4.15.1`
- **Database:** `github.com/jackc/pgx/v5 v5.8.0`
- **Redis:** `github.com/redis/go-redis/v9 v9.18.0`
- **Messaging:** `github.com/ThreeDotsLabs/watermill v1.5.1`
- **AMQP:** `github.com/ThreeDotsLabs/watermill-amqp/v3 v3.0.2`
- **JWT:** `github.com/golang-jwt/jwt/v5 v5.3.1`
- **Dependency Injection:** `go.uber.org/fx v1.24.0`
- **Config Management:** `github.com/caarlos0/env/v11 v11.4.0`
- **Observability:** `go.opentelemetry.io/otel v1.41.0` (traces + metrics)
- **Protocol Buffers:** `google.golang.org/protobuf v1.36.11`
- **Proto Validation:** `connectrpc.com/validate v0.6.0`
- **UUID:** `github.com/google/uuid v1.6.0`
- **Crypto:** `golang.org/x/crypto v0.48.0`
- **Cron:** `github.com/robfig/cron/v3 v3.0.1`
- **Testing:** `github.com/testcontainers/testcontainers-go v0.40.0`

---

## 10. SECURITY HEADERS MIDDLEWARE

**File:** `internal/shared/middleware/security.go`

```go
func SecurityHeaders() echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            h := c.Response().Header()
            h.Set("X-Content-Type-Options", "nosniff")
            h.Set("X-Frame-Options", "DENY")
            h.Set("X-XSS-Protection", "1; mode=block")
            h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
            h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
            h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
            return next(c)
        }
    }
}
```

**Headers Set:**
1. `X-Content-Type-Options: nosniff` — Prevents MIME type sniffing
2. `X-Frame-Options: DENY` — Disables framing (clickjacking protection)
3. `X-XSS-Protection: 1; mode=block` — XSS filter enabled
4. `Referrer-Policy: strict-origin-when-cross-origin` — Limits referrer leakage
5. `Strict-Transport-Security: max-age=31536000; includeSubDomains` — HSTS (1 year)
6. `Permissions-Policy: camera=(), microphone=(), geolocation=()` — Disables sensitive features

---

## 11. BODY LIMIT MIDDLEWARE

**File:** `internal/shared/middleware/chain.go` (line 21)

```go
e.Use(echomw.BodyLimit("10M"))
```

**Details:**
- **Limit:** 10 MB
- **Applies to:** All request bodies
- **Purpose:** Prevents large payload DoS attacks

---

## 12. GZIP MIDDLEWARE

**File:** `internal/shared/middleware/chain.go` (lines 22-23)

```go
e.Use(echomw.GzipWithConfig(echomw.GzipConfig{Level: 5}))
```

**Details:**
- **Compression level:** 5 (balanced between ratio and CPU)
- **Algorithm:** gzip
- **Auto-negotiated:** Based on `Accept-Encoding: gzip` header

---

## 13. JWT/AUTH IMPLEMENTATION

### JWT Generation & Validation
**File:** `internal/shared/auth/jwt.go`

```go
type TokenClaims struct {
    jwt.RegisteredClaims
    UserID      string   `json:"uid"`
    Role        string   `json:"role"`
    Permissions []string `json:"perms,omitempty"`
}

func GenerateAccessToken(cfg *config.Config, userID, role string, permissions []string) (string, error) {
    claims := TokenClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.JWTAccessTTL)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            ID:        uuid.NewString(),
        },
        UserID:      userID,
        Role:        role,
        Permissions: permissions,
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(cfg.JWTSecret))
}
```

**Token Claims:**
- Standard claims: ExpiresAt, IssuedAt, ID (jti)
- Custom claims: UserID, Role, Permissions (array)

**Signing Method:** HS256 (HMAC-SHA256)

### Auth Middleware
**File:** `internal/shared/middleware/auth.go`

```go
func Auth(cfg *config.Config, rdb *redis.Client) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            token := extractBearerToken(c)
            claims, err := auth.ValidateAccessToken(cfg, token)
            
            // Check token blacklist (logout)
            if blacklisted, _ := rdb.Exists(ctx, "blacklist:"+claims.RegisteredClaims.ID).Result(); blacklisted > 0 {
                return domainerr.ErrUnauthorized
            }
            
            ctx = auth.WithUser(ctx, claims)
            return next(c)
        }
    }
}
```

**Features:**
- Bearer token extraction from `Authorization` header
- Signature validation with secret key
- Token blacklist support (logout via Redis key: `blacklist:{jti}`)
- User context injection

### Refresh Token
**File:** `internal/shared/auth/jwt.go` (lines 57-64)

```go
func GenerateRefreshToken() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("generating refresh token: %w", err)
    }
    return base64.URLEncoding.EncodeToString(b), nil
}
```

**Details:**
- **Length:** 32 bytes (256 bits)
- **Encoding:** Base64 URL-safe
- **Generation:** Cryptographically random

### RBAC Middleware
**File:** `internal/shared/middleware/rbac.go`

```go
const (
    PermUserRead   Permission = "user:read"
    PermUserWrite  Permission = "user:write"
    PermUserDelete Permission = "user:delete"
    PermAdminAll   Permission = "admin:*"
)

func RequirePermission(perms ...Permission) echo.MiddlewareFunc {
    // Checks user.HasPermission(string(p)) for all perms
}

func RequireRole(roles ...string) echo.MiddlewareFunc {
    // Checks user.Role == r for any role
}
```

**Implementation:**
- **Permission-based:** Named permission strings
- **Role-based:** Exact role matching

### Password Hashing
**File:** `internal/shared/auth/password.go` (referenced in `cmd/server/main.go`)
- Hash function injected via Fx: `fx.Provide(auth.NewPasswordHasher)`

---

## 14. DATABASE MIGRATION FILES & PATTERNS

### Migration File
**File:** `db/migrations/00001_initial_schema.sql`

**Tool:** Goose (indicated by `+goose` directives)

```sql
-- +goose Up
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      VARCHAR(255) NOT NULL UNIQUE,
    name       VARCHAR(255) NOT NULL,
    password   TEXT NOT NULL,
    role       VARCHAR(50) NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_users_email ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_active ON users (id) WHERE deleted_at IS NULL;

CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type VARCHAR(50) NOT NULL,
    entity_id   UUID NOT NULL,
    action      VARCHAR(20) NOT NULL,
    actor_id    UUID NOT NULL,
    changes     JSONB,
    ip_address  INET,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_entity ON audit_logs (entity_type, entity_id);
CREATE INDEX idx_audit_actor ON audit_logs (actor_id);
CREATE INDEX idx_audit_created ON audit_logs (created_at);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS users;
```

**Patterns:**
- **UUID generation:** Native PostgreSQL `gen_random_uuid()`
- **Soft deletes:** `deleted_at` column (nullable)
- **Timestamps:** `TIMESTAMPTZ` with default `NOW()`
- **Indexes:** Partial indexes on active records (where deleted_at IS NULL)
- **Audit table:** JSONB for change tracking, INET for IP addresses
- **Foreign keys:** Implicit via UUID references

---

## 15. SQLC CONFIGURATION

**File:** `sqlc.yaml`

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/queries/"
    schema: "db/migrations/"
    gen:
      go:
        package: "sqlcgen"
        out: "gen/sqlc"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_empty_slices: true
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
          - db_type: "timestamptz"
            go_type: "time.Time"
          - db_type: "jsonb"
            go_type:
              import: "encoding/json"
              type: "RawMessage"
            nullable: true
```

**Details:**
- **Query location:** `db/queries/` (SQL files)
- **Schema location:** `db/migrations/`
- **Output package:** `sqlcgen` (in `gen/sqlc/`)
- **SQL driver:** pgx/v5
- **JSON tags:** Emitted automatically
- **Empty slices:** Emitted as empty (not null)
- **Type overrides:**
  - PostgreSQL `uuid` → Go `github.com/google/uuid.UUID`
  - PostgreSQL `timestamptz` → Go `time.Time`
  - PostgreSQL `jsonb` → Go `json.RawMessage` (nullable)

---

## MIDDLEWARE CHAIN ORDER

**File:** `internal/shared/middleware/chain.go` (lines 13-48)

Middleware execution order:
1. **Recovery** — Panic handling + stack trace logging
2. **Request ID** — UUID generation/injection for tracing
3. **Request Logger** — Latency, status, IP logging (sanitized)
4. **Body Limit** — 10 MB request body limit
5. **Gzip** — Response compression (level 5)
6. **Security Headers** — HSTS, CSP, X-Frame-Options, etc.
7. **CORS** — Cross-origin resource sharing
8. **Context Timeout** — 30-second global timeout
9. **Rate Limiting** — 100 req/min per user/IP (Redis-backed)
10. **Error Handler** — Centralized error response formatting
11. **Auth + RBAC** — Applied per-route (not global)

---

## KEY CONFIGURATION ENVIRONMENT VARIABLES

**Config struct:** `internal/shared/config/config.go`

```
APP_ENV               (default: "development")
APP_NAME              (default: "gnha-services")
PORT                  (default: 8080)
DATABASE_URL          (required)
REDIS_URL             (required)
RABBITMQ_URL          (required)
ELASTICSEARCH_URL     (default: "http://localhost:9200")
JWT_SECRET            (required, min 32 chars)
JWT_ACCESS_TTL        (default: 15m)
JWT_REFRESH_TTL       (default: 168h)
LOG_LEVEL             (default: "info")
OTEL_EXPORTER_OTLP_ENDPOINT  (default: "http://localhost:4317")
SMTP_HOST             (default: "localhost")
SMTP_PORT             (default: 1025)
SMTP_FROM             (default: "noreply@app.local")
CORS_ORIGINS          (default: "http://localhost:3000", comma-separated)
```

---

## UNRESOLVED QUESTIONS

1. **API versioning in HTTP:** How are Connect RPC routes versioned in the HTTP layer? Are they under `/api/v1/*` paths?
2. **Elasticsearch usage:** `ELASTICSEARCH_URL` is configured but not found in codebase — is it for future use?
3. **SMTP usage:** Configuration exists but not integrated — used for notification service?
4. **Token refresh flow:** How does client obtain new access tokens using refresh tokens?
5. **Permission assignment:** Where are user permissions loaded/cached? Are they in JWT or fetched per-request?
6. **Audit log triggers:** How are audit events triggered — via database triggers or application code?
7. **Rate limit per endpoint:** Is the 100 req/min global or can it be configured per endpoint?
8. **Database connection tuning:** Are the pool settings (25 max, 5 min) sufficient for expected load?

