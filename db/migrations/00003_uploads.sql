-- +goose Up
-- +goose StatementBegin
CREATE TYPE upload_status AS ENUM (
    'PENDING',
    'PROCESSING',
    'COMPLETED',
    'REJECTED',
    'EXPIRED',
    'SUPERSEDED',
    'DELETED'
);

CREATE TABLE uploads (
    id             UUID PRIMARY KEY,
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    object_key     VARCHAR(512) UNIQUE NOT NULL,
    status         upload_status NOT NULL DEFAULT 'PENDING',
    file_size      INT,
    content_type   VARCHAR(100),
    failure_reason TEXT,
    created_at     TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at     TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE INDEX idx_uploads_user ON uploads(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS uploads;
DROP TYPE IF EXISTS upload_status;
-- +goose StatementEnd