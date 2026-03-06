---
phase: 1
priority: critical
status: completed
---

# Phase 1: Security Fixes

## Items

### S1: Rate limiter fail-closed when Redis down
- File: `internal/shared/middleware/rate_limit.go`
- Change: When Redis unavailable, return 503 Service Unavailable instead of allowing through
- Return `connect.NewError(connect.CodeUnavailable, ...)` on Redis error

### S2: SELECT explicit columns (exclude password_hash)
- File: `db/queries/user.sql`
- Change: Replace `SELECT *` in GetByID and ListUsers with explicit column list excluding `password_hash`
- Keep `SELECT *` only in GetByEmail (needed for auth)
- Re-run `sqlc generate` after

### S3: Mapper error sanitization
- File: `internal/modules/user/adapters/grpc/mapper.go`
- Change: In `domainErrorToConnect`, log raw error, return generic "internal error" to client
- `connect.NewError(connect.CodeInternal, fmt.Errorf("internal error"))`

### S4: Max password length
- File: `internal/shared/auth/password.go`
- Change: Add max length check (72 bytes, bcrypt/argon2id limit)
- Return domain error if exceeded

### S5: Token blacklist fail-closed
- File: `internal/shared/auth/jwt.go` (or wherever blacklist check is)
- Change: When Redis error on blacklist check, reject token (fail-closed)

### S6: JWT add iss/aud claims
- File: `internal/shared/auth/jwt.go`
- Change: Add `iss` (app name from config) and `aud` (service name) claims
- Validate on parse

### S7: Swap interceptor order (RBAC before validate)
- File: `internal/modules/user/adapters/grpc/routes.go`
- Change: Move RBACInterceptor before protovalidate interceptor

## Success Criteria
- All security patterns fail-closed
- No password hash in list/get responses
- No raw errors leaked to clients
- `go build ./...` passes
