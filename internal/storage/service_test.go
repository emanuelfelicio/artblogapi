package storage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- STUBS ---

type stubRepository struct {
	create                  func(ctx context.Context, u Upload) (uuid.UUID, error)
	getByID                 func(ctx context.Context, id uuid.UUID) (Upload, error)
	setStatusProcessing     func(ctx context.Context, id uuid.UUID) error
	findUploadByIDForUpdate func(ctx context.Context, id uuid.UUID) (Upload, error)
	updateUploadStatus      func(ctx context.Context, id uuid.UUID, status UploadStatus) error
}

func (s *stubRepository) Create(ctx context.Context, u Upload) (uuid.UUID, error) {
	if s.create != nil {
		return s.create(ctx, u)
	}
	return u.ID, nil
}

func (s *stubRepository) GetByID(ctx context.Context, id uuid.UUID) (Upload, error) {
	if s.getByID != nil {
		return s.getByID(ctx, id)
	}
	return Upload{}, nil
}

func (s *stubRepository) SetStatusProcessing(ctx context.Context, id uuid.UUID) error {
	if s.setStatusProcessing != nil {
		return s.setStatusProcessing(ctx, id)
	}
	return nil
}

func (s *stubRepository) FindUploadByIDForUpdate(ctx context.Context, id uuid.UUID) (Upload, error) {
	if s.findUploadByIDForUpdate != nil {
		return s.findUploadByIDForUpdate(ctx, id)
	}
	return Upload{}, nil
}

func (s *stubRepository) UpdateUploadStatus(ctx context.Context, id uuid.UUID, status UploadStatus) error {
	if s.updateUploadStatus != nil {
		return s.updateUploadStatus(ctx, id, status)
	}
	return nil
}

type stubStorageProvider struct {
	generateUploadURL func(ctx context.Context, key string, contentType ImageContentType, expires time.Duration) (string, error)
	objectExists      func(ctx context.Context, key string) (bool, error)
}

func (s *stubStorageProvider) GenerateUploadURL(ctx context.Context, key string, contentType ImageContentType, expires time.Duration) (string, error) {
	if s.generateUploadURL != nil {
		return s.generateUploadURL(ctx, key, contentType, expires)
	}
	return "https://s3.example.com/presigned", nil
}

func (s *stubStorageProvider) GetObject(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, nil
}

func (s *stubStorageProvider) PutObject(_ context.Context, _ string, _ io.Reader, _ string) error {
	return nil
}

func (s *stubStorageProvider) DeleteObject(_ context.Context, _ string) error {
	return nil
}

func (s *stubStorageProvider) ObjectExists(ctx context.Context, key string) (bool, error) {
	if s.objectExists != nil {
		return s.objectExists(ctx, key)
	}
	return false, nil
}

type stubUploadProcessor struct {
	enqueue func(ctx context.Context, id uuid.UUID) error
}

func (s *stubUploadProcessor) Enqueue(ctx context.Context, id uuid.UUID) error {
	if s.enqueue != nil {
		return s.enqueue(ctx, id)
	}
	return nil
}

// --- HELPERS ---

func newTestService(repo Repository, provider StorageProvider) *Service {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	worker := &stubUploadProcessor{}
	return NewService(repo, provider, worker, 15*time.Minute, logger)
}

func newTestServiceWithProcessor(repo Repository, provider StorageProvider, processor UploadProcessor) *Service {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(repo, provider, processor, 15*time.Minute, logger)
}

// --- TESTS ---

