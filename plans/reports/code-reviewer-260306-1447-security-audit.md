# Security Audit Report -- gnha-services

**Date:** 2026-03-06
**Reviewer:** code-reviewer
**Scope:** All security-related code (auth, middleware, config, input validation, error handling)
**Files reviewed:** 18 files across auth/, middleware/, config/, user module, Dockerfile, .env.example

---

## Overall Security Posture: 8/10

The codebase demonstrates solid security fundamentals: Argon2id password hashing with constant-time comparison, JWT algorithm pinning, token blacklisting, centralized error handling that avoids leaking internals, protovalidate input validation, parameterized queries via sqlc, Redis sliding-window rate limiting, comprehensive security headers, and RBAC at both Echo and Connect RPC layers.

---

## Findings

### CRITICAL -- None

No critical vulnerabilities found. No hardcoded secrets in source code, no SQL injection vectors, no authentication bypass paths.

---

### HIGH

#### H-1: Rate limiter fails open on Redis outage
**File:** `internal/shared/middleware/rate_limit.go:19-21`
**OWASP:** A04:2021 -- Insecure Design
When Redis is unreachable, the rate limiter silently allows all requests through. An attacker who can induce Redis connection failures (or during a Redis outage) gets unlimited request throughput.

```go
if err != nil {
    // On Redis failure, allow request (fail open)
    return next(c)
}
```

**Impact:** Brute-force attacks, credential stuffing, DoS amplification during infrastructure instability.
**Recommendation:** Add a local in-memory fallback rate limiter (e.g., `golang.org/x/time/rate`) that activates when Redis is unavailable. At minimum, log the failure at WARN level so ops can detect it.

---

#### H-2: Dockerfile runs as root
**File:** `Dockerfile:13-19`
**OWASP:** A05:2021 -- Security Misconfiguration
The runtime container runs the binary as root. Container escape vulnerabilities become full host compromises.

**Recommendation:** Add a non-root user:
```dockerfile
RUN addgroup -S app && adduser -S -G app app
USER app
```

---

#### H-3: No password complexity enforcement beyond min length 8
**File:** `proto/user/v1/user.proto:30` -- `min_len = 8` only
**OWASP:** A07:2021 -- Identification and Authentication Failures
The only password validation is `min_len: 8` in proto validation. No uppercase/lowercase/digit/special requirements, no breach-list check, no maximum length (Argon2id will hash arbitrarily long passwords, but extremely long passwords can cause DoS via CPU).

**Recommendation:**
- Add `max_len: 128` to the proto validation rule
- Consider server-side password policy check (complexity or zxcvbn score) in the `CreateUserHandler`

---

### MEDIUM

#### M-1: RBAC too coarse -- all user endpoints require only `user:read`
**File:** `internal/modules/user/adapters/grpc/routes.go:26`
The Echo group applies `RequirePermission(PermUserRead)` to all user endpoints. The `RBACInterceptor` adds write/delete checks for Connect RPC procedures, but the base group permission means a user with only `user:read` reaches the interceptor layer. This is defense-in-depth (interceptor blocks them), but the group-level permission is misleading and could cause confusion when adding non-Connect endpoints to the same group.

**Status:** Partially mitigated by `RBACInterceptor`. Document the layered approach or tighten group permissions per HTTP method.

---

#### M-2: Internal errors leak wrapped messages to Connect RPC clients
**File:** `internal/modules/user/adapters/grpc/mapper.go:32`
```go
return connect.NewError(connect.CodeInternal, err)
```
When an error is NOT a `DomainError`, the raw Go error (which may contain DB connection strings, SQL snippets, or internal paths) is passed directly as the Connect error message. The HTTP `ErrorHandler` correctly returns "internal error", but the Connect RPC path does not.

**Impact:** Information disclosure to API consumers.
**Recommendation:** Replace with:
```go
return connect.NewError(connect.CodeInternal, errors.New("internal error"))
```
And log the original error server-side.

---

#### M-3: No `Issuer` or `Audience` claims in JWT
**File:** `internal/shared/auth/jwt.go:25-29`
The JWT has no `iss` or `aud` claims. If multiple services share the same JWT secret (common in microservice transitions), tokens from one service are valid in another.

**Recommendation:** Set `Issuer` and `Audience` in `RegisteredClaims` and validate them in `ValidateAccessToken` using `jwt.WithIssuer()` and `jwt.WithAudience()` parser options.

---

#### M-4: CORS allows credentials with configurable origins -- verify prod config
**File:** `internal/shared/middleware/chain.go:27-36`
`AllowCredentials: true` with `AllowOrigins` from env var. This is correct IF production CORS_ORIGINS is tightly scoped. The default is `http://localhost:3000` which is fine for dev. Just ensure production deployment sets this to the exact frontend origin(s).

**Status:** No issue if deployed correctly. Flag for deployment checklist.

---

#### M-5: Token blacklist check ignores Redis errors
**File:** `internal/shared/middleware/auth.go:29`
```go
if blacklisted, _ := rdb.Exists(ctx, "blacklist:"+claims.RegisteredClaims.ID).Result(); blacklisted > 0 {
```
The error from `rdb.Exists` is silently discarded. If Redis is down, blacklisted tokens (logged-out sessions) are accepted as valid.

