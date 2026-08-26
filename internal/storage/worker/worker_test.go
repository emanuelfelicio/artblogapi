package worker

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emanuelfelicio/artblogapi/internal/storage"
	"github.com/emanuelfelicio/artblogapi/internal/storage/image"
	"github.com/google/uuid"
)

// --- STUBS ---

type stubRepository struct {
	getNextProcessingJob      func(ctx context.Context, maxRetries int, staleThreshold time.Duration) (storage.Upload, bool, error)
	heartbeatUploadProcessing func(ctx context.Context, id uuid.UUID) error
	updateUploadCompletion    func(ctx context.Context, id uuid.UUID, objectKey string, contentType storage.ImageContentType, status storage.UploadStatus) error
	rejectUpload              func(ctx context.Context, id uuid.UUID, reason string) error
	incrementRetry            func(ctx context.Context, id uuid.UUID, backoff time.Duration) error
}

func (s *stubRepository) GetNextProcessingJob(ctx context.Context, maxRetries int, staleThreshold time.Duration) (storage.Upload, bool, error) {
	if s.getNextProcessingJob != nil {
		return s.getNextProcessingJob(ctx, maxRetries, staleThreshold)
	}
	return storage.Upload{}, false, nil
}

func (s *stubRepository) HeartbeatUploadProcessing(ctx context.Context, id uuid.UUID) error {
	if s.heartbeatUploadProcessing != nil {
		return s.heartbeatUploadProcessing(ctx, id)
	}
	return nil
}

func (s *stubRepository) UpdateUploadCompletion(ctx context.Context, id uuid.UUID, objectKey string, contentType storage.ImageContentType, status storage.UploadStatus) error {
	if s.updateUploadCompletion != nil {
		return s.updateUploadCompletion(ctx, id, objectKey, contentType, status)
	}
	return nil
}

func (s *stubRepository) RejectUpload(ctx context.Context, id uuid.UUID, reason string) error {
	if s.rejectUpload != nil {
		return s.rejectUpload(ctx, id, reason)
	}
	return nil
}

func (s *stubRepository) IncrementRetry(ctx context.Context, id uuid.UUID, backoff time.Duration) error {
	if s.incrementRetry != nil {
		return s.incrementRetry(ctx, id, backoff)
	}
	return nil
}

type stubStorageProvider struct {
	generateUploadURL func(ctx context.Context, key string, expires time.Duration) (string, error)
	getObject         func(ctx context.Context, key string) (io.ReadCloser, error)
	putObject         func(ctx context.Context, key string, reader io.Reader, contentType string) error
	deleteObject      func(ctx context.Context, key string) error
	objectExists      func(ctx context.Context, key string) (bool, error)
}

func (s *stubStorageProvider) GenerateUploadURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	if s.generateUploadURL != nil {
		return s.generateUploadURL(ctx, key, expires)
	}
	return "", nil
}

func (s *stubStorageProvider) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	if s.getObject != nil {
		return s.getObject(ctx, key)
	}
	return io.NopCloser(bytes.NewReader([]byte("fake image data"))), nil
}

func (s *stubStorageProvider) PutObject(ctx context.Context, key string, reader io.Reader, contentType string) error {
	if s.putObject != nil {
		return s.putObject(ctx, key, reader, contentType)
	}
	return nil
}

func (s *stubStorageProvider) DeleteObject(ctx context.Context, key string) error {
	if s.deleteObject != nil {
		return s.deleteObject(ctx, key)
	}
	return nil
}

func (s *stubStorageProvider) ObjectExists(ctx context.Context, key string) (bool, error) {
	if s.objectExists != nil {
		return s.objectExists(ctx, key)
	}
	return false, nil
}

type stubImageProcessor struct {
	process func(ctx context.Context, r io.Reader, purpose storage.UploadPurpose) (image.ProcessedImage, error)
}

