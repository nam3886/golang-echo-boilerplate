# Authentication

## Token Generation

`auth.GenerateAccessToken(cfg, userID, role, permissions)` — returns a signed HS256 JWT.
`auth.GenerateRefreshToken()` — returns a 32-byte cryptographically random base64 string.

> **Note:** `GenerateRefreshToken` is a placeholder. It generates a random token string but has
> **no server-side storage, rotation, or revocation** implemented. Storing refresh tokens,
> rotating them on use, and revoking them on logout is left to application-specific implementation.

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

## Testing Authenticated Endpoints

### Seed Test Users

Run `task seed` to populate the database with test users:

```
admin@example.com  / Admin@123456  (role: admin)
member@example.com / Member@123456 (role: member)
viewer@example.com / Viewer@123456 (role: viewer)
```

### Using Swagger UI

Navigate to `http://localhost:8080/swagger/` (after starting the dev server with `task dev`). Use the "Authorize" button to enter a Bearer token and test endpoints interactively.

### Login/Logout Not Included — Building Blocks Only

This boilerplate provides JWT infrastructure as building blocks. There are **no `/login` or `/logout` endpoints** built in — these are intentionally left for application-specific implementation.

Provided building blocks:
- `auth.GenerateAccessToken(cfg, userID, role, permissions)` — mint a signed JWT
- `auth.GenerateRefreshToken()` — generate a random refresh token string
- `auth.BlacklistToken(ctx, rdb, jti, expiry)` — revoke a token on logout
- `middleware.Auth(cfg, rdb)` — validate + blacklist-check on every protected route

### Example: Generate a Token Programmatically for Testing

```go
import (
    "github.com/gnha/golang-echo-boilerplate/internal/shared/auth"
    "github.com/gnha/golang-echo-boilerplate/internal/shared/config"
)

// cfg loaded from environment (JWT_SECRET, APP_NAME, etc.)
token, err := auth.GenerateAccessToken(cfg, userID, "admin", []string{"user:read", "user:write"})
```

Then use the token in cURL:

```bash
curl http://localhost:8080/api/user/v1/UserService/GetUser \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"id":"<user-uuid>"}'
```
