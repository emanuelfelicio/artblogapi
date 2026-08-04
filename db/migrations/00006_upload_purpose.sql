-- +goose Up
-- +goose StatementBegin
CREATE TYPE upload_purpose AS ENUM (
    'AVATAR',
    'BANNER',
    'POST_IMAGE'
);

ALTER TABLE uploads ADD COLUMN purpose upload_purpose NOT NULL;
ALTER TABLE uploads ADD COLUMN retry_count INT NOT NULL DEFAULT 0;

ALTER TABLE uploads DROP CONSTRAINT IF EXISTS uploads_user_id_fkey;
ALTER TABLE uploads ADD CONSTRAINT uploads_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT;

CREATE INDEX idx_uploads_processing ON uploads(status, updated_at) WHERE status = 'PROCESSING';
CREATE INDEX idx_uploads_gc_superseded ON uploads(status, updated_at) WHERE status = 'SUPERSEDED';
CREATE INDEX idx_uploads_gc_orphans ON uploads(status, created_at) WHERE status = 'COMPLETED' OR status = 'PENDING';

UPDATE uploads SET status = 'BOUND' WHERE id IN (
    SELECT avatar_upload_id FROM users WHERE avatar_upload_id IS NOT NULL
    UNION
    SELECT banner_upload_id FROM users WHERE banner_upload_id IS NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE uploads SET status = 'COMPLETED' WHERE status = 'BOUND';

ALTER TABLE uploads DROP COLUMN IF EXISTS purpose;
ALTER TABLE uploads DROP COLUMN IF EXISTS retry_count;
DROP TYPE IF EXISTS upload_purpose;

ALTER TABLE uploads DROP CONSTRAINT IF EXISTS uploads_user_id_fkey;
ALTER TABLE uploads ADD CONSTRAINT uploads_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_uploads_processing;
DROP INDEX IF EXISTS idx_uploads_gc_superseded;
DROP INDEX IF EXISTS idx_uploads_gc_orphans;
-- +goose StatementEnd
