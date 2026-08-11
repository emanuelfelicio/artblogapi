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
	create func(ctx context.Context, u Upload) (uuid.UUID, error)
}

func (s *stubRepository) Create(ctx context.Context, u Upload) (uuid.UUID, error) {
	if s.create != nil {
		return s.create(ctx, u)
	}
	return u.ID, nil
}

type stubStorageProvider struct {
	generateUploadURL func(ctx context.Context, key string, expires time.Duration) (string, error)
}

func (s *stubStorageProvider) GenerateUploadURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	if s.generateUploadURL != nil {
		return s.generateUploadURL(ctx, key, expires)
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

func (s *stubStorageProvider) ObjectExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type stubUploadProcessor struct{}

func (s *stubUploadProcessor) Enqueue(_ context.Context, _ uuid.UUID) error { return nil }

// --- HELPERS ---

func newTestService(repo Repository, provider StorageProvider) *Service {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	worker := &stubUploadProcessor{}
	return NewService(repo, provider, worker, 15*time.Minute, logger)
}

// --- TESTS ---

func TestService_InitUpload(t *testing.T) {
	userID := uuid.New()
	ctx := context.Background()

	tests := []struct {
		name        string
		purpose     string
		fileSize    int
		contentType string
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
				generateUploadURL: func(_ context.Context, _ string, _ time.Duration) (string, error) {
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

	if len(capturedUpload.ObjectKey) <= len(BucketPrefixQuarantine) {
		t.Fatal("object key should contain the quarantine prefix + upload ID")
	}

	prefix := capturedUpload.ObjectKey[:len(BucketPrefixQuarantine)]
	if prefix != BucketPrefixQuarantine {
		t.Fatalf("expected object key prefix %q, got %q", BucketPrefixQuarantine, prefix)
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
		generateUploadURL: func(_ context.Context, _ string, expires time.Duration) (string, error) {
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
