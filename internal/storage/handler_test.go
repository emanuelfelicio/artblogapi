package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"

	"github.com/emanuelfelicio/artblogapi/internal/testutil/testauth"
	"github.com/emanuelfelicio/artblogapi/internal/testutil/testhttp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// --- STUBS ---

type stubStorageService struct {
	initUpload      func(ctx context.Context, userID uuid.UUID, purpose string, fileSize int, contentType string) (uuid.UUID, string, error)
	completeUpload  func(ctx context.Context, userID uuid.UUID, uploadID uuid.UUID) error
	getUploadStatus func(ctx context.Context, userID uuid.UUID, uploadID uuid.UUID) (Upload, error)
}

func (s *stubStorageService) InitUpload(ctx context.Context, userID uuid.UUID, purpose string, fileSize int, contentType string) (uuid.UUID, string, error) {
	if s.initUpload != nil {
		return s.initUpload(ctx, userID, purpose, fileSize, contentType)
	}
	return uuid.New(), "https://s3.example.com/presigned", nil
}

func (s *stubStorageService) CompleteUpload(ctx context.Context, userID uuid.UUID, uploadID uuid.UUID) error {
	if s.completeUpload != nil {
		return s.completeUpload(ctx, userID, uploadID)
	}
	return nil
}

func (s *stubStorageService) GetUploadStatus(ctx context.Context, userID uuid.UUID, uploadID uuid.UUID) (Upload, error) {
	if s.getUploadStatus != nil {
		return s.getUploadStatus(ctx, userID, uploadID)
	}
	return Upload{}, nil
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// --- HELPERS ---

func setupTestRouter(svc StorageService, authMiddleware gin.HandlerFunc) *gin.Engine {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(svc, logger)

	r := gin.New()
	v1 := r.Group("/api/v1")
	Routes(v1, h, authMiddleware)
	return r
}

// --- POST /uploads/init ---

func TestHandler_InitUpload(t *testing.T) {
	validBody := map[string]any{
		"purpose":      "AVATAR",
		"file_size":    1024,
		"content_type": "image/png",
	}

	tests := []struct {
		name       string
		body       any
		svc        *stubStorageService
		wantStatus int
	}{
		{
			name:       "201 created",
			body:       validBody,
			svc:        &stubStorageService{},
			wantStatus: http.StatusCreated,
		},
		{
			name: "400 missing purpose",
			body: map[string]any{
				"file_size":    1024,
				"content_type": "image/png",
			},
			svc:        &stubStorageService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "400 file_size zero",
			body: map[string]any{
				"purpose":      "AVATAR",
				"file_size":    0,
				"content_type": "image/png",
			},
			svc:        &stubStorageService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "400 service returns ErrInvalidPurpose",
			body: validBody,
			svc: &stubStorageService{
				initUpload: func(_ context.Context, _ uuid.UUID, _ string, _ int, _ string) (uuid.UUID, string, error) {
					return uuid.Nil, "", ErrInvalidPurpose
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "400 service returns ErrFileSizeExceeded",
			body: validBody,
			svc: &stubStorageService{
				initUpload: func(_ context.Context, _ uuid.UUID, _ string, _ int, _ string) (uuid.UUID, string, error) {
					return uuid.Nil, "", ErrFileSizeExceeded
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "400 service returns ErrInvalidFileSize",
			body: validBody,
			svc: &stubStorageService{
				initUpload: func(_ context.Context, _ uuid.UUID, _ string, _ int, _ string) (uuid.UUID, string, error) {
					return uuid.Nil, "", ErrInvalidFileSize
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "400 service returns ErrInvaliImageContentType",
			body: validBody,
			svc: &stubStorageService{
				initUpload: func(_ context.Context, _ uuid.UUID, _ string, _ int, _ string) (uuid.UUID, string, error) {
					return uuid.Nil, "", ErrInvaliImageContentType
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "500 unexpected service error",
			body: validBody,
			svc: &stubStorageService{
				initUpload: func(_ context.Context, _ uuid.UUID, _ string, _ int, _ string) (uuid.UUID, string, error) {
					return uuid.Nil, "", fmt.Errorf("db timeout")
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := setupTestRouter(tc.svc, testauth.WithPrincipal(uuid.NewString()))
			w := testhttp.DoRequest(t, r, http.MethodPost, "/api/v1/uploads/init", tc.body)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

// --- POST /uploads/complete ---

func TestHandler_CompleteUpload(t *testing.T) {
	validBody := map[string]any{
		"upload_id": uuid.NewString(),
	}

	tests := []struct {
		name       string
		body       any
		svc        *stubStorageService
		wantStatus int
	}{
		{
			name:       "202 accepted",
			body:       validBody,
			svc:        &stubStorageService{},
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "400 missing upload_id",
			body:       map[string]any{},
			svc:        &stubStorageService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "400 invalid uuid format",
			body:       map[string]any{"upload_id": "not-a-uuid"},
			svc:        &stubStorageService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "404 upload not found",
			body: validBody,
			svc: &stubStorageService{
				completeUpload: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
					return ErrUploadNotFound
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "403 upload not owned",
			body: validBody,
			svc: &stubStorageService{
				completeUpload: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
					return ErrUploadNotOwned
				},
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "409 upload not pending",
			body: validBody,
			svc: &stubStorageService{
				completeUpload: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
					return ErrUploadNotPending
				},
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "422 file not in quarantine",
			body: validBody,
			svc: &stubStorageService{
				completeUpload: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
					return ErrFileNotInQuarantine
				},
			},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "500 unexpected service error",
			body: validBody,
			svc: &stubStorageService{
				completeUpload: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error {
					return fmt.Errorf("db timeout")
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := setupTestRouter(tc.svc, testauth.WithPrincipal(uuid.NewString()))
			w := testhttp.DoRequest(t, r, http.MethodPost, "/api/v1/uploads/complete", tc.body)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, w.Code, w.Body.String())
			}

			if w.Code == http.StatusAccepted && w.Body.Len() != 0 {
				t.Fatalf("expected empty body for 202 Accepted, got %s", w.Body.String())
			}
		})
	}
}

// --- GET /uploads/:id ---

func TestHandler_GetUploadStatus(t *testing.T) {
	uploadID := uuid.New()
	userID := uuid.New()

	tests := []struct {
		name       string
		idParam    string
		svc        *stubStorageService
		wantStatus int
	}{
		{
			name:    "200 ok",
			idParam: uploadID.String(),
			svc: &stubStorageService{
				getUploadStatus: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (Upload, error) {
					return Upload{
						ID:     uploadID,
						UserID: userID,
						Status: UploadStatusCOMPLETED,
					}, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "400 invalid uuid format",
			idParam:    "not-a-uuid",
			svc:        &stubStorageService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:    "404 not found",
			idParam: uploadID.String(),
			svc: &stubStorageService{
				getUploadStatus: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (Upload, error) {
					return Upload{}, ErrUploadNotFound
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:    "403 forbidden",
			idParam: uploadID.String(),
			svc: &stubStorageService{
				getUploadStatus: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (Upload, error) {
					return Upload{}, ErrUploadNotOwned
				},
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:    "500 unexpected service error",
			idParam: uploadID.String(),
			svc: &stubStorageService{
				getUploadStatus: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (Upload, error) {
					return Upload{}, fmt.Errorf("db timeout")
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := setupTestRouter(tc.svc, testauth.WithPrincipal(userID.String()))
			w := testhttp.DoRequest(t, r, http.MethodGet, "/api/v1/uploads/"+tc.idParam, nil)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}