func (s *stubImageProcessor) Process(ctx context.Context, r io.Reader, purpose storage.UploadPurpose) (image.ProcessedImage, error) {
	if s.process != nil {
		return s.process(ctx, r, purpose)
	}
	return image.ProcessedImage{
		Data:        []byte("processed data"),
		ContentType: storage.ContentTypePNG,
	}, nil
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- TESTS ---

func TestNew_DefaultsAndCustomConfig(t *testing.T) {
	t.Run("default configuration values applied when zero", func(t *testing.T) {
		w := New(&stubRepository{}, &stubStorageProvider{}, nil, make(chan struct{}, 1), Config{}, newTestLogger())

		if w.cfg.Concurrency != 3 {
			t.Errorf("expected Concurrency 3, got %d", w.cfg.Concurrency)
		}
		if w.cfg.TickerInterval != 60*time.Second {
			t.Errorf("expected TickerInterval 60s, got %v", w.cfg.TickerInterval)
		}
		if w.cfg.StaleThreshold != 30*time.Second {
			t.Errorf("expected StaleThreshold 30s, got %v", w.cfg.StaleThreshold)
		}
		if w.cfg.HeartbeatInterval != 10*time.Second {
			t.Errorf("expected HeartbeatInterval 10s, got %v", w.cfg.HeartbeatInterval)
		}
		if w.cfg.BackoffInterval != 10*time.Second {
			t.Errorf("expected BackoffInterval 10s, got %v", w.cfg.BackoffInterval)
		}
		if w.cfg.MaxRetries != 3 {
			t.Errorf("expected MaxRetries 3, got %d", w.cfg.MaxRetries)
		}
		if w.processor == nil {
			t.Error("expected default processor, got nil")
		}
	})

	t.Run("custom configuration values preserved", func(t *testing.T) {
		cfg := Config{
			Concurrency:       5,
			TickerInterval:    100 * time.Millisecond,
			StaleThreshold:    5 * time.Second,
			HeartbeatInterval: 1 * time.Second,
			BackoffInterval:   2 * time.Second,
			MaxRetries:        10,
		}
		proc := &stubImageProcessor{}
		w := New(&stubRepository{}, &stubStorageProvider{}, proc, make(chan struct{}, 1), cfg, newTestLogger())

		if w.cfg.Concurrency != 5 {
			t.Errorf("expected Concurrency 5, got %d", w.cfg.Concurrency)
		}
		if w.cfg.TickerInterval != 100*time.Millisecond {
			t.Errorf("expected TickerInterval 100ms, got %v", w.cfg.TickerInterval)
		}
		if w.cfg.StaleThreshold != 5*time.Second {
			t.Errorf("expected StaleThreshold 5s, got %v", w.cfg.StaleThreshold)
		}
		if w.cfg.HeartbeatInterval != 1*time.Second {
			t.Errorf("expected HeartbeatInterval 1s, got %v", w.cfg.HeartbeatInterval)
		}
		if w.cfg.BackoffInterval != 2*time.Second {
			t.Errorf("expected BackoffInterval 2s, got %v", w.cfg.BackoffInterval)
		}
		if w.cfg.MaxRetries != 10 {
			t.Errorf("expected MaxRetries 10, got %d", w.cfg.MaxRetries)
		}
		if w.processor != proc {
			t.Error("expected custom processor")
		}
	})
}

func TestWorker_Enqueue(t *testing.T) {
	t.Run("signals trigger channel", func(t *testing.T) {
		ch := make(chan struct{}, 1)
		w := New(&stubRepository{}, &stubStorageProvider{}, &stubImageProcessor{}, ch, Config{}, newTestLogger())

		err := w.Enqueue(context.Background(), uuid.New())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		select {
		case <-ch:
		default:
			t.Fatal("expected signal on triggerChan")
		}
	})

	t.Run("does not block when trigger channel is full", func(t *testing.T) {
		ch := make(chan struct{}, 1)
		ch <- struct{}{}

		w := New(&stubRepository{}, &stubStorageProvider{}, &stubImageProcessor{}, ch, Config{}, newTestLogger())

		done := make(chan error, 1)
		go func() {
			done <- w.Enqueue(context.Background(), uuid.New())
		}()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Enqueue bloqueou — provavelmente virou um envio bloqueante no channel")
		}
	})
}

