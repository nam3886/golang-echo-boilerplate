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
| `JWT_REFRESH_TTL` | no | `168h` | Refresh token lifetime (7 days); shorten to reduce exposure window after credential leak |
| `APP_NAME` | no | `golang-echo-boilerplate` | Used as JWT issuer |
| `BLACKLIST_FAIL_OPEN` | no | `false` | Behavior when Redis is unreachable during token blacklist check. `false` (fail-closed): reject request — security over availability. `true` (fail-open): allow request — only use when HA is critical + local cache configured |

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

## Login & Logout Endpoints

The auth module provides two RPC endpoints for user authentication.

### Login Endpoint

**POST /auth.v1.AuthService/Login**

Authenticates a user by email and password, returning an access token + refresh token pair.

Request:
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

Response (success):
```json
{
  "access_token": "eyJhbGci...",
  "refresh_token": "abc123...",
  "expires_in": 900
}
```

Error responses:
- `UNAUTHENTICATED` (401) — Invalid email or password
- `INVALID_ARGUMENT` (400) — Email format invalid or password too short

**Flow:**
1. Look up user by email via `CredentialLookup.GetByEmail()` — returns userID, hashedPassword, role
2. Verify password with `PasswordHasher.Verify()`
3. Generate access token with `auth.GenerateAccessToken(cfg, userID, role, permissions)`
4. Generate refresh token with `auth.GenerateRefreshToken()`
5. Publish `UserLoggedInEvent` to audit trail (fail-open — event publishing failures don't fail the request)
6. Return tokens + expiry

Source: `internal/modules/auth/app/login.go`

### Logout Endpoint

**POST /auth.v1.AuthService/Logout**

Revokes the caller's current access token by blacklisting it in Redis.

Request: `{}` (empty body)

Response (success): `{}` (empty body)

Error responses:
- `UNAUTHENTICATED` (401) — No valid token in Authorization header
- `INTERNAL` (500) — Redis unavailable (fail-closed — blacklist failure rejects request)

**Flow:**
1. Extract token claims from request context (populated by Auth middleware)
2. Blacklist token by JTI in Redis with TTL = remaining token lifetime
3. Publish `UserLoggedOutEvent` to audit trail (fail-open)
4. Return success

Source: `internal/modules/auth/app/logout.go`

## CredentialLookup Interface

The auth module decouples authentication from user storage via the `CredentialLookup` interface:

```go
type CredentialLookup interface {
    GetByEmail(ctx context.Context, email string) (userID, hashedPassword, role string, err error)
}
```

The user module provides the implementation in `adapters/credential_adapter.go`:

```go
func (r *PgUserRepository) GetByEmail(ctx context.Context, email string) (string, string, string, error) {
    // Query: SELECT id, password, role FROM users WHERE email = ? AND deleted_at IS NULL
    // Returns userID, hashedPassword, role
}
```

**Why this pattern?** Auth module needs user credentials but must not import from user module (no cross-module imports). The interface is defined in `internal/shared/auth/` (shared layer) and implemented by the user module, breaking the circular dependency.

## Provided Building Blocks

For custom authentication flows, use these lower-level functions:
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
