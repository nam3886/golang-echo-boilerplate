# API Error Codes

| Code | HTTP | When |
|------|------|------|
| INVALID_ARGUMENT | 400 | Request validation failed |
| UNAUTHENTICATED | 401 | Missing/invalid token |
| PERMISSION_DENIED | 403 | Insufficient permissions |
| NOT_FOUND | 404 | Resource doesn't exist |
| ALREADY_EXISTS | 409 | Duplicate creation |
| FAILED_PRECONDITION | 412 | Precondition not met |
| RESOURCE_EXHAUSTED | 429 | Rate limit exceeded or quota exceeded |
| INTERNAL | 500 | Unexpected server error |
| UNAVAILABLE | 503 | Service temporarily unavailable |

## Error Response Format

```json
{
  "code": "INVALID_ARGUMENT",
  "message": "email is required"
}
```

## Important: Error Matching Behavior

`DomainError.Is()` uses **two-tier matching** based on whether both errors carry a Key:

1. **Both errors have a Key** → matches by `Code == Code && Key == Key` (precise)
2. **Either error has empty Key** → matches by `Code == Code` only (category)

Examples:
- `errors.Is(ErrUserNotFound(), ErrUserNotFound())` → `true` (same Code + Key)
- `errors.Is(ErrUserNotFound(), ErrOrderNotFound())` → `false` (same Code, different Key)
- `errors.Is(err, &DomainError{Code: CodeNotFound})` → `true` (category match — no Key on target)

**Correct patterns:**
```go
// Exact match — use the keyed sentinel from the module's errors.go
if errors.Is(err, domain.ErrUserNotFound()) { ... }

// Category match — use a key-less shared sentinel
if errors.Is(err, sharederr.ErrNotFound()) { ... } // only matches if sharederr.ErrNotFound() has Key=""
```

> ⚠️ `sharederr.ErrNotFound()` has `Key="not_found"` — so it does NOT category-match against
> keyed module errors like `ErrUserNotFound()` (Key="user.not_found"). For category-only matching,
> construct `sharederr.New(sharederr.CodeNotFound, "", "")` or check `errors.As` + `.Code`.

For HTTP status mapping, `DomainError.HTTPStatus()` uses the Code field directly — Key is irrelevant.