func TestWorker_StartAndLifecycle(t *testing.T) {
	t.Run("drains jobs and shuts down gracefully on context cancellation", func(t *testing.T) {
		var processedCount int32
		// Channel used by the mock to signal job processing completion
		jobDone := make(chan struct{})

		repo := &stubRepository{
			getNextProcessingJob: func(_ context.Context, _ int, _ time.Duration) (storage.Upload, bool, error) {
				count := atomic.AddInt32(&processedCount, 1)
				// 1st call returns a pending job; subsequent calls indicate empty queue (false)
				if count == 1 {
					return storage.Upload{
						ID:          uuid.New(),
						Purpose:     storage.PurposeAVATAR,
						ContentType: storage.ContentTypePNG,
					}, true, nil
				}
				return storage.Upload{}, false, nil
			},
			updateUploadCompletion: func(_ context.Context, _ uuid.UUID, _ string, _ storage.ImageContentType, _ storage.UploadStatus) error {
				// When worker completes the upload, close jobDone channel to notify the test
				select {
				case <-jobDone:
				default:
					close(jobDone)
				}
				return nil
			},
		}

		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, key string) (bool, error) {
				// Simulate that file exists in quarantine bucket
				return strings.HasPrefix(key, "quarantine/"), nil
			},
		}

		ch := make(chan struct{}, 10)
		cfg := Config{
			Concurrency:    2,
			TickerInterval: 1 * time.Hour, // Long ticker interval to avoid test noise
		}
		w := New(repo, provider, &stubImageProcessor{}, ch, cfg, newTestLogger())

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start worker in background (Start blocks until context cancellation)
		done := make(chan struct{})
		go func() {
			w.Start(ctx)
			close(done)
		}()

		// Send trigger signal for worker to wake up and process the job
		_ = w.Enqueue(ctx, uuid.New())

		// Block and wait until worker signals completion via jobDone (with 2s fallback timeout)
		select {
		case <-jobDone:
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for worker to process job")
		}

		// Cancel context and verify graceful shutdown
		cancel()

		w.Wait() // Wait for worker goroutines to exit
		<-done   // Wait for main Start loop to exit

		if atomic.LoadInt32(&processedCount) < 1 {
			t.Error("expected at least 1 job check")
		}
	})
}

