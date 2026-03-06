---
status: completed
created: 2026-03-04
completed: 2026-03-04
type: boilerplate
stack: go-1.26, echo, connect-rpc, watermill, sqlc, uber-fx, postgresql, redis, rabbitmq, elasticsearch, signoz
---

# Go API Boilerplate — Implementation Plan

Production-ready modular monolith boilerplate with Go 1.26.

## Context

- [Brainstorm Report](../reports/brainstorm-260304-1636-golang-api-boilerplate.md)
- [Framework Research](../reports/researcher-260304-1217-golang-boilerplate-research.md)
- [Architecture Patterns](../reports/researcher-260304-1437-golang-architecture-patterns.md)

## Phases Overview

| # | Phase | Priority | Effort | Status | Progress |
|---|-------|----------|--------|--------|----------|
| 1 | [Project Foundation](phase-01-project-foundation.md) | P0 | M | completed | 100% |
| 2 | [Shared Infrastructure](phase-02-shared-infrastructure.md) | P0 | L | completed | 100% |
| 3 | [Code Gen Pipeline](phase-03-code-gen-pipeline.md) | P0 | M | completed | 100% |
| 4 | [Auth & Security](phase-04-auth-security.md) | P0 | L | completed | 100% |
| 5 | [Example Module](phase-05-example-module.md) | P0 | L | completed | 100% |
| 6 | [Events & CQRS](phase-06-events-cqrs.md) | P1 | L | completed | 100% |
| 7 | [DevOps & Testing](phase-07-devops-testing.md) | P1 | L | completed | 100% |
| 8 | [Docs & DX Polish](phase-08-docs-dx-polish.md) | P1 | M | completed | 100% |

**Effort:** S (<2h), M (2-4h), L (4-8h)

## Dependencies

```
Phase 1 ──→ Phase 2 ──→ Phase 3 ──→ Phase 4 ──→ Phase 5
                                        │            │
                                        └────────────┼──→ Phase 6
                                                     │
                                                     └──→ Phase 7 ──→ Phase 8
```

## Key Decisions

1. **Go 1.26** — Green Tea GC, `errors.AsType`, `slog.NewMultiHandler`
2. **Protobuf-first** — `.proto` → buf gen → Go + OpenAPI + TS
3. **SigNoz** — replaces Prometheus+Jaeger+Loki+Grafana+AlertManager
4. **Simplified Hexagonal** — domain/app/adapters per module
5. **Uber Fx** — lifecycle hooks, module = bounded context
