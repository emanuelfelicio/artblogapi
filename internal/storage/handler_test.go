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
	initUpload func(ctx context.Context, userID uuid.UUID, purpose string, fileSize int, contentType string) (uuid.UUID, string, error)
}

func (s *stubStorageService) InitUpload(ctx context.Context, userID uuid.UUID, purpose string, fileSize int, contentType string) (uuid.UUID, string, error) {
	if s.initUpload != nil {
		return s.initUpload(ctx, userID, purpose, fileSize, contentType)
	}
	return uuid.New(), "https://s3.example.com/presigned", nil
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
