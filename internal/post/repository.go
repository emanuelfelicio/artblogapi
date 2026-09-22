package post

import (
	"context"
	"errors"
	"fmt"

	"github.com/emanuelfelicio/artblogapi/db"
	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	query *dbgen.Queries
	pool  *pgxpool.Pool
}

func NewRepository(q *dbgen.Queries, pool *pgxpool.Pool) *repository {
	return &repository{query: q, pool: pool}
}

func (r *repository) q(ctx context.Context) *dbgen.Queries {
	if tx, ok := db.TxFromContext(ctx); ok {
		return r.query.WithTx(tx)
	}
	return r.query
}

func (r *repository) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	if r.pool == nil {
		return errors.New("with_transaction: database connection pool is nil")
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return fn(db.ContextWithTx(ctx, tx))
	})
}

func (r *repository) CreatePost(ctx context.Context, id, authorID uuid.UUID, title, content string) (Post, error) {
	row, err := r.q(ctx).CreatePost(ctx, dbgen.CreatePostParams{
		ID:       id,
		AuthorID: authorID,
		Title:    title,
		Content:  content,
	})
	if err != nil {
		return Post{}, fmt.Errorf("create_post: %w", err)
	}

	return Post{
		ID:        row.ID,
		AuthorID:  row.AuthorID,
		Title:     row.Title,
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *repository) GetPostByID(ctx context.Context, id uuid.UUID) (Post, error) {
	row, err := r.q(ctx).GetPostByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Post{}, ErrPostNotFound
		}
		return Post{}, fmt.Errorf("get_post_by_id: %w", err)
	}

	return mapGetPostByIDRow(row), nil
}

