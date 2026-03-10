# Review History - gnha-services

## Reports Index
| Date | Report | Score | Focus |
|------|--------|-------|-------|
| 2026-03-09 | `plans/reports/code-reviewer-260309-1135-user-module.md` | 8.5/10 | User module deep review |
| 2026-03-09 | `plans/reports/code-reviewer-260309-1254-architecture-dx.md` | 8.5/10 | Architecture & DX |
| 2026-03-09 | `plans/reports/review-260309-1246-code-quality-patterns.md` | N/A | Code quality (37 issues) |
| 2026-03-09 | `plans/reports/code-reviewer-260309-1254-docs-onboarding-dx.md` | 7/10 | Docs & onboarding |
| 2026-03-09 | `plans/reports/code-reviewer-260309-1403-testing-events-observability-cicd.md` | 8/10 | Testing/Events/CI |
| 2026-03-09 | `plans/reports/code-reviewer-260309-1403-scaffold-docs-dx-alignment.md` | 8/10 | Scaffold alignment |
| 2026-03-09 | `plans/reports/code-reviewer-260309-boilerplate-fixes.md` | 9/10 | Boilerplate fixes |
| 2026-03-09 | `plans/reports/review-260309-1512-adapter-layer.md` | 8.5/10 | Adapter layer (gRPC/Postgres/Search) |
| 2026-03-09 | `plans/reports/review-260309-1512-scaffold-templates.md` | 8/10 | Scaffold template deep review (27 tmpl) |
| 2026-03-09 | `plans/reports/code-reviewer-260309-1654-adapter-layer-deep-review.md` | 9/10 | Adapter deep review (Postgres+gRPC+Search) |

| 2026-03-09 | `plans/reports/code-reviewer-260309-1655-ci-config-docs-entrypoint.md` | 8/10 | CI/CD, Config, Docs, Entrypoint |
| 2026-03-09 | `plans/reports/code-reviewer-260309-1830-deep-review-fixes-round2.md` | 9/10 | Deep review fixes round 2 (54 files) |

| 2026-03-09 | `plans/reports/code-reviewer-260309-1847-domain-app-deep-review.md` | 9/10 | Domain & App deep review (round 22) |
| 2026-03-09 | `plans/reports/code-reviewer-260309-1847-ci-docs-config-deep-review.md` | 8.5/10 | CI/docs/config deep review (round 23) |
| 2026-03-09 | `plans/reports/code-reviewer-260309-1847-adapter-shared-deep-review.md` | 9/10 | Adapter & shared layer deep review (round 24) |

## Key Findings Per Review

### Adapter & Shared Layer Deep Review (9/10, round 24)
- Report: `plans/reports/code-reviewer-260309-1847-adapter-shared-deep-review.md`
- 0 critical, 0 high, 5 medium, 3 low
- Verified fixes: RETURNING * eliminated, SoftDelete TOCTOU, retry max cap, ErrNoChange in scaffold
- M-SCAFFOLD-3: Scaffold queries.tmpl still uses RETURNING * (user module uses explicit columns)
- M-SCAFFOLD-4: Scaffold adapter_postgres.tmpl Create doesn't hydrate entity with DB timestamps
- Redis CI fallback still missing t.Cleanup for client.Close()
- Error mapping chain verified complete (domain -> connect -> HTTP, all 9 codes)
- RBAC fail-closed verified, middleware ordering correct, cursor pagination robust
- Positives: clean handler pattern, consistent tx handling, config masking, test stubs

### CI/Docs/Config Deep Review (8.5/10, round 23)
- CRITICAL: C-CICD-1 deploy migration still broken (server has no migrate subcommand)
- HIGH: OTel envDefault="http://localhost:4317" makes no-op fallback unreachable (H-CONFIG-1)
- HIGH: mockgen @latest in CI vs @v0.6.0 in Taskfile (I-CICD-1)
- Doc mismatches: adding-a-module step 9 says manual but scaffold auto-injects; testing-strategy claims Cobertura; changelog says MailHog/19 files/wrong mockgen path
- Lefthook suppresses buf/sqlc stderr with 2>/dev/null
- Positives: build passes, all tests pass, good config validation, Dockerfile best practices, comprehensive docs



