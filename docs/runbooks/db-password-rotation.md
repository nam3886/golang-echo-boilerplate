# Runbook: DB Password Rotation

## When to use

Rotate the PostgreSQL password on a schedule (≥90 days) or immediately after a suspected credential leak.

## Steps

### 1. Generate a new password

```bash
openssl rand -base64 32
```

### 2. Update the password in PostgreSQL

```sql
ALTER USER <db_user> WITH PASSWORD '<new_password>';
```

### 3. Update the secret in your secrets manager

Update `DATABASE_URL` in your secrets manager (Vault, AWS Secrets Manager, etc.) with the new password:

```
postgres://<user>:<new_password>@<host>:<port>/<dbname>?sslmode=require
```

### 4. Rolling restart the application

The application reads `DATABASE_URL` at startup. Perform a rolling restart so connections switch to the new credentials without downtime:

```bash
# Kubernetes
kubectl rollout restart deployment/<app-name>

# Docker Compose (dev)
docker compose up -d --force-recreate app
```

### 5. Verify connectivity

```bash
psql "$DATABASE_URL" -c "SELECT 1"
```

Check application logs for connection errors after restart.

### 6. Revoke the old password (optional)

If the old password was leaked, ensure old sessions are terminated:

```sql
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE usename = '<db_user>' AND application_name != 'psql';
```

## Rollback

If the application fails to connect after rotation, restore the old `DATABASE_URL` in the secrets manager and restart.

## Notes

- `DATABASE_URL` is the only required DB credential — no separate read replica URL unless you add one.
- Migrations (`task migrate:up`) also use `DATABASE_URL`; update `.env` for local dev.
- Never commit the new password to `.env` or any tracked file.