func TestWorker_ProcessNextJob(t *testing.T) {
	t.Run("returns error when GetNextProcessingJob fails", func(t *testing.T) {
		expectedErr := errors.New("db query timeout")
		repo := &stubRepository{
			getNextProcessingJob: func(_ context.Context, _ int, _ time.Duration) (storage.Upload, bool, error) {
				return storage.Upload{}, false, expectedErr
			},
		}

		w := New(repo, &stubStorageProvider{}, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		found, err := w.processNextJob(context.Background())

		if found {
			t.Error("expected found to be false")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("returns false and nil when no job is found", func(t *testing.T) {
		repo := &stubRepository{
			getNextProcessingJob: func(_ context.Context, _ int, _ time.Duration) (storage.Upload, bool, error) {
				return storage.Upload{}, false, nil
			},
		}

		w := New(repo, &stubStorageProvider{}, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		found, err := w.processNextJob(context.Background())

		if found {
			t.Error("expected found to be false")
		}
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("handles pipeline failure by invoking handleJobError", func(t *testing.T) {
		uploadID := uuid.New()
		var rejectedReason string
		repo := &stubRepository{
			getNextProcessingJob: func(_ context.Context, _ int, _ time.Duration) (storage.Upload, bool, error) {
				return storage.Upload{
					ID:          uploadID,
					RetryCount:  5,
					Purpose:     storage.PurposeAVATAR,
					ContentType: storage.ContentTypePNG,
				}, true, nil
			},
			rejectUpload: func(_ context.Context, id uuid.UUID, reason string) error {
				rejectedReason = reason
				return nil
			},
		}

		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, _ string) (bool, error) {
				return false, nil // Neither quarantine nor final -> ErrFileNotFound
			},
		}

		w := New(repo, provider, &stubImageProcessor{}, make(chan struct{}, 1), Config{MaxRetries: 3}, newTestLogger())
		found, err := w.processNextJob(context.Background())

		if !found {
			t.Error("expected found to be true")
		}
		if !errors.Is(err, ErrFileNotFound) {
			t.Errorf("expected ErrFileNotFound, got %v", err)
		}
		if rejectedReason != ErrFileNotFound.Error() {
			t.Errorf("expected rejectedReason %q, got %q", ErrFileNotFound.Error(), rejectedReason)
		}
	})
}

func TestWorker_RunPipeline(t *testing.T) {
	uploadID := uuid.New()
	job := storage.Upload{
		ID:          uploadID,
		Purpose:     storage.PurposeAVATAR,
		ContentType: storage.ContentTypePNG,
	}

	t.Run("successful processing pipeline", func(t *testing.T) {
		var updateCompleted bool
		var quarantineDeleted bool

		repo := &stubRepository{
			updateUploadCompletion: func(_ context.Context, id uuid.UUID, key string, ct storage.ImageContentType, status storage.UploadStatus) error {
				if id == uploadID && status == storage.UploadStatusCOMPLETED {
					updateCompleted = true
				}
				return nil
			},
		}

		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, key string) (bool, error) {
				if key == storage.BuildQuarantineKey(uploadID) {
					return true, nil
				}
				return false, nil
			},
			deleteObject: func(_ context.Context, key string) error {
				if key == storage.BuildQuarantineKey(uploadID) {
					quarantineDeleted = true
				}
				return nil
			},
		}

		w := New(repo, provider, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.runPipeline(context.Background(), job)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !quarantineDeleted {
			t.Error("expected quarantine object to be deleted")
		}
		if !updateCompleted {
			t.Error("expected update upload completion to be called")
		}
	})

	t.Run("idempotency check when both quarantine and final exist", func(t *testing.T) {
		var quarantineDeleted bool
		var completedStatusUpdated bool

		repo := &stubRepository{
			updateUploadCompletion: func(_ context.Context, _ uuid.UUID, _ string, _ storage.ImageContentType, status storage.UploadStatus) error {
				completedStatusUpdated = (status == storage.UploadStatusCOMPLETED)
				return nil
			},
		}

		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
			deleteObject: func(_ context.Context, _ string) error {
				quarantineDeleted = true
				return nil
			},
		}

		w := New(repo, provider, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.runPipeline(context.Background(), job)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !quarantineDeleted {
			t.Error("expected quarantine to be deleted during idempotency check")
		}
		if !completedStatusUpdated {
			t.Error("expected upload completion status to be updated")
		}
	})

	t.Run("idempotency check when only final exists", func(t *testing.T) {
		var completedStatusUpdated bool

		repo := &stubRepository{
			updateUploadCompletion: func(_ context.Context, _ uuid.UUID, _ string, _ storage.ImageContentType, status storage.UploadStatus) error {
				completedStatusUpdated = (status == storage.UploadStatusCOMPLETED)
				return nil
			},
		}

		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, key string) (bool, error) {
				return key == storage.BuildFinalKey(uploadID), nil
			},
		}

		w := New(repo, provider, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.runPipeline(context.Background(), job)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !completedStatusUpdated {
			t.Error("expected completion status update")
		}
	})

	t.Run("returns ErrFileNotFound when neither quarantine nor final exist", func(t *testing.T) {
		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, _ string) (bool, error) {
				return false, nil
			},
		}

		w := New(&stubRepository{}, provider, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.runPipeline(context.Background(), job)

		if !errors.Is(err, ErrFileNotFound) {
			t.Errorf("expected ErrFileNotFound, got %v", err)
		}
	})

	t.Run("returns error when quarantine check fails", func(t *testing.T) {
		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, key string) (bool, error) {
				if key == storage.BuildQuarantineKey(uploadID) {
					return false, errors.New("s3 connection failed")
				}
				return false, nil
			},
		}

		w := New(&stubRepository{}, provider, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.runPipeline(context.Background(), job)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when final check fails", func(t *testing.T) {
		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, key string) (bool, error) {
				if key == storage.BuildFinalKey(uploadID) {
					return false, errors.New("s3 check error")
				}
				return true, nil
			},
		}

		w := New(&stubRepository{}, provider, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.runPipeline(context.Background(), job)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error when GetObject fails", func(t *testing.T) {
		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, key string) (bool, error) {
				return key == storage.BuildQuarantineKey(uploadID), nil
			},
			getObject: func(_ context.Context, _ string) (io.ReadCloser, error) {
				return nil, errors.New("object read error")
			},
		}

		w := New(&stubRepository{}, provider, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.runPipeline(context.Background(), job)

		if err == nil {
			t.Fatal("expected error on GetObject failure")
		}
	})

	t.Run("returns error when ImageProcessor.Process fails", func(t *testing.T) {
		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, key string) (bool, error) {
				return key == storage.BuildQuarantineKey(uploadID), nil
			},
		}

		proc := &stubImageProcessor{
			process: func(_ context.Context, _ io.Reader, _ storage.UploadPurpose) (image.ProcessedImage, error) {
				return image.ProcessedImage{}, errors.New("corrupt image data")
			},
		}

		w := New(&stubRepository{}, provider, proc, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.runPipeline(context.Background(), job)

		if err == nil {
			t.Fatal("expected error on image process failure")
		}
	})

	t.Run("returns error when PutObject fails", func(t *testing.T) {
		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, key string) (bool, error) {
				return key == storage.BuildQuarantineKey(uploadID), nil
			},
			putObject: func(_ context.Context, _ string, _ io.Reader, _ string) error {
				return errors.New("s3 write permission denied")
			},
		}

		w := New(&stubRepository{}, provider, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.runPipeline(context.Background(), job)

		if err == nil {
			t.Fatal("expected error on PutObject failure")
		}
	})

	t.Run("returns delete_quarantine_failed error when DeleteObject fails", func(t *testing.T) {
		expectedErr := errors.New("delete object timeout")
		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, key string) (bool, error) {
				return key == storage.BuildQuarantineKey(uploadID), nil
			},
			deleteObject: func(_ context.Context, _ string) error {
				return expectedErr
			},
		}

		w := New(&stubRepository{}, provider, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.runPipeline(context.Background(), job)

		if err == nil {
			t.Fatal("expected error when DeleteObject fails")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected wrapped error %v, got %v", expectedErr, err)
		}
	})

	t.Run("returns error when UpdateUploadCompletion fails", func(t *testing.T) {
		repo := &stubRepository{
			updateUploadCompletion: func(_ context.Context, _ uuid.UUID, _ string, _ storage.ImageContentType, _ storage.UploadStatus) error {
				return errors.New("database commit failed")
			},
		}

		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, key string) (bool, error) {
				return key == storage.BuildQuarantineKey(uploadID), nil
			},
		}

		w := New(repo, provider, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.runPipeline(context.Background(), job)

		if err == nil {
			t.Fatal("expected error when UpdateUploadCompletion fails")
		}
	})

	t.Run("triggers active heartbeat during processing", func(t *testing.T) {
		var heartbeatCount int32
		var wg sync.WaitGroup
		wg.Add(1)

		repo := &stubRepository{
			heartbeatUploadProcessing: func(_ context.Context, _ uuid.UUID) error {
				atomic.AddInt32(&heartbeatCount, 1)
				wg.Done()
				return nil
			},
		}

		provider := &stubStorageProvider{
			objectExists: func(_ context.Context, key string) (bool, error) {
				return key == storage.BuildQuarantineKey(uploadID), nil
			},
		}

		proc := &stubImageProcessor{
			process: func(_ context.Context, _ io.Reader, _ storage.UploadPurpose) (image.ProcessedImage, error) {
				wg.Wait()
				return image.ProcessedImage{Data: []byte("ok"), ContentType: storage.ContentTypePNG}, nil
			},
		}

		cfg := Config{HeartbeatInterval: 10 * time.Millisecond}
		w := New(repo, provider, proc, make(chan struct{}, 1), cfg, newTestLogger())

		err := w.runPipeline(context.Background(), job)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if atomic.LoadInt32(&heartbeatCount) < 1 {
			t.Error("expected at least 1 heartbeat call")
		}
	})
}

