package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

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

func (r *repository) GetNextProcessingJob(ctx context.Context, maxRetries int, staleThreshold time.Duration) (Upload, bool, error) {
	row, err := r.q.GetNextProcessingJob(ctx, dbgen.GetNextProcessingJobParams{
		RetryCount: int32(maxRetries),
		Column2:    pgtype.Interval{Microseconds: staleThreshold.Microseconds(), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Upload{}, false, nil
		}
		return Upload{}, false, fmt.Errorf("get_next_processing_job: %w", err)
	}

	upload := Upload{
		ID:         row.ID,
		UserID:     row.UserID,
		ObjectKey:  row.ObjectKey,
		Status:     UploadStatusPROCESSING,
		Purpose:    UploadPurpose(row.Purpose),
		RetryCount: int(row.RetryCount),
	}
	if row.FileSize.Valid {
		upload.FileSize = int(row.FileSize.Int32)
	}
	if row.ContentType.Valid {
		upload.ContentType = ImageContentType(row.ContentType.String)
	}

	return upload, true, nil
}

func (r *repository) HeartbeatUploadProcessing(ctx context.Context, id uuid.UUID) error {
	if err := r.q.HeartbeatUploadProcessing(ctx, id); err != nil {
		return fmt.Errorf("heartbeat_upload_processing: %w", err)
	}
	return nil
}

func (r *repository) UpdateUploadCompletion(ctx context.Context, id uuid.UUID, objectKey string, contentType ImageContentType, status UploadStatus) error {
	err := r.q.UpdateUploadCompletion(ctx, dbgen.UpdateUploadCompletionParams{
		ID:          id,
		ObjectKey:   objectKey,
		ContentType: pgtype.Text{String: string(contentType), Valid: true},
		Status:      dbgen.UploadStatus(status),
	})
	if err != nil {
		return fmt.Errorf("update_upload_completion: %w", err)
	}
	return nil
}

func (r *repository) RejectUpload(ctx context.Context, id uuid.UUID, reason string) error {
	err := r.q.RejectUpload(ctx, dbgen.RejectUploadParams{
		ID:            id,
		FailureReason: pgtype.Text{String: reason, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("reject_upload: %w", err)
	}
	return nil
}

func (r *repository) IncrementRetry(ctx context.Context, id uuid.UUID, backoff time.Duration) error {
	err := r.q.IncrementUploadRetry(ctx, dbgen.IncrementUploadRetryParams{
		ID:      id,
		Column2: pgtype.Interval{Microseconds: backoff.Microseconds(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("increment_upload_retry: %w", err)
	}
	return nil
}
