-- +goose Up
CREATE INDEX idx_users_pagination ON users (created_at DESC, id DESC) WHERE deleted_at IS NULL;

ALTER TABLE users ADD CONSTRAINT chk_users_role CHECK (role IN ('admin', 'member'));

-- +goose Down
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_role;

DROP INDEX IF EXISTS idx_users_pagination;
