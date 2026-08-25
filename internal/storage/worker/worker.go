package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emanuelfelicio/artblogapi/internal/storage"
	"github.com/emanuelfelicio/artblogapi/internal/storage/image"
	"github.com/google/uuid"
)

type Config struct {
	Concurrency       int
	TickerInterval    time.Duration
	StaleThreshold    time.Duration
	HeartbeatInterval time.Duration
	BackoffInterval   time.Duration
	MaxRetries        int
}

type Repository interface {
	GetNextProcessingJob(ctx context.Context, maxRetries int, staleThreshold time.Duration) (storage.Upload, bool, error)
	HeartbeatUploadProcessing(ctx context.Context, id uuid.UUID) error
	UpdateUploadCompletion(ctx context.Context, id uuid.UUID, objectKey string, contentType storage.ImageContentType, status storage.UploadStatus) error
	RejectUpload(ctx context.Context, id uuid.UUID, reason string) error
	IncrementRetry(ctx context.Context, id uuid.UUID, backoff time.Duration) error
}

type Worker struct {
	repo        Repository
	provider    storage.StorageProvider
	processor   image.ImageProcessor
	triggerChan chan struct{}
	cfg         Config
	logger      *slog.Logger
	wg          sync.WaitGroup
}

func New(
	repo Repository,
	provider storage.StorageProvider,
	processor image.ImageProcessor,
	triggerChan chan struct{},
	cfg Config,
	logger *slog.Logger,
) *Worker {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 3
	}
	if cfg.TickerInterval <= 0 {
		cfg.TickerInterval = 60 * time.Second
	}
	if cfg.StaleThreshold <= 0 {
		cfg.StaleThreshold = 30 * time.Second
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 10 * time.Second
	}
	if cfg.BackoffInterval <= 0 {
		cfg.BackoffInterval = 10 * time.Second
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}

	return &Worker{
		repo:        repo,
		provider:    provider,
		processor:   processor,
		triggerChan: triggerChan,
		cfg:         cfg,
		logger:      logger,
	}
}

// Enqueue implements storage.UploadProcessor. In-memory worker triggers a non-blocking
// notification on triggerChan.
func (w *Worker) Enqueue(_ context.Context, _ uuid.UUID) error {
	select {
	case w.triggerChan <- struct{}{}:
	default:
	}
	return nil
}

func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.TickerInterval)
	defer ticker.Stop()

	// Spawn N worker goroutines for parallel processing (Drain Loop)
	for i := 0; i < w.cfg.Concurrency; i++ {
		w.wg.Add(1)
		go func(workerID int) {
			defer w.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-w.triggerChan:
					for {
						if ctx.Err() != nil {
							return
						}
						found, err := w.processNextJob(ctx)
						if err != nil {
							w.logger.Error("worker_process_job_error",
								slog.Int("worker_id", workerID),
								slog.String("error", err.Error()),
							)
						}
						if !found {
							break
						}
					}
				}
			}
		}(i)
	}

	// Resiliency Ticker: Triggers idle workers when system has no incoming uploads
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			select {
			case w.triggerChan <- struct{}{}:
			default:
			}
		}
	}
}

func (w *Worker) Wait() {
	w.wg.Wait()
}

func (w *Worker) processNextJob(ctx context.Context) (bool, error) {
	job, found, err := w.repo.GetNextProcessingJob(ctx, w.cfg.MaxRetries, w.cfg.StaleThreshold)
	if err != nil {
		return false, fmt.Errorf("get_next_processing_job: %w", err)
	}
	if !found {
		return false, nil
	}

	if err := w.runPipeline(ctx, job); err != nil {
		if handleErr := w.handleJobError(ctx, job, err.Error()); handleErr != nil {
			w.logger.Error("handle_job_error_failed",
				slog.String("upload_id", job.ID.String()),
				slog.String("error", handleErr.Error()),
			)
		}
		return true, err
	}

	return true, nil
}

func (w *Worker) runPipeline(ctx context.Context, job storage.Upload) error {
	// Active Heartbeat: Renews heartbeat_at = now()
	heartbeatCtx, stopHeartbeat := context.WithCancel(ctx)
	defer stopHeartbeat()

	go func() {
		hbTicker := time.NewTicker(w.cfg.HeartbeatInterval)
		defer hbTicker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-hbTicker.C:
				if err := w.repo.HeartbeatUploadProcessing(heartbeatCtx, job.ID); err != nil {
					w.logger.Warn("heartbeat_failed",
						slog.String("upload_id", job.ID.String()),
						slog.String("error", err.Error()),
					)
				}
			}
		}
	}()

	quarantineKey := storage.BuildQuarantineKey(job.ID)
	existsQuarantine, err := w.provider.ObjectExists(ctx, quarantineKey)
	if err != nil {
		return fmt.Errorf("check_quarantine_exists: %w", err)
	}

	finalKey := storage.BuildFinalKey(job.ID)

	// Idempotency check: if quarantine file is gone, check if already completed in final bucket
	if !existsQuarantine {
		if existsFinal, _ := w.provider.ObjectExists(ctx, finalKey); existsFinal {
			return w.repo.UpdateUploadCompletion(ctx, job.ID, finalKey, job.ContentType, storage.UploadStatusCOMPLETED)
		}
		return errors.New("physical_file_not_found")
	}

	objReader, err := w.provider.GetObject(ctx, quarantineKey)
	if err != nil {
		return fmt.Errorf("get_quarantine_object: %w", err)
	}
	defer objReader.Close()

	processed, err := w.processor.Process(ctx, objReader, job.Purpose)
	if err != nil {
		return fmt.Errorf("process_image: %w", err)
	}

	if err := w.provider.PutObject(ctx, finalKey, bytes.NewReader(processed.Data), string(processed.ContentType)); err != nil {
		return fmt.Errorf("put_final_object: %w", err)
	}

	if err := w.provider.DeleteObject(ctx, quarantineKey); err != nil {
		w.logger.Warn("delete_quarantine_failed",
			slog.String("upload_id", job.ID.String()),
			slog.String("key", quarantineKey),
			slog.String("error", err.Error()),
		)
	}

	if err := w.repo.UpdateUploadCompletion(ctx, job.ID, finalKey, processed.ContentType, storage.UploadStatusCOMPLETED); err != nil {
		return fmt.Errorf("complete_upload_status: %w", err)
	}

	return nil
}

func (w *Worker) handleJobError(ctx context.Context, job storage.Upload, reason string) error {
	if job.RetryCount+1 >= w.cfg.MaxRetries {
		w.logger.Warn("upload_rejected_max_retries_reached",
			slog.String("upload_id", job.ID.String()),
			slog.Int("retry_count", job.RetryCount+1),
			slog.String("reason", reason),
		)
		return w.repo.RejectUpload(ctx, job.ID, reason)
	}

	backoff := time.Duration(job.RetryCount+1) * w.cfg.BackoffInterval

	w.logger.Info("upload_scheduled_retry",
		slog.String("upload_id", job.ID.String()),
		slog.Int("next_retry", job.RetryCount+1),
		slog.Duration("backoff", backoff),
	)

	return w.repo.IncrementRetry(ctx, job.ID, backoff)
}
