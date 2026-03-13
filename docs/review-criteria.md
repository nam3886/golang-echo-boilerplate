# Boilerplate Review Criteria

Bộ tiêu chí đánh giá chất lượng boilerplate Go — tập trung DX, architecture, correctness.

## Weighted Scoring

| Criterion | Weight | Rationale |
|-----------|--------|-----------|
| Correctness | 3.0 | Race conditions + idempotency = data corruption risk |
| Security | 2.0 | Auth failures = breach risk |
| Architecture | 2.0 | Layer violations = unmaintainable codebase |
| Testing Quality | 1.0 | Coverage gaps = silent regressions |
| DX | 1.0 | Poor DX = slow onboarding + wrong patterns |
| Observability | 0.5 | Missing logs = undebuggable production |
| Consistency | 0.5 | Inconsistency = confusion, not correctness |
| **Total** | **10.0** | |

**Score interpretation:**
| Điểm | Nghĩa |
|------|-------|
| 9–10 | Production-ready reference, minimal issues |
| 7–8 | Solid foundation, targeted fixes needed |
| 5–6 | Works but confusion/bugs for new joiners |
| < 5 | Architectural rework needed |

> Criteria áp dụng độc lập — high correctness + low DX ≠ average. Ghi điểm từng dimension riêng, tổng là tham khảo.

---

## 1. Correctness `(3.0 pts)`

> Code hoạt động đúng trong mọi điều kiện, không chỉ happy path.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Error semantics | `errors.Is/As` hoạt động đúng? Sentinel errors dùng constructor functions (không phải `var`)? |
| Concurrency safety | Race conditions, atomic ops, goroutine leaks? |
| Data integrity | TOCTOU, constraint checks đầy đủ? |
| Error propagation | Startup fail rõ level: Fatal (unbootable) vs Error (degraded)? Lỗi không bị swallow? |
| Type consistency | Mixed int/int64, signed/unsigned trong cùng domain? |
| Nil guard | Required deps non-nil tại construction? Optional deps dùng config flag, không nil checks tản mát? |
| Idempotency | Event handlers retry-safe? (dedup key, upsert, hoặc audit-based detection) |

---

## 2. Architecture Integrity `(2.0 pts)`

> Các layer tách biệt rõ ràng, dependency đúng chiều, không có coupling ẩn.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Layer separation | Có cross-layer import không? Domain import infra? |
| No cross-module imports | Module A import module B trực tiếp? |
| Dependency direction | Arrows chỉ đúng chiều: domain ← app ← adapter |
| Interface contracts | Repo interface có doc idempotency, error semantics, TOCTOU handling? |
| Dependency classification | Required deps non-nil tại startup? Optional deps documented trong struct? |
| Event schema | Events có version field? Breaking change protocol documented? |

---

## 3. DX — New Joiner Experience `(1.0 pt)`

> Người mới join chỉ cần đọc 1 ví dụ là làm được, không cần hỏi.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Pattern coverage | Scaffold/example cover CRUD + edge cases? (Create với event, Read với 404, Update với conflict, Delete với cascade) |
| Single source of truth | Cùng 1 việc có nhiều cách làm không? (scope: trong 1 module — cross-module OK nếu documented) |
| Scaffold accuracy | `task module:create` sinh code compile được và đúng pattern không? |
| Docs accuracy | Docs mô tả đúng code thực tế? (function names, patterns) |
| Footguns documented | API contract violations + performance cliffs có `⚠️` trong godoc/README? (không chỉ trong test comments) |

---

## 4. Consistency `(0.5 pts)`

> Cùng loại vấn đề → cùng cách giải quyết trong cùng zone.

> **⚠️ Consistency Zones** (giải quyết conflict với §8 KISS):
> - **Public/shared patterns** (repo interfaces, all handlers trong 1 module): bắt buộc nhất quán
> - **Internal/one-off code**: simplest approach wins

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Error constructor style | Named `domain.ErrXxx()` vs inline `sharederr.New()` — nhất quán trong 1 module; cross-module khác style OK nếu documented |
| Logging context | `slog.InfoContext` everywhere; `slog.Info` chỉ cho lifecycle logs (startup/shutdown) |
| Test assertion style | `errors.Is` vs `errors.As + .Code` — chọn 1, không mix trong 1 module |
| Failure mode declaration | Mỗi external dep (Redis, ES) có declare fail-open/fail-closed trong module README? |
| Constraint check parity | Create + Update validate cùng constraints; ngoại lệ có ADR? |

