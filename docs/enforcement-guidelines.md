# Enforcement Guidelines

Concrete implementation rules rút ra từ `docs/review-criteria.md` debate.

> **Legend:** 🔴 HARD RULE — reject PR nếu vi phạm | 🟡 GUIDELINE — apply with judgment

---

## Correctness

### C1 — Sentinel Error Construction 🔴

Named constructor functions, không dùng `var` sentinels.

```go
// ✅
func ErrUserNotFound() error { return fmt.Errorf("%w", sharederr.ErrNotFound()) }

// ❌ false negative với errors.Is()
var ErrUserNotFound = sharederr.ErrNotFound()
```

### C2 — Startup Failure Escalation 🔴

| Tình huống | Hành động |
|-----------|-----------|
| DB/Redis unavailable | `log.Fatal()` |
| Config invalid | `log.Fatal()` với clear message |
| Optional component fail | Log error + nil provider (degraded mode) |
| Graceful shutdown | Signal handler → drain → hard kill timeout |

### C3 — Nil Receiver Guard 🔴

Required deps non-nil tại construction. Optional deps dùng config flag — không nil checks tản mát.

```go
// ✅
type UserApp struct {
    repo   UserRepository // required — constructor panics if nil
    search UserSearch     // optional — nil if config.ElasticsearchURL == ""
}

// ❌
var repo UserRepository // nil accepted? undefined behavior
```

### C4 — Idempotent Event Handlers 🔴

Mọi event subscriber phải idempotent. Chọn 1 trong 3:
- **Dedup key** — idempotency key trong event, skip nếu đã xử lý
- **Upsert semantics** — DB write an toàn khi retry
- **Audit-based** — log mọi execution, detect duplicate post-facto

---

## Architecture

### A1 — Repository Interface Contract 🔴

Mọi repository interface PHẢI có contract comment:

```go
// ✅
// UserRepository — contracts:
//   - All methods retryable (idempotent)
//   - CreateUser: returns ErrAlreadyExists if email duplicate
//   - GetUser: returns ErrNotFound (not nil) if missing
//   - UpdateUser: TOCTOU handled via DB constraint
type UserRepository interface { ... }
```

### A2 — Required vs Optional Dependency Classification 🔴

Phân loại tại module creation, ghi trong struct comment.

```go
// ✅ UserApp dependencies:
//   Required: repo, auth (startup fails if nil)
//   Optional: search (nil if ElasticsearchURL empty, fallback to repo.GetAll)
type UserApp struct {
    repo   UserRepository
    auth   AuthService
    search UserSearch // optional
}
```

### A3 — Event Schema Versioning 🟡

Proto events phải có `version` field. Breaking changes: add field → deploy subscribers → remove old field.

### A4 — Per-Module Config Injection 🟡

Config injected dưới dạng module-scoped struct — không global singleton.

```go
// ✅
type UserConfig struct { DatabaseURL, JWTSecret string }
func NewUserModule(cfg UserConfig) *fx.Module { ... }

// ❌
var globalConfig = config.Get()
```

---

## DX

### DX1 — Pattern Showcase Completeness 🔴

Scaffold và example module PHẢI cover:

| Pattern | Required coverage |
|---------|------------------|
| Create | validation + constraint error + event publish |
| Read | success + not-found |
| Update | conflict detection + event |
| Delete | cascade behavior |

**Metric:** New joiner implement CRUD handler < 30 phút bằng copy-paste + adapt.

### DX2 — Single Source of Truth — Module-Scoped 🟡

Same problem → same solution **trong 1 module**. Cross-module differences OK nếu documented trong module README.

```bash
# Verify: không được có 2 styles trong cùng module
grep -r "ErrEmailTaken()" internal/modules/user/
grep -r 'sharederr.New.*email' internal/modules/user/
```

### DX3 — Footgun Documentation 🟡

**Footgun = API contract violation HOẶC performance cliff.**

- Phải có `⚠️` trong godoc hoặc module README
- Không được chỉ nằm trong test file comments

