# Phase 2: Shared Infrastructure

**Priority:** P0 | **Effort:** L (4-8h) | **Status:** completed
**Depends on:** Phase 1
**Completed:** 2026-03-04

## Context

- [Architecture Patterns](../reports/researcher-260304-1437-golang-architecture-patterns.md) — Error handling, logging, DB patterns

## Overview

Build shared infrastructure layer: PostgreSQL connection pool with retry, Redis client, structured logging (slog), OpenTelemetry setup, domain error types, and BaseModel.

## Files to Create

```
internal/shared/database/postgres.go        # pgx pool + connect with retry
internal/shared/database/redis.go           # go-redis client
internal/shared/observability/logger.go     # slog setup (multi-handler)
internal/shared/observability/tracer.go     # OTel tracer provider
internal/shared/observability/metrics.go    # OTel meter provider
internal/shared/errors/domain_error.go      # DomainError type + error codes
internal/shared/errors/codes.go             # Error code registry
internal/shared/model/base.go              # BaseModel (ID, timestamps, soft delete)
internal/shared/middleware/recovery.go      # Panic recovery
internal/shared/middleware/request_id.go    # Request ID generation/propagation
internal/shared/middleware/request_log.go   # Request/response logging with sanitization
internal/shared/middleware/security.go      # Security headers
internal/shared/middleware/error_handler.go # Centralized Echo error handler
```

## Implementation Steps

### 1. PostgreSQL pool with retry
```go
// internal/shared/database/postgres.go
func NewPostgresPool(cfg *config.Config) (*pgxpool.Pool, error) {
    var pool *pgxpool.Pool
    for i := range 10 { // Go 1.26
        poolCfg, _ := pgxpool.ParseConfig(cfg.DatabaseURL)
        poolCfg.MaxConns = 25
        poolCfg.MinConns = 5
        poolCfg.MaxConnLifetime = 1 * time.Hour
        poolCfg.MaxConnIdleTime = 30 * time.Minute
        pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
        if err == nil {
            if err = pool.Ping(ctx); err == nil {
                return pool, nil
            }
        }
        slog.Warn("db not ready, retrying", "attempt", i+1, "err", err)
        time.Sleep(time.Duration(i+1) * time.Second)
    }
    return nil, fmt.Errorf("postgres connection failed after retries")
}
```
Register in Fx with `fx.Lifecycle` OnStop → `pool.Close()`.

### 2. Redis client
```go
// internal/shared/database/redis.go
func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
    opt, _ := redis.ParseURL(cfg.RedisURL)
    opt.PoolSize = 10 * runtime.NumCPU()
    opt.MinIdleConns = 5
    rdb := redis.NewClient(opt)
    // Retry ping similar to postgres
    return rdb, nil
}
```

### 3. slog setup with Go 1.26 NewMultiHandler
```go
// internal/shared/observability/logger.go
func NewLogger(cfg *config.Config) *slog.Logger {
    level := parseLevel(cfg.LogLevel)
    var handlers []slog.Handler

    if cfg.AppEnv == "development" {
        handlers = append(handlers, slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            Level: level, AddSource: true,
        }))
    } else {
        handlers = append(handlers, slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
            Level: level,
        }))
    }
    // OTLP handler added in Phase 8 (SigNoz)
    logger := slog.New(slog.NewMultiHandler(handlers...))
    slog.SetDefault(logger)
    return logger
}
```

### 4. OpenTelemetry tracer + meter
```go
// internal/shared/observability/tracer.go
func NewTracerProvider(cfg *config.Config) (*sdktrace.TracerProvider, error) {
    exporter, _ := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
        otlptracegrpc.WithInsecure(), // dev only
    )
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("myapp"),
            semconv.ServiceVersion("0.1.0"),
        )),
    )
    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{}, propagation.Baggage{},
    ))
    return tp, nil
}
```

### 5. Domain error types
```go
// internal/shared/errors/domain_error.go
type ErrorCode string
const (
    CodeInvalidArgument    ErrorCode = "INVALID_ARGUMENT"
    CodeNotFound           ErrorCode = "NOT_FOUND"
    CodeAlreadyExists      ErrorCode = "ALREADY_EXISTS"
    CodePermissionDenied   ErrorCode = "PERMISSION_DENIED"
    CodeUnauthenticated    ErrorCode = "UNAUTHENTICATED"
    CodeFailedPrecondition ErrorCode = "FAILED_PRECONDITION"
    CodeInternal           ErrorCode = "INTERNAL"
    CodeUnavailable        ErrorCode = "UNAVAILABLE"
)

type DomainError struct {
    Code    ErrorCode
    Message string
    Err     error
}
func (e *DomainError) Error() string   { return e.Message }
func (e *DomainError) Unwrap() error   { return e.Err }
func (e *DomainError) HTTPStatus() int { return codeToHTTP[e.Code] }
// Connect RPC code mapping: codeToConnect[e.Code]

// Sentinel errors
var (
    ErrNotFound      = &DomainError{Code: CodeNotFound, Message: "not found"}
    ErrAlreadyExists = &DomainError{Code: CodeAlreadyExists, Message: "already exists"}
    ErrForbidden     = &DomainError{Code: CodePermissionDenied, Message: "forbidden"}
    ErrUnauthorized  = &DomainError{Code: CodeUnauthenticated, Message: "unauthorized"}
)
```

### 6. BaseModel
```go
// internal/shared/model/base.go
type BaseModel struct {
    ID        uuid.UUID  `json:"id" db:"id"`
    CreatedAt time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
    DeletedAt *time.Time `json:"-" db:"deleted_at"`
}
```

### 7. Core middleware
- **Recovery:** Catch panics, log stack trace, return 500
- **Request ID:** Generate UUID if missing `X-Request-ID`, inject into context + slog
- **Request Logger:** Log method, path, status, latency. Sanitize: redact Authorization header, password fields
- **Security Headers:** HSTS, X-Content-Type-Options, X-Frame-Options
- **Error Handler:** Translate `DomainError` → HTTP response with correct status + code. Log unexpected errors.

### 8. Fx module registration
```go
// internal/shared/module.go
var Module = fx.Module("shared",
    fx.Provide(config.Load),
    fx.Provide(database.NewPostgresPool),
    fx.Provide(database.NewRedisClient),
    fx.Provide(observability.NewLogger),
    fx.Provide(observability.NewTracerProvider),
    fx.Provide(observability.NewMeterProvider),
)
```

## Todo

- [x] PostgreSQL pool with retry logic
- [x] Redis client with retry
- [x] slog multi-handler setup (text dev / JSON prod)
- [x] OpenTelemetry tracer provider
- [x] OpenTelemetry meter provider
- [x] DomainError type + error codes + HTTP/Connect mapping
- [x] Sentinel errors (NotFound, AlreadyExists, Forbidden, etc.)
- [x] BaseModel with soft delete
- [x] Recovery middleware
- [x] Request ID middleware
- [x] Request logger middleware (with log sanitization)
- [x] Security headers middleware
- [x] Centralized Echo error handler
- [x] Shared Fx module
- [x] Verify: app starts with DB + Redis connected, logs structured

## Success Criteria

- App connects to PostgreSQL/Redis on startup with retry
- Structured logs output (text in dev, JSON in prod)
- Request ID generated and propagated through logs
- DomainError → correct HTTP status + JSON error body
- Panic in handler → 500 response + stack trace logged
- Security headers present in responses

## Next Steps

→ Phase 3: Code Gen Pipeline (buf, sqlc, proto)
