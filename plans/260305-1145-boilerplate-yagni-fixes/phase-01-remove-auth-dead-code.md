---
phase: 01
status: complete
completed: 2026-03-05
priority: CRITICAL
effort: Medium
---

# Phase 01: Remove Auth Half-Implementation + Dead Code

## Overview

Remove all half-implemented auth service code (proto, tables, queries, generated code) and dead code (BaseModel, apikey utils). Keep actively-used auth utilities (jwt.go, password.go, context.go).

## Context

- Brainstorm decision: Auth service is YAGNI for boilerplate. Half-impl confuses new devs.
- jwt.go used by: `middleware/auth.go` (ValidateAccessToken)
- context.go used by: 3 app handlers + 3 middleware (UserFromContext)
- password.go used by: CreateUserHandler (argon2id hashing)
- apikey.go: zero usage anywhere

## Files to DELETE

| File/Directory | Reason |
|----------------|--------|
| `proto/auth/v1/` (entire dir) | AuthService proto — no Go handler exists |
| `gen/proto/auth/v1/` (entire dir) | Generated from unused proto |
| `db/migrations/00002_auth_tables.sql` | refresh_tokens + api_keys tables — unused |
| `db/queries/auth.sql` | 8 queries for auth — unused |
| `gen/sqlc/auth.sql.go` | Generated from unused queries |
| `internal/shared/auth/apikey.go` | GenerateAPIKey/HashAPIKey — zero callers |
| `internal/shared/model/base.go` | BaseModel — zero imports in entire codebase |

## Files to KEEP (confirmed in-use)

| File | Used by |
|------|---------|
| `internal/shared/auth/jwt.go` | middleware/auth.go:22 |
| `internal/shared/auth/password.go` | user/app/create_user.go |
| `internal/shared/auth/context.go` | 6 call sites (3 app handlers + 3 middleware) |

## Implementation Steps

1. Delete files/directories listed above
2. Remove `proto/auth/v1/auth.proto` from `buf.yaml` if referenced
3. Run `sqlc generate` to regenerate without auth queries
4. Run `go build ./...` to verify no broken imports
5. Run `go vet ./...` for correctness
6. Check `gen/sqlc/querier.go` or `models.go` — ensure auth models removed

## Post-Step: Update buf.yaml if needed

Check if `buf.yaml` or `buf.gen.yaml` explicitly lists auth proto path. If so, remove the reference.

## Post-Step: Migration consideration

The `00002_auth_tables.sql` deletion means if anyone has already migrated, those tables stay in DB. This is acceptable for a boilerplate — fresh clones won't have them.

## Todo

- [x] Delete 7 files/dirs (proto/auth/v1, gen/proto/auth/v1, 00002_auth_tables.sql, auth.sql, auth.sql.go, apikey.go, base.go)
- [x] Remove buf.yaml auth reference if exists (verified not present)
- [x] sqlc generate (completed, no auth queries in output)
- [x] go build ./... passes (verified)
- [x] go vet ./... passes (verified)

## Risk

- **LOW**: jwt.go/context.go accidentally deleted → breaks 6+ files. Mitigated by explicit KEEP list above.
