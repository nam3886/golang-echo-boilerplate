---
status: in_progress
created: 2026-03-06
mode: fast
---

# Boilerplate Quick Wins - Fix All Review Issues

## Overview
Fix 21 issues identified by 6 parallel code review agents. Grouped into 4 phases by dependency and parallelizability.

## Phases

| Phase | Description | Items | Effort | Status |
|-------|------------|-------|--------|--------|
| 1 | [Security fixes](phase-01-security-fixes.md) | S1-S7 | 45min | pending |
| 2 | [Email + Infrastructure](phase-02-email-infra.md) | E1-E3, I1-I4 | 45min | pending |
| 3 | [Database + Architecture](phase-03-db-architecture.md) | D1-D3, A1-A3 | 30min | pending |
| 4 | [Events + Code Quality](phase-04-events-quality.md) | V1-V3, T1 | 30min | pending |

**Phase 1 + 2 can run in parallel. Phase 3 + 4 can run in parallel after.**

## Key Decisions
- Remove Traefik entirely
- Replace Mailhog with Mailpit
- Add SMTP auth for AWS SES production
- All fail-open patterns → fail-closed