**Impact:** Logged-out tokens remain active during Redis outages.
**Recommendation:** On Redis error, reject the request (fail closed for security-critical checks) or at minimum log the error.

---

#### M-6: No Content-Security-Policy header
**File:** `internal/shared/middleware/security.go`
The security headers are good (X-Content-Type-Options, X-Frame-Options, HSTS, Permissions-Policy) but `Content-Security-Policy` is missing. This matters for the Swagger UI page which loads scripts from `unpkg.com`.

**Recommendation:** Add CSP header, at least for API responses: `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`. The Swagger route can override with a more permissive CSP.

---

### LOW

#### L-1: JWT_SECRET in .env.example is a guessable placeholder
**File:** `.env.example:19`
`JWT_SECRET=change-me-in-production-use-a-strong-secret`
The config loader enforces 32+ characters (good), but the placeholder itself is 46 characters and passes that check. If someone copies .env.example to .env without changing it, the app starts with a known secret.

**Recommendation:** Make the placeholder shorter than 32 chars so `config.Load()` rejects it, e.g.: `JWT_SECRET=CHANGE_ME`

---

#### L-2: Swagger UI loads scripts from third-party CDN
**File:** `internal/shared/middleware/swagger.go:62-65`
Loading `swagger-ui-dist@5` from `unpkg.com` introduces a supply-chain risk. If unpkg is compromised, the Swagger page serves malicious JS.

**Mitigation:** Only served in non-production (`cfg.AppEnv == "production"` early return). Acceptable for dev. Add a comment noting this is intentionally dev-only.

---

#### L-3: `X-XSS-Protection: 1; mode=block` is deprecated
**File:** `internal/shared/middleware/security.go:12`
Modern browsers have removed XSS Auditor. Setting this header can introduce side-channel vulnerabilities in older browsers. Current best practice is `X-XSS-Protection: 0` with a proper CSP instead.

---

#### L-4: Recovery middleware stack buffer is fixed at 4096 bytes
**File:** `internal/shared/middleware/recovery.go:18`
Deep stacks may be truncated. Minor, but `debug.Stack()` would capture the full trace without manual buffer management.

---

## Positive Observations

1. **Argon2id + constant-time compare** -- Best-in-class password hashing
2. **JWT algorithm pinning** (`SigningMethodHMAC` type assertion) -- Prevents alg-none and RS/HS confusion attacks
3. **Token blacklisting** -- Proper logout support via Redis
4. **Parameterized queries** -- sqlc generates parameterized SQL, no injection vectors found
5. **protovalidate interceptor** -- Input validation at the RPC boundary (email format, UUID format, role enum, length constraints)
6. **Centralized error handler** -- HTTP path returns generic "internal error" for unexpected errors
7. **Security headers** -- Comprehensive set including HSTS, X-Frame-Options DENY, Permissions-Policy
8. **Body size limit** -- 10MB via Echo middleware
9. **Request timeout** -- 30s global context timeout prevents slow-loris
10. **JWT secret length enforcement** -- Rejects secrets under 32 chars at startup
11. **X-Request-ID length cap** -- Rejects IDs over 128 chars (prevents header injection)
12. **Swagger disabled in production** -- Reduces attack surface

---

## Summary Table

| # | Severity | Finding | OWASP |
|---|----------|---------|-------|
| H-1 | HIGH | Rate limiter fails open | A04 |
| H-2 | HIGH | Container runs as root | A05 |
| H-3 | HIGH | Weak password policy | A07 |
| M-1 | MEDIUM | RBAC group too coarse | A01 |
| M-2 | MEDIUM | Internal errors leak to gRPC clients | A04 |
| M-3 | MEDIUM | No JWT issuer/audience | A07 |
| M-4 | MEDIUM | CORS credentials -- verify prod | A05 |
| M-5 | MEDIUM | Blacklist check ignores Redis errors | A07 |
| M-6 | MEDIUM | No CSP header | A05 |
| L-1 | LOW | .env.example secret passes validation | A07 |
| L-2 | LOW | CDN-loaded Swagger JS | A08 |
| L-3 | LOW | Deprecated X-XSS-Protection | A05 |
| L-4 | LOW | Fixed-size recovery stack buffer | -- |

---

## Recommended Priority Actions

1. **H-2** Fix Dockerfile non-root user (5 min, zero risk)
2. **M-2** Sanitize internal errors in Connect RPC mapper (5 min)
3. **H-1** Add in-memory fallback rate limiter or fail-closed behavior (30 min)
4. **M-5** Fail closed on blacklist Redis errors (10 min)
5. **H-3** Add password max_len and consider complexity check (15 min)
6. **M-3** Add JWT issuer/audience claims (15 min)
7. **M-6** Add CSP header (10 min)
8. **L-1** Shorten .env.example JWT_SECRET placeholder (1 min)

---

## Unresolved Questions

- Is there a deployment runbook that verifies CORS_ORIGINS is correctly set for production?
- Will additional modules (beyond user) share the same JWT secret, making M-3 (missing iss/aud) more urgent?
- Is there a plan to add login/logout endpoints? Token blacklisting exists but no login endpoint means it cannot be tested end-to-end.
