-- +goose Up
-- +goose StatementBegin
CREATE TABLE posts (
    id UUID PRIMARY KEY,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(150) NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX idx_posts_author_id ON posts(author_id);
CREATE INDEX idx_posts_created_at ON posts(created_at DESC);

CREATE TABLE post_images (
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    upload_id UUID NOT NULL REFERENCES uploads(id) ON DELETE RESTRICT,
    position SMALLINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,

    PRIMARY KEY (post_id, upload_id),
    CONSTRAINT uq_post_images_upload_id UNIQUE (upload_id),
    CONSTRAINT uq_post_images_post_position UNIQUE (post_id, position) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT chk_post_images_position CHECK (position >= 0 AND position < 10)
);

CREATE INDEX idx_post_images_post_id_position ON post_images(post_id, position ASC);
CREATE INDEX idx_post_images_upload_id ON post_images(upload_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS post_images;
DROP TABLE IF EXISTS posts;
-- +goose StatementEnd
