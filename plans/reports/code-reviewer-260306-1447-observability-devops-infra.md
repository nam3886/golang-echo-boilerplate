# Observability, DevOps & Infrastructure Review

**Date:** 2026-03-06 | **Reviewer:** code-reviewer | **Scope:** Observability + Infrastructure

---

## Scope

- **Files reviewed:** 16 files across observability/, middleware/, deploy/, CI/CD, task runner, linting
- **Focus:** Logging, tracing, metrics, Docker, Docker Compose, CI/CD, Traefik, hot reload, linting

## Overall Assessment

Solid infrastructure for a boilerplate. Structured logging with slog, OTel tracing+metrics with conditional TLS, multi-stage Docker build, proper CI/CD pipeline with 4 stages, Traefik TLS termination, and a comprehensive Taskfile. A few gaps below.

---

## Critical Issues

None.

---

## High Priority

### H-1: Docker image runs as root

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/Dockerfile` (lines 12-19)

The runtime stage has no `USER` directive. Container runs as root by default.

**Fix:**
```dockerfile
# Stage 2: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata curl && \
    addgroup -S app && adduser -S app -G app
COPY --from=builder /server /server
RUN chown app:app /server
USER app
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
    CMD curl -f http://localhost:8080/healthz || exit 1
ENTRYPOINT ["/server"]
```

### H-2: CI coverage report artifact mismatch

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/.gitlab-ci.yml` (lines 53-56)

The `unit-test` job produces `coverage.out` (Go format) but declares a Cobertura artifact at `coverage.xml`. That XML file is never generated, so GitLab coverage visualization will silently fail.

**Fix:** Either remove the `coverage_report` block or add a conversion step:
```yaml
script:
  - go test -race -count=1 -coverprofile=coverage.out ./internal/...
  - go tool cover -func=coverage.out
  - go install github.com/boumenot/gocover-cobertura@latest
  - gocover-cobertura < coverage.out > coverage.xml
```

### H-3: Monitoring compose file missing

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/Taskfile.yml` (lines 154-161)

Tasks `monitor:up` and `monitor:down` reference `deploy/docker-compose.monitor.yml` but this file does not exist. Running `task monitor:up` will fail.

**Impact:** Developers cannot start the monitoring stack. OTel exporters are configured to send to `localhost:4317` but no collector is running.

**Fix:** Create `deploy/docker-compose.monitor.yml` with an OTel Collector + backend (e.g., Jaeger, SigNoz), or remove the dead tasks and document how to run a collector manually.

### H-4: Traefik config uses env var interpolation that Traefik does not support

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/deploy/traefik/traefik.yml` (line 14)

`${ACME_EMAIL}` in a YAML static config file -- Traefik does not natively interpolate environment variables in its YAML config. This will be treated as a literal string.

**Fix:** Use `--certificatesresolvers.letsencrypt.acme.email=$ACME_EMAIL` as a command-line arg in docker-compose, or use Traefik's file provider with `envsubst` entrypoint.

```yaml
# deploy/docker-compose.yml - traefik service
command:
  - "--certificatesresolvers.letsencrypt.acme.email=${ACME_EMAIL}"
```

---

## Medium Priority

### M-1: Logger does not attach service-level attributes

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/observability/logger.go`

Production JSON logs lack `service.name` and `service.version` fields. This makes log aggregation across multiple services harder.

**Fix:** Add default attributes to the logger:
```go
logger := slog.New(handler).With(
    "service", cfg.AppName,
    "env", cfg.AppEnv,
)
```

### M-2: Metrics resource missing DeploymentEnvironment

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/observability/metrics.go` (line 33-36)

TracerProvider sets `DeploymentEnvironment` but MeterProvider omits it. Inconsistent resource attributes between traces and metrics.

**Fix:** Add `semconv.DeploymentEnvironment(cfg.AppEnv)` to the meter resource.

### M-3: Service version hardcoded to "0.1.0"

**Files:** `tracer.go:36`, `metrics.go:35`

Version is hardcoded in both providers. Should be injected via build flags or config.

**Fix:** Add `Version` to config or use `-ldflags`:
```go
var Version = "dev" // set via -ldflags="-X main.Version=..."
```

### M-4: No OTel trace middleware on Echo routes

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/internal/shared/middleware/chain.go`

The middleware chain does not include `otelecho.Middleware()` (from `go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho`). Traces are created only in the event bus, not for HTTP requests. The request logger extracts `trace_id` from context (line 41 of request_log.go) but no middleware creates spans, so `trace_id` will always be empty for HTTP-only requests.

**Fix:**
```go
import "go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"

// Add after RequestID middleware:
e.Use(otelecho.Middleware(cfg.AppName))
```

### M-5: Docker Compose prod exposes no ports on Postgres/Redis/RabbitMQ but no network segmentation docs

The production compose correctly does not expose infrastructure ports externally (good). However, the `backend` network is a flat bridge -- all services can reach all other services. Consider documenting that in a hardened deployment, each dependency should have its own network segment.

### M-6: CI deploy uses SSH with no rollback mechanism

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/.gitlab-ci.yml` (lines 111-154)

