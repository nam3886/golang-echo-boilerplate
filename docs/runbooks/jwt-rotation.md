# JWT Secret Rotation Runbook

## When to Rotate

- `JWT_SECRET` is suspected or confirmed compromised
- Periodic rotation policy (e.g., quarterly)
- Employee with access to secrets offboards

## Impact

Rotating `JWT_SECRET` immediately invalidates **all** active tokens:
- Access tokens (default TTL: 15 min) — users re-authenticate at next request
- Refresh tokens (default TTL: 168 h) — users must log in again

Plan rotation during low-traffic windows if refresh token invalidation is unacceptable.

## Steps

### 1. Generate a new secret

```bash
openssl rand -base64 48
```

The output must be ≥ 32 characters (enforced at startup).

### 2. Update deployment configuration

Set `JWT_SECRET` to the new value in your secrets manager (Vault, AWS Secrets Manager, K8s Secret, etc.).

### 3. Deploy

Rolling deploy picks up the new secret. New tokens are signed with the new secret immediately. Existing tokens signed with the old secret will fail `401 Unauthorized` on the next request.

### 4. Monitor

Watch for an elevated `401` rate in logs/dashboards for ~15 minutes (access token TTL). Expected behavior — clients will re-authenticate.

```
# Example log filter
status=401 path=/connect.*
```

### 5. Rollback (if needed)

Revert `JWT_SECRET` to the previous value and redeploy. Tokens issued during the window with the new secret become invalid again.

## Zero-Downtime Option (Dual-Secret Grace Period)

For zero-disruption rotation, implement dual-secret validation:

1. Add `JWT_SECRET_PREV` env var
2. In `auth.ParseToken`, try the new secret first; on failure try `JWT_SECRET_PREV`
3. Deploy → drain old tokens (wait one refresh TTL = 168 h) → remove `JWT_SECRET_PREV`

This is YAGNI until actually needed — document the approach here rather than pre-implementing.

## Verification

```bash
# Confirm old tokens are rejected
curl -H "Authorization: Bearer <old_token>" http://localhost:8080/connect/...
# Expected: 401 Unauthorized

# Confirm new tokens work
curl -X POST .../auth/login -d '{"email":"...","password":"..."}'
# Use returned token in subsequent requests
```
