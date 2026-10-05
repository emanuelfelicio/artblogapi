package comment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/emanuelfelicio/artblogapi/db"
	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	WithTransaction(context.Context, func(context.Context) error) error
	PostExists(context.Context, uuid.UUID) error
	Create(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (Comment, error)
	ListByPost(context.Context, uuid.UUID, int32, int32) ([]Comment, error)
	GetForUpdate(context.Context, uuid.UUID, uuid.UUID) (Comment, error)
	Update(context.Context, uuid.UUID, uuid.UUID, string) (Comment, error)
	SoftDelete(context.Context, uuid.UUID, uuid.UUID) error
}

type repository struct {
	query *dbgen.Queries
	pool  *pgxpool.Pool
}

func NewRepository(query *dbgen.Queries, pool *pgxpool.Pool) *repository {
	return &repository{query: query, pool: pool}
}

func (r *repository) q(ctx context.Context) *dbgen.Queries {
	if tx, ok := db.TxFromContext(ctx); ok {
		return r.query.WithTx(tx)
	}
	return r.query
}

func (r *repository) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	if r.pool == nil {
		return errors.New("with_transaction: database connection pool is nil")
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return fn(db.ContextWithTx(ctx, tx))
	})
}

func (r *repository) PostExists(ctx context.Context, id uuid.UUID) error {
	if _, err := r.q(ctx).GetPostByIDForComment(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPostNotFound
		}
		return fmt.Errorf("get_post_for_comment: %w", err)
	}
	return nil
}

func (r *repository) Create(ctx context.Context, id, postID, authorID uuid.UUID, content string) (Comment, error) {
	row, err := r.q(ctx).CreateComment(ctx, dbgen.CreateCommentParams{ID: id, PostID: postID, AuthorID: authorID, Content: content})
	if err != nil {
		return Comment{}, fmt.Errorf("create_comment: %w", err)
	}
	return mapComment(row), nil
}

func (r *repository) ListByPost(ctx context.Context, postID uuid.UUID, limit, offset int32) ([]Comment, error) {
	rows, err := r.q(ctx).ListCommentsByPost(ctx, dbgen.ListCommentsByPostParams{PostID: postID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, fmt.Errorf("list_comments_by_post: %w", err)
	}
	result := make([]Comment, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapComment(row))
	}
	return result, nil
}

func (r *repository) GetForUpdate(ctx context.Context, id, postID uuid.UUID) (Comment, error) {
	row, err := r.q(ctx).GetCommentByIDForUpdate(ctx, dbgen.GetCommentByIDForUpdateParams{ID: id, PostID: postID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Comment{}, ErrCommentNotFound
	}
	if err != nil {
		return Comment{}, fmt.Errorf("get_comment_for_update: %w", err)
	}
	return mapComment(row), nil
}

func (r *repository) Update(ctx context.Context, id, postID uuid.UUID, content string) (Comment, error) {
	row, err := r.q(ctx).UpdateComment(ctx, dbgen.UpdateCommentParams{ID: id, PostID: postID, Content: content})
	if errors.Is(err, pgx.ErrNoRows) {
		return Comment{}, ErrCommentNotFound
	}
	if err != nil {
		return Comment{}, fmt.Errorf("update_comment: %w", err)
	}
	return mapComment(row), nil
}

func (r *repository) SoftDelete(ctx context.Context, id, postID uuid.UUID) error {
	_, err := r.q(ctx).SoftDeleteComment(ctx, dbgen.SoftDeleteCommentParams{ID: id, PostID: postID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCommentNotFound
	}
	if err != nil {
		return fmt.Errorf("soft_delete_comment: %w", err)
	}
	return nil
}

func mapComment(row dbgen.Comment) Comment {
	var deletedAt *time.Time
	if row.DeletedAt.Valid {
		deletedAt = &row.DeletedAt.Time
	}
	return Comment{ID: row.ID, PostID: row.PostID, AuthorID: row.AuthorID, Content: row.Content, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time, DeletedAt: deletedAt}
}
