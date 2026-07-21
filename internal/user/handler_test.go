package user

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emanuelfelicio/artblogapi/config/response"
	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// --- STUBS ---

type stubUserService struct {
	getPublicProfile func(ctx context.Context, username string) (User, error)
	getMyProfile     func(ctx context.Context, userID uuid.UUID) (User, error)
	updateProfile    func(ctx context.Context, userID uuid.UUID, displayName, bio *string) (User, error)
	updateAvatar     func(ctx context.Context, userID uuid.UUID, uploadIDStr string) error
	updateBanner     func(ctx context.Context, userID uuid.UUID, uploadIDStr string) error
}

func (s *stubUserService) GetPublicProfile(ctx context.Context, username string) (User, error) {
	if s.getPublicProfile != nil {
		return s.getPublicProfile(ctx, username)
	}
	return User{}, nil
}

func (s *stubUserService) GetMyProfile(ctx context.Context, userID uuid.UUID) (User, error) {
	if s.getMyProfile != nil {
		return s.getMyProfile(ctx, userID)
	}
	return User{}, nil
}

func (s *stubUserService) UpdateProfile(ctx context.Context, userID uuid.UUID, displayName, bio *string) (User, error) {
	if s.updateProfile != nil {
		return s.updateProfile(ctx, userID, displayName, bio)
	}
	return User{}, nil
}

func (s *stubUserService) UpdateAvatar(ctx context.Context, userID uuid.UUID, uploadIDStr string) error {
	if s.updateAvatar != nil {
		return s.updateAvatar(ctx, userID, uploadIDStr)
	}
	return nil
}

func (s *stubUserService) UpdateBanner(ctx context.Context, userID uuid.UUID, uploadIDStr string) error {
	if s.updateBanner != nil {
		return s.updateBanner(ctx, userID, uploadIDStr)
	}
	return nil
}

// --- HELPERS ---

func setupRouter(svc UserService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(svc, logger, "http://cdn.example.com", "http://cdn.example.com/default/avatar.png", "http://cdn.example.com/default/banner.png")

	r := gin.New()
	v1 := r.Group("/api/v1")
	userID := uuid.New()
	authMiddleware := func(c *gin.Context) {
		c.Set("auth.principal", auth.AuthPrincipal{UserID: userID.String()})
		c.Next()
	}
	Routes(v1, h, authMiddleware)
	return r
}

func setupRouterWithPrincipal(svc UserService, principalID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(svc, logger, "http://cdn.example.com", "http://cdn.example.com/default/avatar.png", "http://cdn.example.com/default/banner.png")

	r := gin.New()
	v1 := r.Group("/api/v1")
	authMiddleware := func(c *gin.Context) {
		c.Set("auth.principal", auth.AuthPrincipal{UserID: principalID})
		c.Next()
	}
	Routes(v1, h, authMiddleware)
	return r
}

func setupRouterNoAuth(svc UserService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(svc, logger, "http://cdn.example.com", "http://cdn.example.com/default/avatar.png", "http://cdn.example.com/default/banner.png")

	r := gin.New()
	v1 := r.Group("/api/v1")
	noAuth := func(c *gin.Context) { c.Next() }
	Routes(v1, h, noAuth)
	return r
}

