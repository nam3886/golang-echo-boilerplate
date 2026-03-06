---
phase: 2
priority: high
status: pending
---

# Phase 2: Email + Infrastructure

## Items

### E1: Add SMTP auth support
- File: `internal/modules/notification/email.go`
- Change: Use `smtp.PlainAuth` when SMTPUser/SMTPPassword are set, `nil` auth when empty (dev/Mailpit)
- Pattern: `if cfg.SMTPUser != "" { auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost) }`

### E2: Add SMTP config fields
- File: `internal/shared/config/config.go`
- Add: `SMTPUser`, `SMTPPassword`, `SMTPFromAlias` fields
- Update `.env.example` with new SMTP vars

### E3: Replace Mailhog with Mailpit
- File: `deploy/docker-compose.dev.yml`
- Change: Replace `mailhog/mailhog:latest` with `axllent/mailpit:latest`
- Ports: `1025:1025` (SMTP), `8025:8025` (UI) -- same ports, drop-in replacement

### I1: Remove Traefik
- Files: `deploy/docker-compose.yml`, `deploy/traefik/traefik.yml`
- Change: Remove traefik service, traefik labels from app, traefik network, traefik-certs volume
- App service: add `ports: ["${PORT:-8080}:8080"]`
- Delete `deploy/traefik/` directory

### I2: Fix CI coverage artifact
- File: `.gitlab-ci.yml`
- Change: Match artifact path to actual output format (`coverage.out` not `coverage.xml`)

### I3: Remove monitor compose reference or create stub
- File: `Taskfile.yml`
- Change: Remove `monitor:up` task or add comment that it's optional/TODO

### I4: Add OTel Echo middleware
- File: `internal/shared/middleware/chain.go`
- Change: Add `otelecho.Middleware()` to middleware chain
- Import: `go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho`

## Success Criteria
- Email works with Mailpit (dev) and AWS SES (prod)
- No Traefik references in docker-compose.yml
- CI artifacts match actual output
- HTTP request spans appear in traces