Deploy stages do `docker compose up -d --no-deps app` with no health check verification or rollback on failure. If the new image crashes, the service stays down.

**Fix:** Add post-deploy health check:
```yaml
script:
  - |
    ssh $STAGING_USER@$STAGING_HOST "
      cd $STAGING_DIR &&
      IMAGE_TAG=${CI_COMMIT_SHORT_SHA} docker compose ... up -d --no-deps app &&
      sleep 5 &&
      curl -sf http://localhost:8080/healthz || (
        IMAGE_TAG=previous docker compose ... up -d --no-deps app && exit 1
      )
    "
```

### M-7: Dev compose Redis has no password

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/deploy/docker-compose.dev.yml` (line 19-30)

Dev Redis runs with no `--requirepass`. The app config expects `REDIS_URL` which may or may not include auth. Prod compose uses `--requirepass ${REDIS_PASSWORD}`. This divergence can hide auth-related bugs.

---

## Low Priority

### L-1: Air watches `.proto` and `.sql` files but cannot rebuild protos

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/.air.toml` (line 13)

`include_ext = ["go", "sql", "proto"]` triggers rebuild on proto/sql changes, but `cmd` only runs `go build`. Changed proto files will trigger a useless rebuild without code generation.

**Fix:** Either remove `proto` and `sql` from `include_ext` or change `cmd` to `task generate && go build ...`.

### L-2: Lefthook generated check suppresses errors

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/.lefthook.yml` (lines 10-11)

`buf generate 2>/dev/null` and `sqlc generate 2>/dev/null` swallow errors. If these tools are not installed, the hook silently passes.

### L-3: golangci-lint version not pinned in CI

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/.gitlab-ci.yml` (line 22)

`image: golangci/golangci-lint:latest` -- different pipeline runs can use different linter versions, causing inconsistent results. Pin to a specific version.

### L-4: Elasticsearch security disabled in dev

**File:** `/Users/namnguyen/Desktop/www/freelance/gnha-services/deploy/docker-compose.dev.yml` (line 56)

`xpack.security.enabled=false` -- acceptable for dev, but worth a comment noting this should never be in prod.

### L-5: Traefik Docker socket mounted read-only (good) but no TLS on socket

The Docker socket `/var/run/docker.sock` is mounted `:ro` which is good. For hardened setups, consider using a Docker socket proxy (e.g., `tecnativa/docker-socket-proxy`).

---

## Positive Observations

1. **Structured logging done right** -- slog with JSON in prod, text in dev, log levels configurable, trace_id + request_id + user_id in request logs
2. **OTel conditional TLS** -- `WithInsecure()` only in development, production requires TLS
3. **Multi-stage Docker build** -- small final image with `alpine:3.19`, stripped binary with `-ldflags="-s -w"`
4. **Docker HEALTHCHECK** -- built into Dockerfile
5. **Proper service dependencies** -- `depends_on` with `condition: service_healthy` in prod compose
6. **Comprehensive Taskfile** -- dev:setup, generate, lint, test, migrate, seed, module:create all well-organized
7. **Lefthook pre-commit/pre-push** -- lint on commit, test on push, generated code staleness check
8. **Traefik TLS with Let's Encrypt** -- auto-cert with HTTP challenge, HTTP-to-HTTPS redirect
9. **Recovery middleware** -- catches panics with stack traces
10. **Event bus trace propagation** -- OTel context injected into Watermill message metadata
11. **Graceful shutdown** -- Fx lifecycle hooks for OTel, DB, Redis, Echo server
12. **Rate limiting with Redis** -- 100 req/min global rate limit
13. **Good linter selection** -- gocritic, revive, staticcheck, errcheck -- good coverage without being overly strict

---

## Recommended Actions (Priority Order)

1. **[H-1]** Add non-root `USER` to Dockerfile
2. **[H-2]** Fix coverage.xml generation or remove Cobertura artifact declaration
3. **[H-4]** Fix Traefik env var interpolation (use CLI args or envsubst)
4. **[H-3]** Create monitoring compose file or remove dead tasks
5. **[M-4]** Add `otelecho.Middleware()` to enable HTTP request tracing
6. **[M-2]** Add `DeploymentEnvironment` to MeterProvider resource
7. **[M-1]** Add service-level attributes to slog logger
8. **[M-6]** Add post-deploy health check and rollback in CI
9. **[L-3]** Pin golangci-lint version in CI

---

## Metrics

| Metric | Value |
|--------|-------|
| Observability Coverage | 70% -- tracing setup exists but not wired to HTTP |
| Docker Security | 7/10 -- multi-stage, healthcheck, but runs as root |
| CI/CD Completeness | 8/10 -- 4 stages, good structure, missing rollback |
| Dev Workflow | 9/10 -- excellent Taskfile, hot reload, lefthook |
| Linting Config | 9/10 -- solid selection, gen dirs excluded |

---

## Unresolved Questions

1. Is the monitoring stack (SigNoz/Jaeger) intended to be self-hosted or cloud-managed? The missing compose file suggests this is deferred.
2. Should the CI pipeline include a SAST/DAST scanning stage? Currently only lint + test.
3. Is database migration intended to run as part of deploy or separately? Currently only available via `task migrate:up` locally.
