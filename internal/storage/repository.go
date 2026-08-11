package storage

import (
	"context"
	"fmt"

	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	q    *dbgen.Queries
	pool *pgxpool.Pool
}

func NewRepository(q *dbgen.Queries, pool *pgxpool.Pool) *repository {
	return &repository{q: q, pool: pool}
}

func (r *repository) Create(ctx context.Context, u Upload) (uuid.UUID, error) {

	uploadID, err := r.q.CreateUpload(ctx, dbgen.CreateUploadParams{
		ID:          u.ID,
		UserID:      u.UserID,
		ObjectKey:   u.ObjectKey,
		Status:      dbgen.UploadStatus(u.Status),
		Purpose:     dbgen.UploadPurpose(u.Purpose),
		FileSize:    pgtype.Int4{Int32: int32(u.FileSize), Valid: true},
		ContentType: pgtype.Text{String: string(u.ContentType), Valid: true},
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create_upload: %w", err)
	}
	return uploadID, nil
}
