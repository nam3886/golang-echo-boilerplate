# Boilerplate Review Criteria

Bộ tiêu chí đánh giá chất lượng boilerplate Go — tập trung DX, architecture, correctness.

**Scoring:**
| Điểm | Nghĩa |
|------|-------|
| 9–10 | Production-ready reference, minimal issues |
| 7–8 | Solid foundation, targeted fixes needed |
| 5–6 | Works but confusion/bugs for new joiners |
| < 5 | Architectural rework needed |

---

## 1. Correctness

> Code hoạt động đúng trong mọi điều kiện, không chỉ happy path.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Error semantics | `errors.Is/As` hoạt động đúng contract chuẩn Go? |
| Concurrency safety | Race conditions, atomic ops, goroutine leaks? |
| Data integrity | TOCTOU, constraint checks đầy đủ? |
| Error propagation | Lỗi có bị swallow không? Startup fail đúng chỗ không? |
| Type consistency | Mixed int/int64, signed/unsigned trong cùng domain? |

---

## 2. Architecture Integrity

> Các layer tách biệt rõ ràng, dependency đúng chiều, không có coupling ẩn.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Layer separation | Có cross-layer import không? Domain import infra? |
| No cross-module imports | Module A import module B trực tiếp? |
| Dependency direction | Arrows chỉ đúng chiều: domain ← app ← adapter |
| Interface contracts | Repo contract documented rõ? Caller biết retry được không? |
| Optional deps | Nil-receiver pattern hay conditional scattered? |

---

## 3. DX — New Joiner Experience

> Người mới join chỉ cần đọc 1 ví dụ là làm được, không cần hỏi.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Pattern clarity | Đọc 1 handler → hiểu pattern để viết handler tiếp theo? |
| Single source of truth | Cùng 1 việc có nhiều cách làm không? (event topic, error constructors) |
| Scaffold accuracy | `task module:create` sinh code compile được và đúng pattern không? |
| Docs accuracy | Docs mô tả đúng code thực tế? (function names, patterns) |
| Footguns documented | Traps có warning rõ ràng, không chỉ trong comment test file? |

---

## 4. Consistency

> Cùng loại vấn đề → cùng cách giải quyết, không có ngoại lệ không giải thích.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Error constructor style | Named `domain.ErrXxx()` vs inline `sharederr.New()` — chọn 1 |
| Logging context | `slog.InfoContext` hay `slog.Info` — nhất quán? |
| Test assertion style | `errors.Is` vs `errors.As + .Code` — chọn 1 |
| Failure mode | Redis fail → fail-open hay fail-closed — nhất quán? |
| Constraint check pattern | Create vs Update có cùng level of strictness không? |

---

## 5. Security

> Hệ thống an toàn theo mặc định, không cần config thêm.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| RBAC fail-closed | Unmapped procedures bị deny by default? |
| Error leakage | Internal errors bị expose ra client không? |
| Rate limiting | Atomic? Bypassable? |
| Auth blacklist | Fail-safe khi Redis down? |
| Password handling | Hash algo đúng? Excluded từ response/log? |

---

## 6. Observability

> Khi có bug production, có đủ context để debug không cần thêm code.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Trace propagation | `ctx` được pass qua đủ layers? `slog.XxxContext` được dùng? |
| Structured logging | Key-value pairs nhất quán? |
| Error logging | Duplicate logs? Đúng severity? |
| Sampling config | Tracer có config hay hardcode `AlwaysSample`? |

---

## 7. Testing Quality

> Tests bắt được bugs thật, không chỉ verify happy path.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Coverage breadth | Mỗi handler cover: success, not-found, validation, repo error, event failure? |
| Build tag discipline | Unit test ẩn sau `integration` tag không? |
| Mock scope | Mock đúng layer, không mock quá sâu? |
| Test helper reuse | Boilerplate trong test được extract ra helper chưa? |
| Integration realism | `testcontainers` với infra thật, không mock DB? |

---

## 8. YAGNI / KISS / DRY

> Code chỉ đủ phức tạp cho bài toán hiện tại, không more.

| Tiêu chí | Câu hỏi kiểm tra |
|----------|-----------------|
| Over-abstraction | Abstraction có đủ 2+ use cases không, hay chỉ 1? |
| Duplication | Code duplicate có lý do rõ ràng (vd: sqlc generated types)? |
| Dead code | Fields indexed nhưng không query? Re-exports không dùng? |
| Premature generics | Generic helper cho 1 use case duy nhất? |
