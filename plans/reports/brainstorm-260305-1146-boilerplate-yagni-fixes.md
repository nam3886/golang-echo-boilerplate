# Brainstorm: Boilerplate YAGNI Fixes

**Date:** 2026-03-05
**Sources:** code-reviewer-260305-0110, code-reviewer-260305-0141, 20-criteria scoring matrix
**Method:** YAGNI filter on 27 consolidated findings → 6 actionable items

---

## Problem Statement

Boilerplate has excellent architectural bones (7.7/10 avg) but fails "new dev just follows patterns" goal. Two audits found 27 items. Most are over-engineering for a boilerplate. Need minimum fixes to reach READY.

## YAGNI Filter Results

**27 items → 6 items.** 21 items rejected as over-engineering for boilerplate context.

### Items to Fix

| # | What | Size | Why |
|---|------|------|-----|
| A | Xóa auth half-impl (proto, tables, queries, apikey utils, BaseModel, empty cron refs) | Medium | Dead/half code confuses new devs |
| B | Fix CreateUser UUID — pass domain ID vào INSERT | Small | Bug: API returns wrong ID |
| C | Rewrite adding-a-module.md — match actual code patterns | Medium | Core boilerplate value; current doc causes compile errors |
| D | Add 1-2 example test files — domain unit + repo integration | Medium | New dev needs template to follow |
| E | Fix naming convention — pick PgXxxRepository, document | Small | Consistency for new devs |
| F | Setup mockgen — go generate directive in repository interface | Small | Agreed from prior brainstorm |

### Items Rejected (YAGNI)

- Password validation at domain level → proto validation `min_len=8` sufficient
- Health checks → dummy placeholder acceptable, infra-specific
- Fx OnStop hooks → OS reclaims on container death
- RBAC apply → middleware exists, dev applies per business logic
- X-Request-ID validation → unlikely attack vector
- OTel WithInsecure → dev env default, prod has network policy
- SELECT * password hash → mapper filters, not a security issue
- Swagger hardcoded → 1-line change when adding module
- Watermill context.Background → router.Close() handles it
- Audit DRY → explicit handlers clearer than generic
- Logger inject → slog.SetDefault works fine
- All LOW items (docker-compose.monitor, task dev:stop, ES unused, etc.)

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Auth module | REMOVE entirely | YAGNI — boilerplate shows user module pattern. Auth is app-specific |
| UUID strategy | Domain controls ID, pass to INSERT | Simpler than reading RETURNING row |
| Test approach | mockgen + 2-layer (unit + integration) | Unified pattern for new devs |
| Naming | PgXxxRepository convention | Matches existing code |

## Success Criteria

- `go build ./...` passes
- `go test ./...` passes with example tests
- New dev follows adding-a-module.md → zero compile errors
- No half-implemented features in codebase
- Naming conventions consistent across code + docs

## Unresolved Questions

None — all decisions made.