func TestService_InitUpload(t *testing.T) {
	userID := uuid.New()
	ctx := context.Background()

	tests := []struct {
		name        string
		purpose     UploadPurpose
		fileSize    int
		contentType ImageContentType
		repo        *stubRepository
		provider    *stubStorageProvider
		wantErr     error
	}{
		{
			name:        "success with AVATAR purpose",
			purpose:     "AVATAR",
			fileSize:    1024,
			contentType: "image/png",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
		},
		{
			name:        "success with BANNER purpose",
			purpose:     "BANNER",
			fileSize:    2048,
			contentType: "image/jpeg",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
		},
		{
			name:        "success with POST_IMAGE purpose",
			purpose:     "POST_IMAGE",
			fileSize:    4096,
			contentType: "image/webp",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
		},
		{
			name:        "invalid purpose",
			purpose:     "INVALID",
			fileSize:    1024,
			contentType: "image/png",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
			wantErr:     ErrInvalidPurpose,
		},
		{
			name:        "empty purpose",
			purpose:     "",
			fileSize:    1024,
			contentType: "image/png",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
			wantErr:     ErrInvalidPurpose,
		},
		{
			name:        "file size zero",
			purpose:     "AVATAR",
			fileSize:    0,
			contentType: "image/png",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
			wantErr:     ErrInvalidFileSize,
		},
		{
			name:        "file size negative",
			purpose:     "AVATAR",
			fileSize:    -1,
			contentType: "image/png",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
			wantErr:     ErrInvalidFileSize,
		},
		{
			name:        "AVATAR file size exceeds 5MiB limit",
			purpose:     "AVATAR",
			fileSize:    5<<20 + 1,
			contentType: "image/png",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
			wantErr:     ErrFileSizeExceeded,
		},
		{
			name:        "AVATAR file size at exact 5MiB limit succeeds",
			purpose:     "AVATAR",
			fileSize:    5 << 20,
			contentType: "image/png",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
		},
		{
			name:        "BANNER file size exceeds 8MiB limit",
			purpose:     "BANNER",
			fileSize:    8<<20 + 1,
			contentType: "image/jpeg",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
			wantErr:     ErrFileSizeExceeded,
		},
		{
			name:        "POST_IMAGE file size exceeds 20MiB limit",
			purpose:     "POST_IMAGE",
			fileSize:    20<<20 + 1,
			contentType: "image/webp",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
			wantErr:     ErrFileSizeExceeded,
		},
		{
			name:        "invalid content type",
			purpose:     "AVATAR",
			fileSize:    1024,
			contentType: "application/pdf",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
			wantErr:     ErrInvaliImageContentType,
		},
		{
			name:        "empty content type",
			purpose:     "AVATAR",
			fileSize:    1024,
			contentType: "",
			repo:        &stubRepository{},
			provider:    &stubStorageProvider{},
			wantErr:     ErrInvaliImageContentType,
		},
		{
			name:        "provider failure",
			purpose:     "AVATAR",
			fileSize:    1024,
			contentType: "image/png",
			repo:        &stubRepository{},
			provider: &stubStorageProvider{
				generateUploadURL: func(_ context.Context, _ string, _ ImageContentType, _ time.Duration) (string, error) {
					return "", errors.New("s3 unavailable")
				},
			},
			wantErr: errors.New("generate_upload_url"),
		},
		{
			name:        "repository failure",
			purpose:     "AVATAR",
			fileSize:    1024,
			contentType: "image/png",
			repo: &stubRepository{
				create: func(_ context.Context, _ Upload) (uuid.UUID, error) {
					return uuid.Nil, errors.New("db connection refused")
				},
			},
			provider: &stubStorageProvider{},
			wantErr:  errors.New("db connection refused"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestService(tc.repo, tc.provider)
			uploadID, presignedURL, err := svc.InitUpload(ctx, userID, tc.purpose, tc.fileSize, tc.contentType)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr.Error())
				}

				if errors.Is(tc.wantErr, ErrInvalidPurpose) ||
					errors.Is(tc.wantErr, ErrInvaliImageContentType) ||
					errors.Is(tc.wantErr, ErrInvalidFileSize) ||
					errors.Is(tc.wantErr, ErrFileSizeExceeded) {
					if !errors.Is(err, tc.wantErr) {
						t.Fatalf("expected error %v, got %v", tc.wantErr, err)
					}
				} else {
					if err.Error() == "" {
						t.Fatalf("expected non-empty error, got empty")
					}
				}

				if uploadID != uuid.Nil {
					t.Fatalf("expected uuid.Nil on error, got %s", uploadID)
				}
				if presignedURL != "" {
					t.Fatalf("expected empty presigned URL on error, got %s", presignedURL)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if uploadID == uuid.Nil {
				t.Fatal("expected non-nil upload ID")
			}
			if presignedURL == "" {
				t.Fatal("expected non-empty presigned URL")
			}
		})
	}
}

func TestService_InitUpload_ObjectKeyPrefixSpy(t *testing.T) {
	var capturedUpload Upload
	repo := &stubRepository{
		create: func(_ context.Context, u Upload) (uuid.UUID, error) {
			capturedUpload = u
			return u.ID, nil
		},
	}

	svc := newTestService(repo, &stubStorageProvider{})
	_, _, err := svc.InitUpload(context.Background(), uuid.New(), "AVATAR", 1024, "image/png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedUpload.ObjectKey) <= len(bucketPrefixQuarantine) {
		t.Fatal("object key should contain the quarantine prefix + upload ID")
	}

	prefix := capturedUpload.ObjectKey[:len(bucketPrefixQuarantine)]
	if prefix != bucketPrefixQuarantine {
		t.Fatalf("expected object key prefix %q, got %q", bucketPrefixQuarantine, prefix)
	}
}

