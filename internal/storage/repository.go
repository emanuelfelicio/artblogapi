package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (Upload, error) {
	row, err := r.q.GetUploadByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Upload{}, ErrUploadNotFound
		}
		return Upload{}, fmt.Errorf("get_upload_by_id: %w", err)
	}

	upload := Upload{
		ID:        row.ID,
		UserID:    row.UserID,
		ObjectKey: row.ObjectKey,
		Status:    UploadStatus(row.Status),
		Purpose:   UploadPurpose(row.Purpose),
	}

	if row.FileSize.Valid {
		upload.FileSize = int(row.FileSize.Int32)
	}
	if row.ContentType.Valid {
		upload.ContentType = ImageContentType(row.ContentType.String)
	}
	if row.FailureReason.Valid {
		reason := row.FailureReason.String
		upload.FailureReason = &reason
	}
	if row.CreatedAt.Valid {
		upload.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		upload.UpdatedAt = row.UpdatedAt.Time
	}

	return upload, nil
}

func (r *repository) SetStatusProcessing(ctx context.Context, id uuid.UUID) error {
	err := r.q.UpdateUploadStatus(ctx, dbgen.UpdateUploadStatusParams{
		ID:     id,
		Status: dbgen.UploadStatusPROCESSING,
	})
	if err != nil {
		return fmt.Errorf("set_status_processing: %w", err)
	}
	return nil
}
