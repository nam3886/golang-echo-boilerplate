# Code Review: gnha-services Go API Boilerplate

**Reviewer**: code-reviewer
**Date**: 2026-03-04
**Scope**: Full codebase review (8 phases)

---

## Scope

- **Files reviewed**: 48 Go files + 2 SQL migrations + 2 proto files
- **LOC (internal + cmd)**: ~2,523
- **Focus**: Security, architecture, error handling, code quality, edge cases
- **Generated code skipped**: `gen/sqlc/*.go`, `gen/proto/**` (read for context only)

---

## Overall Assessment

This is a well-structured Go modular monolith boilerplate with solid architectural foundations. The hexagonal architecture is correctly implemented with clean dependency direction. Security primitives (argon2id, JWT with signing method validation, RBAC) are implemented correctly. The code is clean, idiomatic Go with good separation of concerns.

However, there are several issues ranging from critical security gaps to medium-priority correctness concerns that should be addressed before production use.

---

## Critical Issues

### C1. JWT Secret Minimum Length Not Enforced

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/config/config.go` (line 29)
**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/auth/jwt.go` (line 35)

The `JWTSecret` is loaded from env with no minimum length validation. An operator could set a weak 1-character secret and the system would happily sign tokens with it.

```go
// config.go - Add validation in Load()
func Load() (*Config, error) {
    cfg := &Config{}
    if err := env.Parse(cfg); err != nil {
        return nil, fmt.Errorf("loading config: %w", err)
    }
    if len(cfg.JWTSecret) < 32 {
        return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
    }
    return cfg, nil
}
```

**Impact**: A weak JWT secret can be brute-forced, allowing forged tokens.

### C2. `uuid.MustParse` Panics on Invalid Input from External Sources

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` (lines 32, 82, 110, 126, 139)

`uuid.MustParse` is used to convert `domain.UserID` (which is ultimately a `string` from user input via gRPC) into `uuid.UUID`. If the string is not a valid UUID, this panics and crashes the request (caught by recovery middleware, but still a 500 instead of 400).

```go
// Fix: validate and return error instead of panicking
func (r *PgUserRepository) GetByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
    parsedID, err := uuid.Parse(string(id))
    if err != nil {
        return nil, sharederr.New(sharederr.CodeInvalidArgument, "invalid user ID format")
    }
    q := sqlcgen.New(r.pool)
    row, err := q.GetUserByID(ctx, parsedID)
    // ...
}
```

**Impact**: Any invalid UUID string in a request causes a panic -> 500 instead of a clean 400 error. The proto validation (`string.uuid = true`) mitigates this for Connect RPC clients, but the domain layer should not rely on transport-layer validation.

### C3. Notification Template Vulnerable to XSS via Stored Data

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/notification/subscriber.go` (lines 50-57)

The welcome email template uses `html/template` (good), but the template syntax `{{.Name}}`, `{{.Email}}`, `{{.Role}}` auto-escapes in `html/template`. This is actually safe. **Downgraded from critical after verification** -- `html/template` auto-escapes. No action needed.

### C4. SMTP Email Header Injection

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/notification/email.go` (lines 27-33)

The `Send` method constructs raw SMTP headers by string concatenation. If `to`, `subject`, or `body` contain CRLF characters (`\r\n`), an attacker could inject additional headers (e.g., BCC to exfiltrate data).

```go
// Fix: sanitize inputs
func sanitizeHeader(s string) string {
    s = strings.ReplaceAll(s, "\r", "")
    s = strings.ReplaceAll(s, "\n", "")
    return s
}

func (s *SMTPSender) Send(_ context.Context, to, subject, body string) error {
    to = sanitizeHeader(to)
    subject = sanitizeHeader(subject)
    // ... rest of method
}
```

**Impact**: If a user registers with an email containing CRLF sequences, the welcome notification could be manipulated to add arbitrary SMTP headers.

---

## High Priority

### H1. Cursor Pagination is Broken for ListUsers

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` (lines 54-86)

The `CursorID` parameter in the SQL query expects a UUID for the tuple comparison `(created_at, id) < ($2, $3)`, but the code sets `CursorID` as a `pgtype.Timestamptz` with the same timestamp value as `CursorCreatedAt`. The comment on line 64 acknowledges this: "Better approach: use separate query params in production."

