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

	return mapPost(row), nil
}

func (r *repository) GetPostByID(ctx context.Context, id uuid.UUID) (Post, error) {
	row, err := r.q(ctx).GetPostByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Post{}, ErrPostNotFound
		}
		return Post{}, fmt.Errorf("get_post_by_id: %w", err)
	}

	return mapPost(row), nil
}

func (r *repository) GetPostByIDForUpdate(ctx context.Context, id uuid.UUID) (Post, error) {
	row, err := r.q(ctx).GetPostByIDForUpdate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Post{}, ErrPostNotFound
		}
		return Post{}, fmt.Errorf("get_post_by_id_for_update: %w", err)
	}

	return mapPost(row), nil
}

func (r *repository) GetPostWithImages(ctx context.Context, id uuid.UUID) (Post, error) {
	rows, err := r.q(ctx).GetPostWithImagesByID(ctx, id)
	if err != nil {
		return Post{}, fmt.Errorf("get_post_with_images: %w", err)
	}
	if len(rows) == 0 {
		return Post{}, ErrPostNotFound
	}

	p := Post{
		ID:        rows[0].ID,
		AuthorID:  rows[0].AuthorID,
		Title:     rows[0].Title,
		Content:   rows[0].Content,
		CreatedAt: rows[0].CreatedAt.Time,
		UpdatedAt: rows[0].UpdatedAt.Time,
		Images:    make([]PostImage, 0, len(rows)),
	}

	for _, row := range rows {
		if row.UploadID.Valid {
			contentType := ""
			if row.ContentType.Valid {
				contentType = row.ContentType.String
			}
			objectKey := ""
			if row.ObjectKey.Valid {
				objectKey = row.ObjectKey.String
			}
			p.Images = append(p.Images, PostImage{
				PostID:      row.ID,
				UploadID:    row.UploadID.Bytes,
				Position:    int(row.Position.Int16),
				ObjectKey:   objectKey,
				ContentType: contentType,
				CreatedAt:   row.ImageCreatedAt.Time,
			})
		}
	}

	return p, nil
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

	return mapPost(row), nil
}

func (r *repository) DeletePost(ctx context.Context, id uuid.UUID) error {
	if err := r.q(ctx).DeletePost(ctx, id); err != nil {
		return fmt.Errorf("delete_post: %w", err)
	}
	return nil
}

func (r *repository) BatchInsertPostImages(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID, positions []int16) error {
	if len(uploadIDs) == 0 {
		return nil
	}
	err := r.q(ctx).BatchInsertPostImages(ctx, dbgen.BatchInsertPostImagesParams{
		PostID:  postID,
		Column2: uploadIDs,
		Column3: positions,
	})
	if err != nil {
		return fmt.Errorf("batch_insert_post_images: %w", err)
	}
	return nil
}

func (r *repository) DeletePostImages(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID) error {
	if len(uploadIDs) == 0 {
		return nil
	}
	err := r.q(ctx).DeletePostImages(ctx, dbgen.DeletePostImagesParams{
		PostID:  postID,
		Column2: uploadIDs,
	})
	if err != nil {
		return fmt.Errorf("delete_post_images: %w", err)
	}
	return nil
}

func (r *repository) UpdatePostImagePositions(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID, positions []int16) error {
	if len(uploadIDs) == 0 {
		return nil
	}
	err := r.q(ctx).UpdatePostImagePositions(ctx, dbgen.UpdatePostImagePositionsParams{
		PostID:  postID,
		Column2: uploadIDs,
		Column3: positions,
	})
	if err != nil {
		return fmt.Errorf("update_post_image_positions: %w", err)
	}
	return nil
}

func (r *repository) DeletePostImagesByPostID(ctx context.Context, postID uuid.UUID) ([]uuid.UUID, error) {
	uploadIDs, err := r.q(ctx).DeletePostImagesByPostID(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("delete_post_images_by_post_id: %w", err)
	}
	return uploadIDs, nil
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
		posts = append(posts, mapPost(row))
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
		posts = append(posts, mapPost(row))
	}
	return posts, nil
}

func textParam(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

func mapPost(row dbgen.Post) Post {
	return Post{
		ID:        row.ID,
		AuthorID:  row.AuthorID,
		Title:     row.Title,
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}