func doRequest(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(b)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeResponse[T any](t *testing.T, w *httptest.ResponseRecorder) response.Response[T] {
	t.Helper()
	var resp response.Response[T]
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

func sampleUser() User {
	key := "avatars/file.png"
	return User{
		ID:          uuid.New(),
		Username:    "joao",
		Email:       "joao@example.com",
		DisplayName: "João",
		Bio:         "artista",
		AvatarKey:   &key,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// --- GET /:username ---

func TestHandler_GetPublicProfile_200(t *testing.T) {
	u := sampleUser()
	svc := &stubUserService{
		getPublicProfile: func(_ context.Context, username string) (User, error) {
			return u, nil
		},
	}
	r := setupRouter(svc)
	w := doRequest(t, r, http.MethodGet, "/api/v1/users/joao", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := decodeResponse[PublicProfileResponse](t, w)
	if resp.Data.Username != u.Username {
		t.Errorf("expected username %q, got %q", u.Username, resp.Data.Username)
	}
	if !strings.HasPrefix(resp.Data.AvatarURL, "http://cdn.example.com/") {
		t.Errorf("expected cdn avatar URL, got %q", resp.Data.AvatarURL)
	}
}

func TestHandler_GetPublicProfile_404(t *testing.T) {
	svc := &stubUserService{
		getPublicProfile: func(_ context.Context, _ string) (User, error) {
			return User{}, ErrUserNotFound
		},
	}
	r := setupRouter(svc)
	w := doRequest(t, r, http.MethodGet, "/api/v1/users/ghost", nil)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- GET /me ---

func TestHandler_GetMyProfile_200(t *testing.T) {
	u := sampleUser()
	svc := &stubUserService{
		getMyProfile: func(_ context.Context, _ uuid.UUID) (User, error) {
			return u, nil
		},
	}
	r := setupRouterWithPrincipal(svc, u.ID.String())
	w := doRequest(t, r, http.MethodGet, "/api/v1/users/me", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := decodeResponse[MyProfileResponse](t, w)
	if resp.Data.Email != u.Email {
		t.Errorf("expected email %q, got %q", u.Email, resp.Data.Email)
	}
}

func TestHandler_GetMyProfile_500_NoPrincipal(t *testing.T) {
	svc := &stubUserService{}
	r := setupRouterNoAuth(svc)
	w := doRequest(t, r, http.MethodGet, "/api/v1/users/me", nil)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- PUT /me ---

func TestHandler_UpdateProfile_200(t *testing.T) {
	u := sampleUser()
	svc := &stubUserService{
		updateProfile: func(_ context.Context, _ uuid.UUID, dn, b *string) (User, error) {
			if dn != nil {
				u.DisplayName = *dn
			}
			return u, nil
		},
	}
	r := setupRouterWithPrincipal(svc, u.ID.String())
	body := map[string]string{"display_name": "Novo Nome"}
	w := doRequest(t, r, http.MethodPut, "/api/v1/users/me", body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdateProfile_400_BioTooLong(t *testing.T) {
	svc := &stubUserService{}
	r := setupRouterWithPrincipal(svc, uuid.NewString())
	body := map[string]string{"bio": strings.Repeat("a", 501)}
	w := doRequest(t, r, http.MethodPut, "/api/v1/users/me", body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdateProfile_400_DisplayNameTooLong(t *testing.T) {
	svc := &stubUserService{}
	r := setupRouterWithPrincipal(svc, uuid.NewString())
	body := map[string]string{"display_name": strings.Repeat("x", 61)}
	w := doRequest(t, r, http.MethodPut, "/api/v1/users/me", body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdateProfile_500_NoPrincipal(t *testing.T) {
	svc := &stubUserService{}
	r := setupRouterNoAuth(svc)
	body := map[string]string{"display_name": "qualquer"}
	w := doRequest(t, r, http.MethodPut, "/api/v1/users/me", body)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- PUT /me/avatar ---

func TestHandler_UpdateAvatar_204(t *testing.T) {
	svc := &stubUserService{
		updateAvatar: func(_ context.Context, _ uuid.UUID, _ string) error {
			return nil
		},
	}
	r := setupRouterWithPrincipal(svc, uuid.NewString())
	body := map[string]string{"upload_id": uuid.NewString()}
	w := doRequest(t, r, http.MethodPut, "/api/v1/users/me/avatar", body)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdateAvatar_422_UploadNotFound(t *testing.T) {
	svc := &stubUserService{
		updateAvatar: func(_ context.Context, _ uuid.UUID, _ string) error {
			return ErrUploadNotFound
		},
	}
	r := setupRouterWithPrincipal(svc, uuid.NewString())
	body := map[string]string{"upload_id": uuid.NewString()}
	w := doRequest(t, r, http.MethodPut, "/api/v1/users/me/avatar", body)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdateAvatar_400_MissingUploadID(t *testing.T) {
	svc := &stubUserService{}
	r := setupRouterWithPrincipal(svc, uuid.NewString())
	body := map[string]string{}
	w := doRequest(t, r, http.MethodPut, "/api/v1/users/me/avatar", body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdateAvatar_500_NoPrincipal(t *testing.T) {
	svc := &stubUserService{}
	r := setupRouterNoAuth(svc)
	body := map[string]string{"upload_id": uuid.NewString()}
	w := doRequest(t, r, http.MethodPut, "/api/v1/users/me/avatar", body)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- PUT /me/banner ---

func TestHandler_UpdateBanner_204(t *testing.T) {
	svc := &stubUserService{
		updateBanner: func(_ context.Context, _ uuid.UUID, _ string) error { return nil },
	}
	r := setupRouterWithPrincipal(svc, uuid.NewString())
	body := map[string]string{"upload_id": uuid.NewString()}
	w := doRequest(t, r, http.MethodPut, "/api/v1/users/me/banner", body)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdateBanner_422_UploadNotFound(t *testing.T) {
	svc := &stubUserService{
		updateBanner: func(_ context.Context, _ uuid.UUID, _ string) error {
			return ErrUploadNotFound
		},
	}
	r := setupRouterWithPrincipal(svc, uuid.NewString())
	body := map[string]string{"upload_id": uuid.NewString()}
	w := doRequest(t, r, http.MethodPut, "/api/v1/users/me/banner", body)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

// --- URL resolution ---

func TestHandler_AvatarURL_Fallback(t *testing.T) {
	svc := &stubUserService{
		getPublicProfile: func(_ context.Context, _ string) (User, error) {
			u := sampleUser()
			u.AvatarKey = nil
			return u, nil
		},
	}
	r := setupRouter(svc)
	w := doRequest(t, r, http.MethodGet, "/api/v1/users/joao", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := decodeResponse[PublicProfileResponse](t, w)
	if resp.Data.AvatarURL != "http://cdn.example.com/default/avatar.png" {
		t.Errorf("expected default avatar URL, got %q", resp.Data.AvatarURL)
	}
}

func TestHandler_BannerURL_Fallback(t *testing.T) {
	svc := &stubUserService{
		getPublicProfile: func(_ context.Context, _ string) (User, error) {
			u := sampleUser()
			u.BannerKey = nil
			return u, nil
		},
	}
	r := setupRouter(svc)
	w := doRequest(t, r, http.MethodGet, "/api/v1/users/joao", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := decodeResponse[PublicProfileResponse](t, w)
	if resp.Data.BannerURL != "http://cdn.example.com/default/banner.png" {
		t.Errorf("expected default banner URL, got %q", resp.Data.BannerURL)
	}
}