This means cursor pagination will skip or duplicate rows when multiple users share the same `created_at` timestamp, because the ID tiebreaker is wrong.

```go
// The SQL uses: (created_at, id) < ($2, $3)
// But $3 receives a Timestamptz, not a UUID.
// This will cause a Postgres type error or incorrect results.
```

**Fix**: Either change the SQL to use separate parameters, or properly encode the UUID in the cursor and pass it as a UUID parameter.

### H2. Update Transaction Uses Non-Locking Read (Lost Updates)

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` (lines 102-135)

The `Update` method begins a transaction, reads the user, applies mutations, then writes. However, `GetUserByID` does not use `SELECT ... FOR UPDATE`, so concurrent updates can produce lost-update anomalies.

```go
// Fix: use a locking query or add a separate sqlc query with FOR UPDATE
// In user.sql, add:
// -- name: GetUserByIDForUpdate :one
// SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;
```

**Impact**: Under concurrent updates, one update can silently overwrite another.

### H3. Event Publish Failure Silently Swallowed

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user.go` (line 61)

```go
_ = h.bus.Publish(ctx, events.TopicUserCreated, events.UserCreatedEvent{...})
```

The publish error is discarded with `_`. If RabbitMQ is down, the user is created but no audit log or welcome email is sent, with no indication of failure.

**Options**:
1. Log the error (minimum): `if err := h.bus.Publish(...); err != nil { slog.Error("failed to publish user.created", "err", err) }`
2. Use the transactional outbox pattern for guaranteed delivery.

### H4. Audit Trail Always Uses Self as Actor (Incorrect Attribution)

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/audit/subscriber.go` (lines 39, 59, 75)

```go
ActorID: uuid.MustParse(event.UserID), // self-created
```

The audit log always records the user as the actor. But for admin-initiated updates or deletions, the actual actor (the admin) is lost. The events should carry an `ActorID` field populated from the auth context.

### H5. No Input Validation in Application Layer

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user.go`

The `CreateUserCmd` does not validate:
- Email format (relies entirely on proto validation, bypassed if called from non-gRPC code)
- Password strength/minimum length
- Empty password (would hash an empty string)

The domain layer validates email non-emptiness but not format. For a boilerplate, this should have at minimum password length validation in the app layer.

### H6. CORS `AllowCredentials: true` with Wildcard-Compatible Origins

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/chain.go` (lines 27-36)

`AllowCredentials: true` is set alongside a configurable `AllowOrigins`. If an operator sets `CORS_ORIGINS=*`, this becomes a security misconfiguration (browsers actually reject `Access-Control-Allow-Origin: *` with credentials, but Echo's CORS middleware may reflect the requesting origin instead, effectively allowing any origin).

**Fix**: Add a validation in `config.Load()` that rejects `*` when credentials mode is expected.

---

## Medium Priority

### M1. Distributed Cron Lock Released Immediately After Job Completes

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/cron/scheduler.go` (lines 33-37)

```go
locked, err := s.rdb.SetNX(ctx, "cron:"+name, "locked", 5*time.Minute).Result()
// ...
defer s.rdb.Del(ctx, "cron:"+name)
```

The lock is acquired with a 5-minute TTL but released immediately via `defer Del` when the job function returns. If a job finishes in 100ms, there is a 4:59 window where another instance could also run the job (though unlikely within the same cron tick). The real issue is if the job is designed to run less frequently than every 5 minutes -- the lock provides no protection between cron ticks.

Consider: keep the lock for the full TTL (remove the `defer Del`) if you want to ensure at-most-once per 5-minute window, or use the cron interval as the TTL.

### M2. Rate Limiter Fails Open

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/rate_limit.go` (lines 19-20)

```go
if err != nil {
    // On Redis failure, allow request (fail open)
    return next(c)
}
```

This is a design decision documented in a comment (good), but worth noting: if Redis goes down, rate limiting is completely disabled. For a production system, consider a local in-memory fallback.

### M3. No Graceful Shutdown for TracerProvider and MeterProvider

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/module.go`

