-- +goose Up
-- idx_users_active indexes PK column id with WHERE deleted_at IS NULL.
-- The PK B-tree already covers id lookups, making this index pure overhead.
DROP INDEX IF EXISTS idx_users_active;

-- +goose Down
CREATE INDEX idx_users_active ON users (id) WHERE deleted_at IS NULL;
