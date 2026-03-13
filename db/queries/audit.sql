-- name: CreateAuditLog :exec
INSERT INTO audit_logs (id, entity_type, entity_id, action, actor_id, changes, ip_address, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO NOTHING;