---

## 5. Security `(2.0 pts)`

> Hệ thống an toàn theo mặc định, không cần config thêm.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| RBAC fail-closed | Unmapped procedures bị deny by default? |
| Error leakage | Internal errors bị expose ra client không? |
| Input validation boundary | Validation tập trung tại domain constructors, không scatter trong handlers? |
| Rate limiting | Scope defined (per-ip/user/key)? Algorithm defined? Distributed (Redis-backed)? |
| Auth blacklist | Fail strategy explicit: fail-closed (default) hay fail-open với local cache? |
| Password handling | Hash algo đúng (bcrypt/argon2)? Excluded từ response/log? |
| HTTPS + CORS | HTTP redirect to HTTPS? CORS AllowOrigins explicit (không dùng `*`)? |
| Audit trail | Sensitive ops log Who + What + When + Status? |

---

## 6. Observability `(0.5 pts)`

> Khi có bug production, có đủ context để debug không cần thêm code.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Trace propagation | `ctx` pass qua đủ layers? `slog.XxxContext` dùng mọi nơi (trừ lifecycle logs)? |
| Structured logging — standard keys | `user_id`, `module`, `operation`, `error_code`, `duration_ms`, `retry_count` — nhất quán? |
| Error logging — single point | Log exactly once tại catch boundary, không duplicate across layers? |
| MVL per error | Mỗi error log có: error_code + operation + resource_id + retryable flag? |
| Sampling config | Env-aware: dev=AlwaysSample, prod=1% (configurable)? Không hardcode? |

---

## 7. Testing Quality `(1.0 pt)`

> Tests bắt được bugs thật theo đúng layer.

> **⚠️ Testing Pyramid** (giải quyết conflict giữa §7 và §8 YAGNI):
> - **App logic** (handlers, use cases): unit tests + mock repos → fast
> - **Adapters** (sqlc, Redis, Watermill): integration tests + testcontainers → real infra
> - **Full flow**: 1 integration test per module (sample workflow)

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Layer-appropriate strategy | App logic dùng mock repos (unit)? Adapters dùng testcontainers (integration)? |
| Coverage breadth — risk-based | Critical paths (auth, payments): 5 scenarios? Standard CRUD: 3–4? Utilities: 1–2? |
| Build tag discipline | Unit tests không ẩn sau `integration` tag? |
| Mock scope | Mock đúng layer (repo interface), không mock internal implementations? |
| Test helper reuse | Boilerplate trong test được extract ra helper (testutil.NewTestPostgres, testutil.Ptr)? |

---

## 8. YAGNI / KISS / DRY `(scored via other dimensions)`

> Code chỉ đủ phức tạp cho bài toán hiện tại.

> **⚠️ Không có điểm riêng** — vi phạm YAGNI/KISS/DRY làm giảm điểm ở dimension liên quan (Architecture, DX, Correctness).

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Abstraction maturity | Abstraction có 2+ actual use cases + stable (không thay đổi trong 2+ tuần)? |
| DRY exceptions | Duplication có lý do: generated code, different domain semantics, boilerplate layers? |
| Dead code | Fields indexed nhưng không query? Re-exports không dùng? |
| Premature generics | Generic cho 1 use case duy nhất? (Phân biệt: generic reuse vs generic type-safety) |

---

## Conflict Resolutions

| Conflict | Resolution |
|----------|-----------|
| §4 Consistency vs §8 KISS | **Consistency Zones**: bắt buộc ở public/shared patterns; simplest approach ở internal/one-off |
| §7 Coverage breadth vs §8 YAGNI | **Risk-based coverage**: critical → 5 scenarios; standard → 3–4; utility → 1–2 |
| §7 Integration realism vs CI speed | **Testing Pyramid**: adapters dùng testcontainers; app logic dùng mock repos |
| §8 DRY vs §7 Test isolation | **DRY exceptions**: test helpers OK nếu stateless; avoid shared mutable test state |

---

*See `docs/enforcement-guidelines.md` for concrete implementation rules per principle.*