The `TracerProvider` and `MeterProvider` are created but never shut down via `fx.Lifecycle`. This means in-flight traces and metrics may be lost on shutdown.

```go
// Fix: Add fx.Invoke that registers shutdown hooks
fx.Invoke(func(lc fx.Lifecycle, tp *sdktrace.TracerProvider) {
    lc.Append(fx.Hook{
        OnStop: func(ctx context.Context) error {
            return tp.Shutdown(ctx)
        },
    })
}),
```

### M4. `BaseModel` in `internal/shared/model/base.go` Appears Unused

The `BaseModel` struct with `json` and `db` tags is defined but never referenced. The domain uses its own fields, and sqlc generates its own models. This is dead code.

### M5. `sensitiveHeaders` Map Defined But Not Used in Request Logger

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/request_log.go`

The `sensitiveHeaders` map and `SanitizeHeader` function are defined but never called within `RequestLogger()`. The logger does not log headers at all, so the sanitization code is dead code.

### M6. Watermill Router `Run(ctx)` Receives OnStart Context (Short-Lived)

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/events/subscriber.go` (line 59)

```go
go func() {
    if err := router.Run(ctx); err != nil {
```

The `ctx` here is the `OnStart` context from Fx, which has a short timeout (default 15s). If the router uses this context for its lifecycle, it will shut down after the start timeout expires. The router should use `context.Background()` or a long-lived context.

### M7. No Email Uniqueness Constraint Race Condition Handling

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/app/create_user.go` (lines 38-44)

The check-then-create pattern for email uniqueness has a TOCTOU race: two concurrent requests with the same email could both pass the check before either writes. The DB has a UNIQUE constraint that catches this, but the resulting Postgres error is not mapped to `domain.ErrEmailTaken` -- it surfaces as a generic "inserting user" error.

**Fix**: Catch the unique violation error code from Postgres:
```go
if err := h.repo.Create(ctx, user); err != nil {
    if isUniqueViolation(err) {
        return nil, domain.ErrEmailTaken
    }
    return nil, fmt.Errorf("creating user: %w", err)
}
```

### M8. `SoftDelete` Does Not Verify User Exists

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/modules/user/adapters/postgres/repository.go` (lines 137-140)

`SoftDeleteUser` executes an UPDATE but does not check if any rows were affected. Deleting a non-existent user succeeds silently.

```go
func (r *PgUserRepository) SoftDelete(ctx context.Context, id domain.UserID) error {
    q := sqlcgen.New(r.pool)
    // Should check result.RowsAffected() and return ErrNotFound if 0
    return q.SoftDeleteUser(ctx, uuid.MustParse(string(id)))
}
```

---

## Low Priority

### L1. `RequestID` Does Not Validate Incoming X-Request-ID

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/request_id.go`

An attacker can set `X-Request-ID` to any arbitrary string (including very long strings or strings with special characters that could pollute logs). Consider validating length and format.

### L2. Security Headers Include Deprecated X-XSS-Protection

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/security.go` (line 12)

`X-XSS-Protection: 1; mode=block` is deprecated and can introduce vulnerabilities in older browsers. Modern browsers have removed XSS auditors. Consider setting it to `0` instead, or removing it and relying on CSP.

### L3. No Content-Security-Policy Header

The security headers middleware does not set `Content-Security-Policy`. For an API service this is less critical, but the Swagger UI serves HTML and would benefit from a CSP.

