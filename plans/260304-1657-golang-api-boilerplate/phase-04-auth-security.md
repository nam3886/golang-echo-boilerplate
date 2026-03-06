# Phase 4: Auth & Security

**Priority:** P0 | **Effort:** L (4-8h) | **Status:** completed
**Depends on:** Phase 3
**Completed:** 2026-03-04

## Context

- [Brainstorm Report](../reports/brainstorm-260304-1636-golang-api-boilerplate.md) — Security section
- [Architecture Patterns](../reports/researcher-260304-1437-golang-architecture-patterns.md) — JWT flow, RBAC, rate limiting

## Overview

Implement authentication (JWT + refresh token), password hashing (argon2id), RBAC middleware, API key management, rate limiting, and the full middleware chain in correct order.

## Files to Create

```
internal/shared/auth/jwt.go              # JWT generation + validation
internal/shared/auth/password.go         # argon2id hashing
internal/shared/auth/context.go          # User context helpers
internal/shared/middleware/auth.go       # JWT auth middleware
internal/shared/middleware/rbac.go       # Role/permission checking
internal/shared/middleware/rate_limit.go # Redis sliding window
internal/shared/middleware/cors.go       # CORS config
internal/shared/middleware/timeout.go    # Per-route timeout
internal/shared/middleware/body_limit.go # Request body limit
internal/shared/middleware/compress.go   # Gzip compression
internal/shared/middleware/chain.go      # Full middleware chain setup
internal/shared/auth/apikey.go           # API key generation + validation
proto/auth/v1/auth.proto                 # Auth service proto
db/queries/auth.sql                      # Refresh tokens, API keys queries
db/migrations/00002_auth_tables.sql      # refresh_tokens, api_keys tables
```

## Implementation Steps

### 1. Password hashing — argon2id
```go
// internal/shared/auth/password.go
func HashPassword(password string) (string, error) {
    salt := make([]byte, 16)
    crypto_rand.Read(salt)
    hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
    // Encode as $argon2id$v=19$m=65536,t=3,p=4$<salt>$<hash>
    return encoded, nil
}

func VerifyPassword(password, encoded string) (bool, error) {
    // Decode params, salt, hash from encoded string
    // Re-hash with same params and compare
}
```

### 2. JWT generation + validation
```go
// internal/shared/auth/jwt.go
type TokenClaims struct {
    jwt.RegisteredClaims
    UserID      string   `json:"uid"`
    Role        string   `json:"role"`
    Permissions []string `json:"perms,omitempty"`
}

func GenerateAccessToken(cfg *config.Config, user User) (string, error) {
    claims := TokenClaims{
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.JWTAccessTTL)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            ID:        uuid.NewString(), // jti for blacklisting
        },
        UserID: user.ID, Role: user.Role, Permissions: user.Permissions,
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(cfg.JWTSecret))
}

func GenerateRefreshToken() string {
    b := make([]byte, 32)
    crypto_rand.Read(b)
    return base64.URLEncoding.EncodeToString(b)
}
```

### 3. Auth middleware
```go
// internal/shared/middleware/auth.go
func AuthMiddleware(cfg *config.Config, rdb *redis.Client) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            token := extractBearerToken(c)
            if token == "" { return ErrUnauthorized }

            claims, err := auth.ValidateAccessToken(cfg, token)
            if err != nil { return ErrUnauthorized }

            // Check blacklist (logout)
            if blacklisted, _ := rdb.Exists(ctx, "blacklist:"+claims.ID).Result(); blacklisted > 0 {
                return ErrUnauthorized
            }

            ctx := auth.WithUser(c.Request().Context(), claims)
            c.SetRequest(c.Request().WithContext(ctx))
            return next(c)
        }
    }
}
```

### 4. RBAC middleware
```go
// internal/shared/middleware/rbac.go
type Permission string
const (
    PermUserRead   Permission = "user:read"
    PermUserWrite  Permission = "user:write"
    PermUserDelete Permission = "user:delete"
    PermAdminAll   Permission = "admin:*"
)

func RequirePermission(perms ...Permission) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            user := auth.UserFromContext(c.Request().Context())
            if user == nil { return ErrUnauthorized }
            for _, p := range perms {
                if !user.HasPermission(string(p)) {
                    return ErrForbidden
                }
            }
            return next(c)
        }
    }
}

func RequireRole(roles ...string) echo.MiddlewareFunc { ... }
```