### Deep Review Fixes Round 2 (9/10, round 21)
- 25+ issues resolved across security, correctness, testing, infra, docs
- SoftDelete TOCTOU race eliminated (single UPDATE...RETURNING)
- ErrNoChange pattern added (shared sentinel, repo intercepts, app signals)
- JWT Subject claim, ErrorHandler echo-code mapping, exponential backoff
- OTel empty endpoint guard, recovery panic truncation, isPermanentSMTPError typed assertion
- RETURNING * columns replaced with explicit lists in user.sql + scaffold queries.tmpl
- Audit subscriber PII fix (msg_id instead of raw payload)
- New tests: audit (7), notification (5), update edge cases (3), delete edge case (1)
- UNFIXED: scaffold adapter_postgres.tmpl Update missing ErrNoChange handler (C-3)
- UNFIXED: notification subscriber still logs raw payload (M-NEW-4)
- Redis CI fallback missing t.Cleanup; exponential backoff has no max cap

### CI/CD, Config, Docs, Entrypoint (8/10, round 20)
- CRITICAL: Deploy migration cmd uses wrong binary path AND non-existent subcommand
- CRITICAL: Dockerfile healthcheck has no start-period (server retry budget = 60s, healthcheck window = 30s)
- mockgen version unpinned in CI vs pinned in Taskfile
- CLAUDE.md references deleted fixture functions
- testing-strategy.md falsely claims Cobertura format
- .env.example missing ELASTICSEARCH_INDEX_PREFIX
- adding-a-module.md still shows RETURNING * in SQL (contradicts prior fix)
- Dockerfile does not copy migrations to runtime stage
- Positives: non-root Docker user, seed idempotency, Config.String() masks secrets, readiness checks all deps


### User Module
- Missing tests: no-op mutations, empty-ID handlers, DeleteUser gRPC
- Delete event fixed to use DeletedAt
- App-layer limit default dead code (proto validation enforces gte:1)
- SoftDelete SQL no longer returns password (fixed c491b86)

### Architecture & DX
- Strong boilerplate, gaps are undocumented implicit patterns

### Code Quality & Patterns (5,218 LOC, 37 issues)
- 1 critical (RBAC read endpoints -- fixed), 6 high, 16 medium, 14 low
- Top DRY: actorID extraction copy-pasted (fixed with ActorIDFromContext)
- Scores: domain 9/10, app 8/10, repo 9/10, gRPC 9/10, middleware 8/10

### Docs & Onboarding (7/10)
- Event topic ownership contradiction in adding-a-module.md
- testify examples in code-standards.md (codebase uses stdlib)
- Scaffold-vs-user drift in permissions, validation order, event fields

### Testing/Events/CI (8/10)
- sqlc version mismatch: Taskfile v1.30.0 vs CI v1.28.0
- 4 missing test cases for Update/Delete edge cases
- Audit/notification subscribers at 0% coverage

### Scaffold Alignment (8/10)
- update_test.tmpl compile error (`&domain.X{}` cross-package)
- SoftDelete `:one RETURNING *` vs user `:exec`
- Docs actorID outdated in 3 locations

### Boilerplate Fixes (9/10)
- RBAC: all 5 procedures mapped, fail-closed, 10 tests
- Password removal from RETURNING clauses
- ActorIDFromContext DRY helper added

### Adapter Layer (8.5/10 -> 9/10 with fixes)
- Thin gRPC handler (correct), closure-based UoW, nil-safe search
- CreateUser discards RETURNING row (timestamp inconsistency)
- 5 near-identical domain mappers (sqlc tradeoff, documented)
- Update integration test missing
- Cursor pagination lacks HMAC/expiry (acceptable for internal API)