func TestWorker_HandleJobError(t *testing.T) {
	uploadID := uuid.New()
	job := storage.Upload{
		ID:         uploadID,
		RetryCount: 1,
	}

	t.Run("rejects immediately on ErrFileNotFound", func(t *testing.T) {
		var rejectedReason string
		repo := &stubRepository{
			rejectUpload: func(_ context.Context, id uuid.UUID, reason string) error {
				rejectedReason = reason
				return nil
			},
		}

		w := New(repo, &stubStorageProvider{}, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.handleJobError(context.Background(), job, ErrFileNotFound)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rejectedReason != ErrFileNotFound.Error() {
			t.Errorf("expected reason %q, got %q", ErrFileNotFound.Error(), rejectedReason)
		}
	})

	t.Run("increments retry count with backoff when under MaxRetries", func(t *testing.T) {
		var capturedBackoff time.Duration
		repo := &stubRepository{
			incrementRetry: func(_ context.Context, _ uuid.UUID, backoff time.Duration) error {
				capturedBackoff = backoff
				return nil
			},
		}

		cfg := Config{
			MaxRetries:      3,
			BackoffInterval: 10 * time.Second,
		}
		w := New(repo, &stubStorageProvider{}, &stubImageProcessor{}, make(chan struct{}, 1), cfg, newTestLogger())
		err := w.handleJobError(context.Background(), job, errors.New("temporary s3 glitch"))

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedBackoff := 20 * time.Second // (RetryCount 1 + 1) * 10s
		if capturedBackoff != expectedBackoff {
			t.Errorf("expected backoff %v, got %v", expectedBackoff, capturedBackoff)
		}
	})

	t.Run("rejects upload when MaxRetries is reached", func(t *testing.T) {
		maxJob := storage.Upload{
			ID:         uploadID,
			RetryCount: 2,
		}

		var rejected bool
		repo := &stubRepository{
			rejectUpload: func(_ context.Context, _ uuid.UUID, _ string) error {
				rejected = true
				return nil
			},
		}

		cfg := Config{
			MaxRetries: 3,
		}
		w := New(repo, &stubStorageProvider{}, &stubImageProcessor{}, make(chan struct{}, 1), cfg, newTestLogger())
		err := w.handleJobError(context.Background(), maxJob, errors.New("third failure"))

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !rejected {
			t.Error("expected upload to be rejected when max retries reached")
		}
	})

	t.Run("returns repository error if RejectUpload fails", func(t *testing.T) {
		expectedErr := errors.New("db error")
		repo := &stubRepository{
			rejectUpload: func(_ context.Context, _ uuid.UUID, _ string) error {
				return expectedErr
			},
		}

		w := New(repo, &stubStorageProvider{}, &stubImageProcessor{}, make(chan struct{}, 1), Config{}, newTestLogger())
		err := w.handleJobError(context.Background(), job, ErrFileNotFound)

		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})
}
