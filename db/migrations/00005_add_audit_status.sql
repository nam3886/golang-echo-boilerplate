-- +goose Up
ALTER TABLE audit_logs ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'success';

-- +goose Down
ALTER TABLE audit_logs DROP COLUMN IF EXISTS status;
