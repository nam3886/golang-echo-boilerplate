-- name: GetUserByID :one
SELECT id, email, name, role, created_at, updated_at, deleted_at FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT id, email, name, password, role, created_at, updated_at, deleted_at
FROM users WHERE email = $1 AND deleted_at IS NULL;

-- name: GetUserByIDForUpdate :one
SELECT id, email, name, password, role, created_at, updated_at, deleted_at
FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: ListUsers :many
SELECT id, email, name, role, created_at, updated_at, deleted_at FROM users
WHERE deleted_at IS NULL
  AND (sqlc.narg('cursor_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('cursor_created_at'), sqlc.narg('cursor_id')::uuid))
ORDER BY created_at DESC, id DESC
LIMIT $1;

-- name: CreateUser :one
INSERT INTO users (id, email, name, password, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, email, name, password, role, created_at, updated_at, deleted_at;

-- name: UpdateUser :one
UPDATE users
SET name = COALESCE(sqlc.narg('name'), name),
    role = COALESCE(sqlc.narg('role'), role),
    email = COALESCE(sqlc.narg('email'), email),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, email, name, password, role, created_at, updated_at, deleted_at;

-- name: SoftDeleteUser :one
UPDATE users SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL
RETURNING id, email, name, password, role, created_at, updated_at, deleted_at;
