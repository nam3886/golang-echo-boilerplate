# Brainstorm: Boilerplate Fix Scope

**Date:** 2026-03-05
**Context:** Post code-review of Go modular monolith boilerplate (production seed)

## Problem
Full codebase review found 3 CRITICAL, 11 IMPORTANT, 11 MINOR issues. Need to determine what's in-scope for a production seed boilerplate vs feature work.

## Decision: Production Seed Boilerplate
Boilerplate will be cloned and deployed to staging/prod after adding business logic. Security + correctness bugs must be fixed. Missing features (AuthService, RBAC, tests) are out of scope — they depend on specific business requirements.

## Agreed Fix Scope (6 items)

### MUST FIX — Correctness Bugs (3)
1. **Pagination cursor skips 1 row** — nextCursor built from probe row, app truncates but keeps wrong cursor
2. **SoftDelete succeeds for non-existent users** — `:exec` doesn't check affected rows, should return ErrNotFound
3. **Email uniqueness TOCTOU race** — Need to map Postgres unique violation to domain error

### SHOULD FIX — Pattern Consistency (3)
4. **UpdateUser/DeleteUser don't publish events** — Create publishes, Update/Delete should too for consistency
5. **Audit ActorID = EntityID** — Should extract actual actor from JWT context
6. **buf.validate not enforced** — Annotations exist but no protovalidate interceptor wired

### OUT OF SCOPE (boilerplate)
- AuthService implementation (middleware + JWT infra exists, handlers are feature work)
- RBAC on routes (depends on business rules)
- Unit tests (testcontainers infra exists, tests follow features)
- Elasticsearch config (unused but harmless)
- Minor naming/style issues

## Next Steps
Create implementation plan → fix all 6 items → re-review → commit.
