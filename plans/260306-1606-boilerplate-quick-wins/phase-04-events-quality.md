---
phase: 4
priority: medium
status: pending
---

# Phase 4: Events + Code Quality

## Items

### V1: Add dead-letter exchange config
- File: `internal/shared/events/bus.go`
- Change: Configure AMQP DLX (dead-letter exchange) for poison messages
- After Watermill retries exhausted, route to `{topic}.dlq` queue

### V2: Handle json.Marshal errors in audit subscriber
- File: `internal/modules/audit/subscriber.go`
- Change: Return error from handler when `json.Marshal` fails (currently silently discarded at lines 65, 92, 119)

### V3: Deduplicate audit subscriber handlers
- File: `internal/modules/audit/subscriber.go`
- Change: Extract generic `handleAuditEvent(msg *message.Message, action string) error`
- Reduce 3x ~30-line handlers to 1x generic handler + 3 one-liners

### T1: Add missing create_user error-path tests
- File: `internal/modules/user/app/create_user_test.go`
- Add tests: invalid role, hasher failure, repo failure, event publish verification

## Success Criteria
- Poison messages routed to DLQ instead of infinite requeue
- No silently discarded errors
- Audit subscriber code reduced by ~60 lines
- create_user test coverage covers error paths
