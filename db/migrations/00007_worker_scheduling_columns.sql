-- +goose Up
-- +goose StatementBegin
ALTER TABLE uploads ADD COLUMN next_retry_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE uploads ADD COLUMN heartbeat_at TIMESTAMPTZ;

CREATE INDEX idx_uploads_worker_queue ON uploads(status, retry_count, next_retry_at, heartbeat_at) WHERE status = 'PROCESSING';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_uploads_worker_queue;
ALTER TABLE uploads DROP COLUMN IF EXISTS heartbeat_at;
ALTER TABLE uploads DROP COLUMN IF EXISTS next_retry_at;
-- +goose StatementEnd