func TestService_InitUpload_UploadFieldsSpy(t *testing.T) {
	var capturedUpload Upload
	repo := &stubRepository{
		create: func(_ context.Context, u Upload) (uuid.UUID, error) {
			capturedUpload = u
			return u.ID, nil
		},
	}

	userID := uuid.New()
	svc := newTestService(repo, &stubStorageProvider{})
	_, _, err := svc.InitUpload(context.Background(), userID, "BANNER", 2048, "image/jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedUpload.UserID != userID {
		t.Fatalf("expected UserID %s, got %s", userID, capturedUpload.UserID)
	}
	if capturedUpload.Status != UploadStatusPENDING {
		t.Fatalf("expected status %s, got %s", UploadStatusPENDING, capturedUpload.Status)
	}
	if capturedUpload.Purpose != PurposeBANNER {
		t.Fatalf("expected purpose %s, got %s", PurposeBANNER, capturedUpload.Purpose)
	}
	if capturedUpload.FileSize != 2048 {
		t.Fatalf("expected file size 2048, got %d", capturedUpload.FileSize)
	}
	if capturedUpload.ContentType != ContentTypeJPEG {
		t.Fatalf("expected content type %s, got %s", ContentTypeJPEG, capturedUpload.ContentType)
	}
}

func TestService_InitUpload_PresignTTLSpy(t *testing.T) {
	var capturedTTL time.Duration
	provider := &stubStorageProvider{
		generateUploadURL: func(_ context.Context, _ string, _ ImageContentType, expires time.Duration) (string, error) {
			capturedTTL = expires
			return "https://s3.example.com/presigned", nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	expectedTTL := 30 * time.Minute
	svc := NewService(&stubRepository{}, provider, &stubUploadProcessor{}, expectedTTL, logger)

	_, _, err := svc.InitUpload(context.Background(), uuid.New(), "AVATAR", 1024, "image/png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedTTL != expectedTTL {
		t.Fatalf("expected presign TTL %v, got %v", expectedTTL, capturedTTL)
	}
}

func TestService_CompleteUpload(t *testing.T) {
	ownerID := uuid.New()
	uploadID := uuid.New()
	ctx := context.Background()

	pendingUpload := Upload{
		ID:        uploadID,
		UserID:    ownerID,
		ObjectKey: bucketPrefixQuarantine + uploadID.String(),
		Status:    UploadStatusPENDING,
		Purpose:   PurposeAVATAR,
	}

	tests := []struct {
		name      string
		userID    uuid.UUID
		uploadID  uuid.UUID
		repo      *stubRepository
		provider  *stubStorageProvider
		processor *stubUploadProcessor
		wantErr   error
	}{
		{
			name:     "success",
			userID:   ownerID,
			uploadID: uploadID,
			repo: &stubRepository{
				getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return pendingUpload, nil
				},
			},
			provider: &stubStorageProvider{
				objectExists: func(_ context.Context, _ string) (bool, error) {
					return true, nil
				},
			},
			processor: &stubUploadProcessor{},
		},
		{
			name:     "upload not found",
			userID:   ownerID,
			uploadID: uuid.New(),
			repo: &stubRepository{
				getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return Upload{}, ErrUploadNotFound
				},
			},
			provider:  &stubStorageProvider{},
			processor: &stubUploadProcessor{},
			wantErr:   ErrUploadNotFound,
		},
		{
			name:     "upload belongs to another user",
			userID:   uuid.New(),
			uploadID: uploadID,
			repo: &stubRepository{
				getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return pendingUpload, nil
				},
			},
			provider:  &stubStorageProvider{},
			processor: &stubUploadProcessor{},
			wantErr:   ErrUploadNotOwned,
		},
		{
			name:     "upload not in PENDING status",
			userID:   ownerID,
			uploadID: uploadID,
			repo: &stubRepository{
				getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					u := pendingUpload
					u.Status = UploadStatusCOMPLETED
					return u, nil
				},
			},
			provider:  &stubStorageProvider{},
			processor: &stubUploadProcessor{},
			wantErr:   ErrUploadNotPending,
		},
		{
			name:     "file not in quarantine bucket",
			userID:   ownerID,
			uploadID: uploadID,
			repo: &stubRepository{
				getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return pendingUpload, nil
				},
			},
			provider: &stubStorageProvider{
				objectExists: func(_ context.Context, _ string) (bool, error) {
					return false, nil
				},
			},
			processor: &stubUploadProcessor{},
			wantErr:   ErrFileNotFound,
		},
		{
			name:     "storage provider ObjectExists error",
			userID:   ownerID,
			uploadID: uploadID,
			repo: &stubRepository{
				getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return pendingUpload, nil
				},
			},
			provider: &stubStorageProvider{
				objectExists: func(_ context.Context, _ string) (bool, error) {
					return false, errors.New("s3 timeout")
				},
			},
			processor: &stubUploadProcessor{},
			wantErr:   errors.New("check_quarantine_object"),
		},
		{
			name:     "repo SetStatusProcessing error",
			userID:   ownerID,
			uploadID: uploadID,
			repo: &stubRepository{
				getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return pendingUpload, nil
				},
				setStatusProcessing: func(_ context.Context, _ uuid.UUID) error {
					return errors.New("db write failed")
				},
			},
			provider: &stubStorageProvider{
				objectExists: func(_ context.Context, _ string) (bool, error) {
					return true, nil
				},
			},
			processor: &stubUploadProcessor{},
			wantErr:   errors.New("db write failed"),
		},
		{
			name:     "enqueue failure propagates error",
			userID:   ownerID,
			uploadID: uploadID,
			repo: &stubRepository{
				getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return pendingUpload, nil
				},
			},
			provider: &stubStorageProvider{
				objectExists: func(_ context.Context, _ string) (bool, error) {
					return true, nil
				},
			},
			processor: &stubUploadProcessor{
				enqueue: func(_ context.Context, _ uuid.UUID) error {
					return errors.New("channel full")
				},
			},
			wantErr: errors.New("enqueue_upload"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestServiceWithProcessor(tc.repo, tc.provider, tc.processor)
			err := svc.CompleteUpload(ctx, tc.userID, tc.uploadID)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr.Error())
				}

				if errors.Is(tc.wantErr, ErrUploadNotFound) ||
					errors.Is(tc.wantErr, ErrUploadNotOwned) ||
					errors.Is(tc.wantErr, ErrUploadNotPending) ||
					errors.Is(tc.wantErr, ErrFileNotFound) {
					if !errors.Is(err, tc.wantErr) {
						t.Fatalf("expected error %v, got %v", tc.wantErr, err)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestService_CompleteUpload_QuarantineKeySpy(t *testing.T) {
	ownerID := uuid.New()
	uploadID := uuid.New()

	var capturedKey string
	repo := &stubRepository{
		getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
			return Upload{
				ID:     uploadID,
				UserID: ownerID,
				Status: UploadStatusPENDING,
			}, nil
		},
	}
	provider := &stubStorageProvider{
		objectExists: func(_ context.Context, key string) (bool, error) {
			capturedKey = key
			return true, nil
		},
	}

	svc := newTestService(repo, provider)
	err := svc.CompleteUpload(context.Background(), ownerID, uploadID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedKey := bucketPrefixQuarantine + uploadID.String()
	if capturedKey != expectedKey {
		t.Fatalf("expected quarantine key %q, got %q", expectedKey, capturedKey)
	}
}

func TestService_CompleteUpload_EnqueueSpy(t *testing.T) {
	ownerID := uuid.New()
	uploadID := uuid.New()

	var enqueuedID uuid.UUID
	repo := &stubRepository{
		getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
			return Upload{
				ID:     uploadID,
				UserID: ownerID,
				Status: UploadStatusPENDING,
			}, nil
		},
	}
	provider := &stubStorageProvider{
		objectExists: func(_ context.Context, _ string) (bool, error) {
			return true, nil
		},
	}
	processor := &stubUploadProcessor{
		enqueue: func(_ context.Context, id uuid.UUID) error {
			enqueuedID = id
			return nil
		},
	}

	svc := newTestServiceWithProcessor(repo, provider, processor)
	err := svc.CompleteUpload(context.Background(), ownerID, uploadID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if enqueuedID != uploadID {
		t.Fatalf("expected enqueued upload ID %s, got %s", uploadID, enqueuedID)
	}
}

func TestService_GetUploadStatus(t *testing.T) {
	ownerID := uuid.New()
	uploadID := uuid.New()
	ctx := context.Background()

	mockUpload := Upload{
		ID:        uploadID,
		UserID:    ownerID,
		Status:    UploadStatusCOMPLETED,
		ObjectKey: "final/some-key.webp",
	}

	tests := []struct {
		name     string
		userID   uuid.UUID
		uploadID uuid.UUID
		repo     *stubRepository
		wantRes  Upload
		wantErr  error
	}{
		{
			name:     "success",
			userID:   ownerID,
			uploadID: uploadID,
			repo: &stubRepository{
				getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return mockUpload, nil
				},
			},
			wantRes: mockUpload,
		},
		{
			name:     "upload not found",
			userID:   ownerID,
			uploadID: uploadID,
			repo: &stubRepository{
				getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return Upload{}, ErrUploadNotFound
				},
			},
			wantErr: ErrUploadNotFound,
		},
		{
			name:     "not owned",
			userID:   uuid.New(),
			uploadID: uploadID,
			repo: &stubRepository{
				getByID: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return mockUpload, nil
				},
			},
			wantErr: ErrUploadNotOwned,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestService(tc.repo, &stubStorageProvider{})
			res, err := svc.GetUploadStatus(ctx, tc.userID, tc.uploadID)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr.Error())
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if res.ID != tc.wantRes.ID || res.Status != tc.wantRes.Status || res.UserID != tc.wantRes.UserID {
				t.Fatalf("unexpected result: %+v", res)
			}
		})
	}
}

