-- name: CreateAuditLog :exec
INSERT INTO audit_logs (id, entity_type, entity_id, action, actor_id, changes, ip_address)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO NOTHING;