func (r *repository) GetPostByIDForUpdate(ctx context.Context, id uuid.UUID) (Post, error) {
	row, err := r.q(ctx).GetPostByIDForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Post{}, ErrPostNotFound
		}
		return Post{}, fmt.Errorf("get_post_by_id_for_update: %w", err)
	}

	return Post{
		ID:        row.ID,
		AuthorID:  row.AuthorID,
		Title:     row.Title,
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *repository) UpdatePost(ctx context.Context, id uuid.UUID, title, content *string) (Post, error) {
	row, err := r.q(ctx).UpdatePost(ctx, dbgen.UpdatePostParams{
		ID:      id,
		Title:   textParam(title),
		Content: textParam(content),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Post{}, ErrPostNotFound
		}
		return Post{}, fmt.Errorf("update_post: %w", err)
	}

	return Post{
		ID:        row.ID,
		AuthorID:  row.AuthorID,
		Title:     row.Title,
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *repository) DeletePost(ctx context.Context, id uuid.UUID) error {
	if err := r.q(ctx).DeletePost(ctx, id); err != nil {
		return fmt.Errorf("delete_post: %w", err)
	}
	return nil
}

func (r *repository) InsertPostImage(ctx context.Context, postID, uploadID uuid.UUID, position int16) error {
	err := r.q(ctx).InsertPostImage(ctx, dbgen.InsertPostImageParams{
		PostID:   postID,
		UploadID: uploadID,
		Position: position,
	})
	if err != nil {
		return fmt.Errorf("insert_post_image: %w", err)
	}
	return nil
}

func (r *repository) GetPostImagesByPostID(ctx context.Context, postID uuid.UUID) ([]PostImage, error) {
	rows, err := r.q(ctx).GetPostImagesByPostID(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("get_post_images_by_post_id: %w", err)
	}

	images := make([]PostImage, 0, len(rows))
	for _, row := range rows {
		contentType := ""
		if row.ContentType.Valid {
			contentType = row.ContentType.String
		}
		images = append(images, PostImage{
			PostID:      row.PostID,
			UploadID:    row.UploadID,
			Position:    int(row.Position),
			ObjectKey:   row.ObjectKey,
			ContentType: contentType,
			CreatedAt:   row.CreatedAt.Time,
		})
	}
	return images, nil
}

func (r *repository) GetPostImagesByPostIDs(ctx context.Context, postIDs []uuid.UUID) (map[uuid.UUID][]PostImage, error) {
	result := make(map[uuid.UUID][]PostImage)
	if len(postIDs) == 0 {
		return result, nil
	}

	rows, err := r.q(ctx).GetPostImagesByPostIDs(ctx, postIDs)
	if err != nil {
		return nil, fmt.Errorf("get_post_images_by_post_ids: %w", err)
	}

	for _, row := range rows {
		contentType := ""
		if row.ContentType.Valid {
			contentType = row.ContentType.String
		}
		result[row.PostID] = append(result[row.PostID], PostImage{
			PostID:      row.PostID,
			UploadID:    row.UploadID,
			Position:    int(row.Position),
			ObjectKey:   row.ObjectKey,
			ContentType: contentType,
			CreatedAt:   row.CreatedAt.Time,
		})
	}
	return result, nil
}

func (r *repository) DeletePostImage(ctx context.Context, postID, uploadID uuid.UUID) error {
	err := r.q(ctx).DeletePostImage(ctx, dbgen.DeletePostImageParams{
		PostID:   postID,
		UploadID: uploadID,
	})
	if err != nil {
		return fmt.Errorf("delete_post_image: %w", err)
	}
	return nil
}

func (r *repository) UpdatePostImagePosition(ctx context.Context, postID, uploadID uuid.UUID, position int16) error {
	err := r.q(ctx).UpdatePostImagePosition(ctx, dbgen.UpdatePostImagePositionParams{
		PostID:   postID,
		UploadID: uploadID,
		Position: position,
	})
	if err != nil {
		return fmt.Errorf("update_post_image_position: %w", err)
	}
	return nil
}

func (r *repository) ListRecentPosts(ctx context.Context, limit, offset int32) ([]Post, error) {
	rows, err := r.q(ctx).ListRecentPosts(ctx, dbgen.ListRecentPostsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list_recent_posts: %w", err)
	}

	posts := make([]Post, 0, len(rows))
	for _, row := range rows {
		posts = append(posts, mapListRecentPostsRow(row))
	}
	return posts, nil
}

func (r *repository) ListPostsByAuthor(ctx context.Context, authorID uuid.UUID, limit, offset int32) ([]Post, error) {
	rows, err := r.q(ctx).ListPostsByAuthor(ctx, dbgen.ListPostsByAuthorParams{
		AuthorID: authorID,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list_posts_by_author: %w", err)
	}

	posts := make([]Post, 0, len(rows))
	for _, row := range rows {
		posts = append(posts, mapListPostsByAuthorRow(row))
	}
	return posts, nil
}

func textParam(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

func mapGetPostByIDRow(row dbgen.GetPostByIDRow) Post {
	p := Post{
		ID:        row.ID,
		AuthorID:  row.AuthorID,
		Title:     row.Title,
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
		Author: PostAuthor{
			ID:          row.AuthorID,
			Username:    row.AuthorUsername,
			DisplayName: row.AuthorDisplayName.String,
		},
	}
	if row.AuthorAvatarKey.Valid {
		key := row.AuthorAvatarKey.String
		p.Author.AvatarKey = &key
	}
	return p
}

func mapListRecentPostsRow(row dbgen.ListRecentPostsRow) Post {
	p := Post{
		ID:        row.ID,
		AuthorID:  row.AuthorID,
		Title:     row.Title,
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
		Author: PostAuthor{
			ID:          row.AuthorID,
			Username:    row.AuthorUsername,
			DisplayName: row.AuthorDisplayName.String,
		},
	}
	if row.AuthorAvatarKey.Valid {
		key := row.AuthorAvatarKey.String
		p.Author.AvatarKey = &key
	}
	return p
}

func mapListPostsByAuthorRow(row dbgen.ListPostsByAuthorRow) Post {
	p := Post{
		ID:        row.ID,
		AuthorID:  row.AuthorID,
		Title:     row.Title,
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
		Author: PostAuthor{
			ID:          row.AuthorID,
			Username:    row.AuthorUsername,
			DisplayName: row.AuthorDisplayName.String,
		},
	}
	if row.AuthorAvatarKey.Valid {
		key := row.AuthorAvatarKey.String
		p.Author.AvatarKey = &key
	}
	return p
}