func TestService_Bind(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	otherUserID := uuid.New()
	uploadID := uuid.New()

	tests := []struct {
		name    string
		repo    Repository
		userID  uuid.UUID
		purpose UploadPurpose
		wantErr error
	}{
		{
			name: "success when completed and owned",
			repo: &stubRepository{
				findUploadByIDForUpdate: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return Upload{
						ID:      uploadID,
						UserID:  ownerID,
						Status:  UploadStatusCOMPLETED,
						Purpose: PurposeAVATAR,
					}, nil
				},
				updateUploadStatus: func(_ context.Context, _ uuid.UUID, status UploadStatus) error {
					if status != UploadStatusBOUND {
						t.Fatalf("expected BOUND status, got %v", status)
					}
					return nil
				},
			},
			userID:  ownerID,
			purpose: PurposeAVATAR,
			wantErr: nil,
		},
		{
			name: "fails when not owned by user",
			repo: &stubRepository{
				findUploadByIDForUpdate: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return Upload{
						ID:      uploadID,
						UserID:  ownerID,
						Status:  UploadStatusCOMPLETED,
						Purpose: PurposeAVATAR,
					}, nil
				},
			},
			userID:  otherUserID,
			purpose: PurposeAVATAR,
			wantErr: ErrUploadNotFound,
		},
		{
			name: "fails when not completed",
			repo: &stubRepository{
				findUploadByIDForUpdate: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return Upload{
						ID:      uploadID,
						UserID:  ownerID,
						Status:  UploadStatusPENDING,
						Purpose: PurposeAVATAR,
					}, nil
				},
			},
			userID:  ownerID,
			purpose: PurposeAVATAR,
			wantErr: ErrUploadNotCompleted,
		},
		{
			name: "fails when purpose does not match",
			repo: &stubRepository{
				findUploadByIDForUpdate: func(_ context.Context, _ uuid.UUID) (Upload, error) {
					return Upload{
						ID:      uploadID,
						UserID:  ownerID,
						Status:  UploadStatusCOMPLETED,
						Purpose: PurposeBANNER,
					}, nil
				},
			},
			userID:  ownerID,
			purpose: PurposeAVATAR,
			wantErr: ErrUploadInvalidPurpose,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestService(tc.repo, &stubStorageProvider{})
			err := svc.Bind(ctx, uploadID, tc.userID, tc.purpose)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestService_Supersede(t *testing.T) {
	ctx := context.Background()
	uploadID := uuid.New()
	var updatedStatus UploadStatus

	repo := &stubRepository{
		updateUploadStatus: func(_ context.Context, id uuid.UUID, status UploadStatus) error {
			if id != uploadID {
				t.Fatalf("expected uploadID %v, got %v", uploadID, id)
			}
			updatedStatus = status
			return nil
		},
	}

	svc := newTestService(repo, &stubStorageProvider{})
	if err := svc.Supersede(ctx, uploadID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedStatus != UploadStatusSUPERSEDED {
		t.Fatalf("expected SUPERSEDED, got %v", updatedStatus)
	}
}