```go
// ✅
// GetAll retrieves all users.
// ⚠️ Performance: O(n) full scan — do not call in hot paths.
// Use Search() with pagination for user-facing queries.
func (r *UserRepo) GetAll(ctx context.Context) ([]*User, error)
```

---

## Consistency

### CONS1 — Error Handler Style Fixed per Module 🔴

Chọn 1 style per module — không mix:
- **Domain module** → Named constructors (`domain.ErrXxx()`)
- **Adapter/transport** → Inline factory OK (`sharederr.New(...)`)

```bash
# Verify: domain layer không dùng sharederr.New
grep -r "sharederr\.New" internal/modules/user/domain/ # phải empty
```

### CONS2 — Logging Context Propagation Mandatory 🔴

`slog.XxxContext(ctx, ...)` everywhere. `slog.Info` (non-context) chỉ cho lifecycle logs.

```bash
# Verify: nên gần như empty
grep -rn "slog\.Info(" internal/modules/
```

### CONS3 — Constraint Checking Parity 🟡

Create + Update validate cùng constraints. Ngoại lệ phải có ADR.

```go
// ✅ Test both with same invalid input
func TestCreateUser_InvalidEmail(t *testing.T) { ... }
func TestUpdateUser_InvalidEmail(t *testing.T) { ... } // same error expected
```

### CONS4 — Failure Mode Declaration 🔴

Mỗi external dep PHẢI declare trong module README:

```markdown
## Failure Modes
| Dependency | Mode | Behavior |
|------------|------|----------|
| Redis (session blacklist) | fail-closed | Reject auth request if unreachable |
| Redis (cache) | fail-open | Continue without cache |
| Elasticsearch | fail-open | CRUD works; search returns empty |
```

---

## Security

### S1 — Input Validation Boundary 🔴

Validation tại **domain constructors** — không scatter trong handlers.

```go
// ✅
func NewUser(email, name string) (*User, error) {
    if !isValidEmail(email) {
        return nil, fmt.Errorf("%w: invalid email", sharederr.ErrInvalidArgument())
    }
    return &User{email: email, name: name}, nil
}

// ❌ validation trong handler
func (a *UserApp) CreateUser(ctx context.Context, req *proto.CreateUserRequest) error {
    if req.Email == "" { ... } // repeated across handlers
}
```

### S2 — Auth Blacklist Fail Strategy 🔴

Phải explicit trong config:

```go
type AuthConfig struct {
    BlacklistFailOpen bool          // false = fail-closed (default, safer)
    BlacklistTimeout  time.Duration // timeout trước khi apply fail strategy
    BlacklistCacheTTL time.Duration // local cache nếu fail-open
}
```

Default: **fail-closed**. Fail-open chỉ dùng nếu HA quan trọng hơn security + có local cache.

### S3 — Audit Trail Completeness 🟡

Sensitive operations (login, password change, role change, delete) log: **Who + What + When + Status**.

```go
// ✅
slog.InfoContext(ctx, "password_changed",
    slog.String("user_id", userID),
    slog.String("result", "success"),
    slog.String("ip", getClientIP(ctx)),
)
```

### S4 — HTTPS + CORS Boundary 🔴

```go
// ✅
e.Use(middleware.HTTPSRedirect())
e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
    AllowOrigins:     []string{"https://app.example.com"}, // không dùng "*"
    AllowCredentials: true,
}))
```

### S5 — Rate Limiting Specification 🟡

Config phải define scope + algorithm + distributed:

```go
type RateLimitConfig struct {
    Scope       string        // "per-ip" | "per-user" | "per-api-key"
    Algorithm   string        // "sliding-window" | "token-bucket"
    Limit       int
    Window      time.Duration
    Distributed bool          // true = Redis-backed; false = in-memory (bypass risk)
}
```

### S6 — Secret Rotation Protocol 🟡

Runbook tại `docs/runbooks/` cho JWT + DB password. Pattern:
1. Deploy dual-validation (accept old + new)
2. Wait drain period (JWT TTL hoặc connection timeout)
3. Remove old secret

---

## Observability

### OBS1 — Error Logging Single Point 🔴

