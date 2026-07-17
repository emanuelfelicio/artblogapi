-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
    DROP COLUMN IF EXISTS avatar_url,
    DROP COLUMN IF EXISTS banner_url,
    ADD COLUMN avatar_upload_id UUID REFERENCES uploads(id) ON DELETE SET NULL,
    ADD COLUMN banner_upload_id UUID REFERENCES uploads(id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users
    DROP COLUMN IF EXISTS avatar_upload_id,
    DROP COLUMN IF EXISTS banner_upload_id,
    ADD COLUMN avatar_url TEXT,
    ADD COLUMN banner_url TEXT;
-- +goose StatementEnd