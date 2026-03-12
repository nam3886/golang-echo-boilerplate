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

`DomainError.Is()` matches by error **code** (category), not by identity.
This means `errors.Is(ErrUserNotFound(), ErrOrderNotFound())` returns `true`
because both use `CodeNotFound`.

For HTTP status mapping, this is correct. For distinguishing between specific
errors of the same category, use `errors.As()` and check the `Message` field.