Log error **exactly once** tại catch boundary. Rule per layer:
- **App layer**: log domain errors (error_code + operation)
- **Adapter layer**: log infra errors (timeout, retry_count)
- **Handler/middleware**: log chỉ unhandled (panic, 5xx)

### OBS2 — Structured Logging Standard Keys 🔴

| Key | Type | Example |
|-----|------|---------|
| `user_id` | int64 | `123` |
| `module` | string | `"user"` |
| `operation` | string | `"CreateUser"` |
| `error_code` | string | `"ALREADY_EXISTS"` |
| `duration_ms` | int | `45` |
| `retry_count` | int | `2` |

**PII rule:** Log `user_id` (ID only) — không log email, phone, password, token.

### OBS3 — Trace Propagation 🔴

Mọi public function nhận `context.Context` làm param đầu tiên. Dùng `slog.XxxContext` để propagate trace_id.

```bash
# Verify: high coverage expected
grep -c "context.Context" internal/modules/user/app/
```

### OBS4 — Sampling Configuration 🔴

| Environment | Sampling |
|-------------|----------|
| dev / test | `AlwaysSample` |
| staging | 10% |
| production | 1% (via env var `OTEL_SAMPLING_RATIO`) |

`AlwaysSample` hardcode trong production code = HARD REJECT.

### OBS5 — Minimum Viable Error Log 🟡

Mỗi error log phải đủ để answer: "What went wrong? Why? Retryable?"

```go
// ✅
slog.ErrorContext(ctx, "failed to create user",
    slog.String("error_code", "ALREADY_EXISTS"),
    slog.String("operation", "CreateUser"),
    slog.Int64("user_id", req.UserID),
    slog.Bool("retryable", false),
)
```

---

## Testing

### T1 — Testing Pyramid Layer Strategy 🔴

| Layer | Test type | Infra | Speed |
|-------|-----------|-------|-------|
| Domain logic, app handlers | Unit + mock repos | gomock | Fast (<100ms) |
| Repository adapters (sqlc) | Integration | testcontainers | Slow (OK) |
| Event subscribers | Integration | testcontainers + Watermill | Slow (OK) |
| Full HTTP flow | 1 integration per module | testcontainers | Slow (OK) |

### T2 — Coverage Breadth Risk-Based 🟡

| Risk | Scenarios |
|------|-----------|
| Critical (auth, payments, data integrity) | success + not-found + validation + repo error + event failure + concurrency |
| Standard (CRUD handlers) | success + not-found + validation + 1 error path |
| Utility (helpers, formatters) | happy path + boundary |

### T3 — Integration Test Selectivity 🟡

testcontainers dùng cho **adapter contract** only — không cho app logic.

```
✅ user/adapters/postgres/user_repo_test.go  → testcontainers
✅ user/app/create_user_handler_test.go      → gomock (fast)
❌ user/app/create_user_handler_test.go      → testcontainers (overkill)
```

---

## YAGNI / KISS / DRY

### Y1 — Abstraction Maturity Gate 🟡

Abstract khi đáp ứng CẢ HAI:
1. **2+ actual use cases** (không phải theoretical)
2. **Stable** — pattern không thay đổi trong 2+ tuần

Không abstract khi: single use case, high volatility, syntactic convenience only.

### Y2 — DRY Exception Framework 🟡

Duplication **acceptable** khi:
- Generated code (sqlc, proto, mocks)
- Same structure, different domain semantics (`User.Create` ≠ `Post.Create`)
- Architectural boilerplate (handler scaffolds, domain error constructors per module)
- Cost of abstraction > cost of duplication

**Trước khi abstract:** Stable? Callers understand? Doesn't increase coupling?

### Y3 — Consistency Zones 🟡

Consistency áp dụng theo zone — không phải toàn bộ codebase:
- **Public/shared patterns** (repo interfaces, all handlers trong 1 module) → bắt buộc nhất quán
- **Internal/one-off code** → simplest approach wins

> Giải quyết conflict §4 vs §8: Consistency không conflict với KISS khi áp dụng đúng zone.

---

*Source: review-criteria-debate team synthesis | 2026-03-13*
*See `plans/reports/research-summary-260313-1127-review-criteria-standard-principles.md` for full debate.*