### L4. Seed Users Have Predictable Passwords

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/cmd/seed/main.go` (lines 25-28)

Seed users have passwords like `Admin@123456`. This is acceptable for development but should be flagged/documented to never be used in production seeding.

### L5. `go 1.25.0` in go.mod

**File**: `/Users/namnguyen/Desktop/www/freelance/gnha-services/go.mod` (line 3)

The Go version is set to `1.25.0`. This is a future version. If this is intentional (forward-looking boilerplate), document it. Otherwise, use a released version.

---

## Positive Observations

1. **Correct JWT signing method validation** -- `ValidateAccessToken` checks `t.Method.(*jwt.SigningMethodHMAC)`, preventing algorithm confusion attacks. Well done.
2. **Argon2id with constant-time comparison** -- Password hashing uses proper parameters (64MB memory, 3 iterations) and `subtle.ConstantTimeCompare`. Textbook implementation.
3. **Clean hexagonal architecture** -- Domain has no infrastructure imports. Repository interface defined in domain. Adapters implement the interface. Dependency direction is correct throughout.
4. **Encapsulated domain entity** -- `User` struct uses unexported fields with getters and behavior methods (`ChangeName`, `ChangeRole`). `Reconstitute` pattern for hydration from persistence.
5. **Fx dependency injection** -- Clean module composition. Each module is self-contained with `fx.Module`.
6. **Error handling architecture** -- Centralized error handler with domain error codes mapped to both HTTP status codes and Connect RPC codes. Single source of truth.
7. **Cursor-based pagination** -- Correct pattern (fetch N+1, trim to N, set hasMore).
8. **Token blacklist via Redis** -- JWT revocation implemented in auth middleware.
9. **OTel trace propagation through events** -- Event bus injects trace context into message metadata.
10. **Request logging sanitizes sensitive headers** -- Though the sanitizer is unused (M5), the design intent is correct.
11. **Testcontainers integration** -- Real infrastructure in tests, no mocks for database/queue.
12. **SQL uses parameterized queries throughout** -- No SQL injection risk (sqlc-generated code).

---

## Recommended Actions (Priority Order)

1. **[Critical]** Enforce minimum JWT secret length (32+ chars) in config validation
2. **[Critical]** Replace `uuid.MustParse` with `uuid.Parse` + error return in repository layer
3. **[Critical]** Sanitize SMTP header inputs to prevent header injection
4. **[High]** Fix cursor pagination -- pass actual UUID for the ID parameter
5. **[High]** Add `SELECT FOR UPDATE` to the update transaction flow
6. **[High]** Log event publish errors instead of silently discarding them
7. **[High]** Add `ActorID` to domain events from auth context
8. **[High]** Add password length validation in CreateUserHandler
9. **[Medium]** Handle Postgres unique violation as ErrEmailTaken in Create
10. **[Medium]** Shut down TracerProvider and MeterProvider on Fx lifecycle stop
11. **[Medium]** Use `context.Background()` for Watermill router Run
12. **[Medium]** Check rows affected in SoftDelete
13. **[Low]** Remove dead code (BaseModel, sensitiveHeaders/SanitizeHeader)
14. **[Low]** Validate or limit X-Request-ID length

---

## Metrics

| Metric | Value |
|--------|-------|
| Go files (hand-written) | 48 |
| LOC (internal + cmd) | ~2,523 |
| Critical issues | 3 |
| High issues | 6 |
| Medium issues | 8 |
| Low issues | 5 |
| Test coverage | Testcontainers infra present, no test files found |
| Linting issues | Not run (no golangci-lint in path) |

---

## Architecture Diagram

```
cmd/server/main.go
    |
    +-- fx.New()
        |
        +-- shared.Module (config, postgres, redis, otel)
        +-- events.Module (watermill publisher/subscriber/router)
        +-- user.Module
        |   +-- domain/ (User, UserRepository port)
        |   +-- app/ (CreateUser, GetUser, ListUsers, UpdateUser, DeleteUser)
        |   +-- adapters/
        |       +-- postgres/ (PgUserRepository)
        |       +-- grpc/ (Connect RPC handler, mapper, routes)
        +-- audit.Module (event subscriber -> audit_logs)
        +-- notification.Module (event subscriber -> SMTP)
        +-- cron.Module (scheduler with Redis distributed lock)
```

Dependency flow: `grpc -> app -> domain <- postgres` (correct hexagonal direction)

---

## Unresolved Questions

1. **AuthService proto defined but no Go implementation exists** -- `auth.proto` defines Login/RefreshToken/Logout RPCs, but there are no corresponding handler files. Is this intentional (planned for next phase)?
2. **No test files were found** in the codebase. The testutil helpers exist, but no `*_test.go` files. Are tests planned separately?
3. **go 1.25.0** -- Is this intentional for a future Go version or a typo?
4. **Rate limit of 100 req/min globally** -- Is this the intended production value? It seems low for an API that serves multiple users.
