---
status: complete
created: 2026-03-05
completed: 2026-03-05
slug: boilerplate-yagni-fixes
source: brainstorm-260305-1146-boilerplate-yagni-fixes.md
---

# Boilerplate YAGNI Fixes

6 targeted fixes to make boilerplate production-ready for new devs.

## Context

- **Brainstorm report:** `plans/reports/brainstorm-260305-1146-boilerplate-yagni-fixes.md`
- **Audit reports:** `plans/reports/code-reviewer-260305-0110-full-boilerplate-review.md`, `plans/reports/code-reviewer-260305-0141-full-boilerplate-audit.md`
- **Principle:** YAGNI — 27 findings filtered to 6 actionable items

## Phases

| Phase | Description | Status | Priority | Effort |
|-------|-------------|--------|----------|--------|
| 01 | Remove auth half-impl + dead code | complete | CRITICAL | Medium |
| 02 | Fix CreateUser UUID mismatch | complete | CRITICAL | Small |
| 03 | Rewrite docs + example tests + mockgen | complete | HIGH | Medium |

## Dependencies

- Phase 02 depends on Phase 01 (sqlc regenerate after auth queries removed)
- Phase 03 independent (can parallel with 01+02)

## Success Criteria

- `go build ./...` passes
- `go vet ./...` passes
- `go test ./...` passes with example tests
- New dev follows adding-a-module.md → zero compile errors
- No half-implemented features in codebase
- Naming conventions consistent across code + docs