### 5. Rate limiting — Redis sliding window
```go
// internal/shared/middleware/rate_limit.go
func RateLimitMiddleware(rdb *redis.Client) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            key := rateLimitKey(c) // user_id or IP
            // Redis MULTI: ZADD + ZREMRANGEBYSCORE + ZCARD
            // sliding window: 100 req/min per user, 20 req/min per IP
            count := slidingWindowCount(ctx, rdb, key, time.Minute)
            if count > limit {
                c.Response().Header().Set("Retry-After", "60")
                return echo.NewHTTPError(429, "rate limit exceeded")
            }
            return next(c)
        }
    }
}
```

### 6. API key management
```go
// internal/shared/auth/apikey.go
// API key format: "myapp_live_<random32>" (prefix identifies key type)
func GenerateAPIKey(prefix string) (plaintext string, hash string, err error) {
    b := make([]byte, 32)
    crypto_rand.Read(b)
    plaintext = prefix + "_" + base64.URLEncoding.EncodeToString(b)
    hash = sha256Hex(plaintext)
    return plaintext, hash, nil
}
// Store hash in DB, return plaintext to user ONCE
// Validate: hash incoming key, lookup in DB
```

### 7. Full middleware chain — CORRECT ORDER
```go
// internal/shared/middleware/chain.go
func SetupMiddleware(e *echo.Echo, cfg *config.Config, rdb *redis.Client) {
    // 1. Recovery
    e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
        StackSize: 4 << 10, LogErrorFunc: logPanic,
    }))
    // 2. Request ID
    e.Use(RequestIDMiddleware())
    // 3. Request Logger (with sanitization)
    e.Use(RequestLoggerMiddleware())
    // 4. Body Limit
    e.Use(middleware.BodyLimit("10M"))
    // 5. Gzip
    e.Use(middleware.GzipWithConfig(middleware.GzipConfig{Level: 5}))
    // 6. Security Headers
    e.Use(SecurityHeadersMiddleware())
    // 7. CORS
    e.Use(middleware.CORSWithConfig(corsConfig(cfg)))
    // 8. Global Timeout (30s default)
    e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{Timeout: 30 * time.Second}))
    // 9. Rate Limiting
    e.Use(RateLimitMiddleware(rdb))
    // 10. OpenTelemetry (otelecho)
    e.Use(otelecho.Middleware("myapp"))

    // 11-12: Auth + RBAC applied at route group level, not global
    e.HTTPErrorHandler = ErrorHandler
}
```

### 8. Auth service proto + migration
```protobuf
// proto/auth/v1/auth.proto
service AuthService {
  rpc Login(LoginRequest) returns (LoginResponse);
  rpc RefreshToken(RefreshTokenRequest) returns (RefreshTokenResponse);
  rpc Logout(LogoutRequest) returns (LogoutResponse);
}
```

Migration: `refresh_tokens` table (user_id, token_hash, family, expires_at) + `api_keys` table (user_id, name, key_hash, prefix, permissions, last_used_at, expires_at).

## Todo

- [x] argon2id password hashing + verification
- [x] JWT access token generation + validation (golang-jwt/jwt/v5)
- [x] Refresh token generation + Redis storage + rotation detection
- [x] Auth middleware (Bearer token extraction + validation + blacklist check)
- [x] RBAC middleware (RequirePermission, RequireRole)
- [x] Rate limiting middleware (Redis sliding window)
- [x] API key generation + validation + DB storage
- [x] CORS config (explicit origins, Connect-Protocol-Version header)
- [x] Security headers middleware (HSTS, X-Content-Type-Options, etc.)
- [x] Full middleware chain in correct order
- [x] Auth proto (Login, RefreshToken, Logout)
- [x] Auth migration (refresh_tokens, api_keys tables)
- [x] Auth sqlc queries
- [x] Connect interceptor for auth (gRPC side)
- [x] `task generate` includes new proto + queries
- [x] Verify: login → access token + refresh cookie → authenticated request → refresh → logout

## Success Criteria

- Login returns JWT + sets HTTP-only refresh cookie
- Authenticated requests work with Bearer token
- Expired token → 401
- Refresh token rotation works (reuse detection)
- RBAC blocks unauthorized role/permission
- Rate limit returns 429 with Retry-After header
- API key authentication works for public API endpoints
- Middleware chain order verified (recovery first, auth at group level)

## Risk Assessment

- **Refresh token rotation:** Must handle race conditions (concurrent refresh). Use Redis SETNX for atomic family check.
- **argon2id tuning:** Default params (m=64MB, t=3, p=4) may be slow on weak VPS. Benchmark and adjust.

## Next Steps

→ Phase 5: Example Module (user module with full hexagonal structure)
