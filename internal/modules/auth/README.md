# Auth Module

## Purpose

Handles user authentication: login (JWT access token issuance) and logout (token revocation via blacklist).

## Structure

```
app/             — LoginHandler, LogoutHandler + unit tests
adapters/grpc/   — Connect RPC handler, routes
module.go        — fx.Module wiring
```

## Events

| Topic | Published by | Consumed by |
|-------|-------------|-------------|
| `user.logged_in` | LoginHandler | audit |
| `user.logged_out` | LogoutHandler | audit |

## Failure Modes

| Dependency | Failure | Behavior |
|------------|---------|----------|
| PostgreSQL | Unavailable | Login fails (fail-closed) — credential lookup returns error |
| Redis (token blacklist) | Unavailable on logout | **Fail-closed** — logout returns 500; token is NOT blacklisted. Configure `BLACKLIST_FAIL_OPEN=true` to allow logout to succeed without blacklisting (reduced security). |
| Redis (token blacklist) | Unavailable on auth middleware | **Fail-closed** by default — request rejected. Set `BLACKLIST_FAIL_OPEN=true` for fail-open with local cache fallback. |
| RabbitMQ | Unavailable | Login/logout events not published; auth operation succeeds (fail-open for events only) |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | required | HMAC-SHA256 signing key |
| `JWT_ACCESS_TTL` | `15m` | Access token lifetime |
| `BLACKLIST_FAIL_OPEN` | `false` | Redis blacklist failure strategy (`false` = fail-closed) |

## Security Notes

- Login returns a single access token. Refresh tokens are not implemented — there is no `/refresh` endpoint.
- Logout blacklists the access token JTI in Redis with the token's remaining TTL.
- Both user-not-found and wrong-password return the same `invalid_credentials` error to prevent email oracle attacks.
- See `docs/runbooks/jwt-rotation.md` for key rotation procedure.
