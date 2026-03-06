# Documentation Update Analysis: Boilerplate Fixes

**Date:** 2026-03-05
**Status:** Analysis Complete
**Context:** 6 boilerplate fixes affecting user module, repository patterns, and event handling

---

## Summary

Verified code implementations against current documentation. Found **4 areas requiring documentation updates**. 3 of 6 fixes are already documented or not documentation-dependent.

---

## Findings by Fix

### 1. Pagination: List Returns (users, cursor, hasMore, error) ✓ UPDATE NEEDED

**Status:** Not fully documented

**Implementation Evidence:**
- `domain/repository.go:9` — `List(ctx context.Context, limit int, cursor string) ([]*User, string, bool, error)`
- `adapters/postgres/repository.go:59-96` — Implements limit+1 probe internally
- Lines 62-63: Fetches `limit+1` to detect more pages
- Lines 82-85: Strips extra row if hasMore detected

**Documentation Gaps:**
- `code-standards.md` shows old interface signature without pagination return values
- `architecture.md` doesn't mention cursor-based pagination or hasMore pattern
- No documentation of limit+1 probing pattern used internally

**Action:** Update `code-standards.md` repository interface section with new signature and explain the limit+1 pattern.

---

### 2. SoftDelete: Returns ErrNotFound for Non-existent Users ✓ UPDATE NEEDED

**Status:** Not documented

**Implementation Evidence:**
- `domain/repository.go:12` — `SoftDelete(ctx context.Context, id UserID) error`
- `adapters/postgres/repository.go:156-170` — Checks `rows == 0` and returns `sharederr.ErrNotFound`
- Line 167: `if rows == 0 { return sharederr.ErrNotFound }`

**Documentation Gaps:**
- `code-standards.md` doesn't mention SoftDelete behavior
- Error handling section doesn't document expected ErrNotFound for non-existent users

**Action:** Add SoftDelete documentation to repository interface section with error behavior explanation.

---

### 3. Create: Catches Postgres 23505 → domain.ErrEmailTaken ✓ ALREADY DOCUMENTED

**Status:** Documented but needs example update

**Implementation Evidence:**
- `adapters/postgres/repository.go:108-110` — Catches pgErr.Code "23505" and returns `domain.ErrEmailTaken`
- `code-standards.md:233-240` — CreateUserHandler example shows validation before repo.Create()
- `error-codes.md:9` — `ALREADY_EXISTS` (409) error code defined

**Current Documentation:**
- `code-standards.md` shows email uniqueness check in app handler (good pattern)
- Example doesn't show the catch-all for duplicate constraint in repository

**Action:** Minor — Add inline comment in code-standards.md example showing repository-level constraint catching as fallback.

---

### 4. Update/Delete Handlers Publish Events ✓ UPDATE NEEDED

**Status:** Partially documented

**Implementation Evidence:**

**Update Handler** (`app/update_user.go:56-63`):
- Publishes `UserUpdatedEvent` to `TopicUserUpdated`
- Includes ActorID extraction
- Graceful error logging (doesn't fail handler on event error)

**Delete Handler** (`app/delete_user.go:34-41`):
- Publishes `UserDeletedEvent` to `TopicUserDeleted`
- Includes ActorID extraction
- Graceful error logging

**Current Documentation:**
- `code-standards.md:257-265` shows CreateUserHandler with event publishing
- **Missing:** UpdateUserHandler and DeleteUserHandler examples
- **Missing:** Event publishing patterns for mutations beyond Create

**Action:** Add UpdateUserHandler and DeleteUserHandler examples to code-standards.md showing event publishing pattern consistency.

---

### 5. All Events Now Carry ActorID ✓ UPDATE NEEDED

**Status:** Partially documented

**Implementation Evidence:**
- `shared/events/topics.go:12-34` — All three event types have `ActorID string` field
- `app/create_user.go:62-68` — ActorID extracted from auth context
- `app/update_user.go:52-58` — ActorID extracted from auth context
- `app/delete_user.go:30-36` — ActorID extracted from auth context

**Current Documentation:**
- `code-standards.md:259-265` shows UserCreatedEvent with ActorID field
- **Missing:** Documentation that ActorID extraction is standard pattern for all mutations
- **Missing:** Explanation of auth.UserFromContext() pattern
- **Missing:** Event type definitions in documentation

**Action:**
1. Add section to code-standards.md documenting event structure and ActorID pattern
2. Document auth.UserFromContext() usage pattern
3. Reference event topic constants and types

---

### 6. protovalidate Interceptor Wired via connectrpc.com/validate ✓ ALREADY DOCUMENTED

**Status:** Implementation matches expected patterns, not documented

**Implementation Evidence:**
- `adapters/grpc/routes.go:7` — Import `"connectrpc.com/validate"`
- `routes.go:18` — `connect.WithInterceptors(validate.NewInterceptor())`
- Routes mounted with auth middleware on top

**Current Documentation:**
- `architecture.md:39` lists middleware chain but doesn't include validation interception
- No proto-level validation documentation

**Action:** Document in architecture.md that protovalidate interceptor is wired automatically via NewInterceptor(). Add to middleware chain description.

---

## Documentation Files Status

| File | Change Type | Sections | Priority |
|------|-------------|----------|----------|
| `code-standards.md` | Update | Repository interface signature, Create error handling, UpdateUserHandler, DeleteUserHandler, Event publishing patterns, ActorID extraction | HIGH |
| `architecture.md` | Update | Middleware chain (add validation), Event structure | MEDIUM |
| `adding-a-module.md` | Update | Repository interface example signature | LOW |
| `error-codes.md` | No change | ALREADY_EXISTS code sufficient | - |
| **Changelog** | Missing | Need to create | HIGH |

---

## Recommended Updates

### HIGH Priority (Blocking)

1. **code-standards.md** — Repository interface section
   - Update List signature with pagination return values
   - Document SoftDelete with ErrNotFound behavior
   - Add UpdateUserHandler handler example
   - Add DeleteUserHandler handler example
   - Add Event structure documentation section
   - Document ActorID extraction pattern with auth.UserFromContext()

2. **Create project-changelog.md**
   - Document this batch of boilerplate fixes
   - Record version/date information

### MEDIUM Priority

3. **architecture.md**
   - Update middleware chain to include protovalidate interception
   - Document event publishing flow for mutations

### LOW Priority

4. **adding-a-module.md**
   - Update Repository interface example to match new pagination signature

---

## Implementation Order

1. Create `docs/project-changelog.md` with boilerplate fix batch entry
2. Update `docs/code-standards.md` repository patterns and handler examples
3. Update `docs/architecture.md` middleware and event flow
4. Update `docs/adding-a-module.md` example signatures

---

## Verification Checklist

- [x] Verified List pagination signature in domain/repository.go
- [x] Verified SoftDelete ErrNotFound behavior in postgres/repository.go
- [x] Verified 23505 constraint catch in Create handler
- [x] Verified Update/Delete event publishing in app handlers
- [x] Verified ActorID extraction across all handlers
- [x] Verified protovalidate interceptor wiring in routes.go
- [x] Cross-referenced code with existing documentation
- [x] Identified all documentation gaps

---

## Notes

- All implementations follow established patterns (hexagonal architecture, event bus, middleware chain)
- Code is well-structured; documentation needs alignment, not code changes
- Event handling with graceful error logging is production-ready pattern worth documenting
- ActorID field enables audit trail correlation across all mutations