### Adapter Layer Deep Review (9/10, round 19)
- Report: `plans/reports/code-reviewer-260309-1654-adapter-layer-deep-review.md`
- 0 critical, 2 important, 8 medium, 5 minor
- I-1: Update COALESCE sends all fields even when unchanged (audit trigger risk)
- I-2: GetByEmail carries password hash in domain entity (latent leak risk)
- M-2: Create discards RETURNING row (Go vs DB timestamp mismatch)
- M-3/M-4: No integration tests for Update or GetByEmail
- M-1: Cursor decodeCursor accepts zero time/nil UUID without error
- Positives: password excluded from all read queries, fail-closed RBAC, graceful ES degradation

### Scaffold Templates Deep Review (8/10)
- CRITICAL: adapter_postgres.tmpl Update does NOT handle ErrNoChange (user module does)
- CRITICAL: queries.tmpl uses RETURNING * for Create/Update (leaks future sensitive cols)
- Scaffold has 1 generic toDomain mapper; user module has 5 per-query-type mappers
- domainErrorToConnect wrapper is unnecessary indirection (user calls connectutil directly)
- Event contract field order inconsistent within template itself
- No mapper_test template (user module has 115-line mapper_test.go)
- DX: one-command scaffold + auto-register + auto-RBAC is excellent
- Post-scaffold manual work underestimated (~10 files to customize)

## Resolved Issues
- C-4: OTLP WithEndpointURL (full URL with scheme)
- H-4: Metrics DeploymentEnvironmentName
- I-8: Email CRLF injection fixed
- I-9: SubscriberFactory per-handler queues
- I-10: CSP Swagger override
- I-11: No-op update event suppressed
- H-6: CapturingPublisher captures all messages
- H-8: RBAC read endpoints mapped
- H-2: Scaffold RBAC auto-injection
- H-3: Scaffold typed permission constants
- C-5/C-6/C-7: Scaffold test/integration/update templates fixed
- Password hash exposure removed from all RETURNING clauses
- Alpine 3.21, Redis appendonly, RabbitMQ non-management
- Router cancellable context, cron module removed
### Round 21 resolutions
- H-INFRA-3: Recovery panic truncation (200 chars)
- H-INFRA-4: JWT Subject claim added
- M-INFRA-2: ErrorHandler echo-code mapping (405, 413, timeout)
- M-INFRA-4: Exponential backoff (was linear)
- M-INFRA-5: Config String() builder pattern (was 22 positional args)
- M-INFRA-9: OTel empty endpoint returns no-op provider
- M-DOM-2: isPermanentSMTPError uses textproto.Error typed assertion
- I-DOM-1: SoftDelete uses sharederr.ErrNotFound() consistently
- I-DOM-3: Get/List handlers wrap repo errors with context
- M-DOM-1: 4 of 6 missing edge-case tests added (no-op, same-value, already-deleted, event-failure)
- M-NEW-3: SoftDelete TOCTOU race eliminated (single UPDATE...RETURNING)
- M-NEW-7: CI integration tests on MR (manual+allow_failure)
- M-NEW-8: CI generated-check includes mocks
- C-4/M-NEW-14: RETURNING * replaced with explicit columns (user.sql + scaffold)
- I-CICD-2: fixtures.go deleted, testing-strategy.md updated
- I-CICD-4: .env.example ELASTICSEARCH_INDEX_PREFIX added
- I-CICD-5/I-CICD-6: adding-a-module.md fixed (auto steps, RETURNING columns, providers)
- C-CICD-2: Dockerfile healthcheck start-period=60s retries=5
- M-CICD-1: Lefthook checks mock staleness
- M-CICD-2: CI generated-check runs on main too
- M-CICD-3: Dockerfile copies db/migrations
- M-NEW-1: Scaffold delete event uses DeletedAt with fallback
- M-NEW-2: code-standards.md actorID examples updated
- M-NEW-12/M-NEW-13: adding-a-module module.go + file count fixed
- H-1: otelhttp upgraded to v0.67.0
- H-NEW-1: sqlc version aligned (v1.30.0 in CI)
- M-11: Audit/notification subscriber tests added (0% -> covered)
- contextKey renamed to middlewareContextKey (naming clarity)
- rbac.md scaffold auto-injection note added
- domainErrorToConnect wrapper removed (direct connectutil calls)
