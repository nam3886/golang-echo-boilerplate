# Authentication

## Token Generation

`auth.GenerateAccessToken(cfg, userID, role, permissions)` — returns a signed HS256 JWT.
`auth.GenerateRefreshToken()` — returns a 32-byte cryptographically random base64 string.

Source: `internal/shared/auth/jwt.go`

## Token Structure (Claims)

| Claim | JSON key | Description |
|-------|----------|-------------|
| `iss` | standard | `APP_NAME` env value |
| `aud` | standard | `"golang-echo-boilerplate"` |
| `exp` | standard | `now + JWT_ACCESS_TTL` (default 15m) |
| `iat` | standard | issued-at timestamp |
| `jti` | standard | UUID — used for blacklisting |
| `uid` | custom   | user ID |
| `role` | custom  | user role string |
| `perms` | custom | `[]string` of permission strings |

## Token Validation

`auth.ValidateAccessToken(cfg, tokenStr)` — parses and verifies:
- Signing method is HMAC
- Issuer matches `APP_NAME`
- Audience matches `"golang-echo-boilerplate"`
- Signature valid against `JWT_SECRET`
- Not expired (standard `exp` check)

Returns `*TokenClaims` or an error.

## Blacklisting (Logout)

Tokens are blacklisted by `jti` in Redis to support logout before expiry.

- `auth.BlacklistToken(ctx, rdb, jti, tokenExpiry)` — writes `blacklist:{jti}` with TTL = remaining token lifetime. No-ops if already expired.
- `auth.IsBlacklisted(ctx, rdb, jti)` — checks existence of the Redis key.

Source: `internal/shared/auth/blacklist.go`

## Middleware Flow

`middleware.Auth(cfg, rdb)` (Echo middleware):

1. Extract `Bearer <token>` from `Authorization` header.
2. Call `auth.ValidateAccessToken` — reject if invalid/expired.
3. Call `auth.IsBlacklisted` — **fail closed**: any Redis error rejects the request.
4. Call `auth.WithUser(ctx, claims)` to inject `AuthUser` into request context.
5. Pass to next handler.

Source: `internal/shared/middleware/auth.go`

## Context Access

```go
user := auth.UserFromContext(ctx)  // returns *AuthUser or nil
// Fields: UserID, Role, Permissions []string, TokenID (jti)
```

Source: `internal/shared/auth/context.go`

## Config Variables

| Env var | Required | Default | Description |
|---------|----------|---------|-------------|
| `JWT_SECRET` | yes | — | HS256 signing key (min 32 chars) |
| `JWT_ACCESS_TTL` | no | `15m` | Access token lifetime |
| `JWT_REFRESH_TTL` | no | `168h` | Refresh token lifetime (7d) |
| `APP_NAME` | no | `golang-echo-boilerplate` | Used as JWT issuer |
