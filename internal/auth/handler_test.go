package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// --- STUBS ---

type stubAuthService struct {
	register func(ctx context.Context, username, email, rawPassword, userAgent, ip, deviceID string) (Auth, error)
	login    func(ctx context.Context, credential, password, userAgent, ip, deviceID string) (Auth, error)
	refresh  func(ctx context.Context, refreshToken, userAgent, ip, deviceID string) (Auth, error)
	logout   func(ctx context.Context, refreshToken string, currentUserID uuid.UUID) error
}

func (s *stubAuthService) Register(ctx context.Context, username, email, rawPassword, userAgent, ip, deviceID string) (Auth, error) {
	if s.register != nil {
		return s.register(ctx, username, email, rawPassword, userAgent, ip, deviceID)
	}
	return Auth{}, nil
}

func (s *stubAuthService) Login(ctx context.Context, credential, password, userAgent, ip, deviceID string) (Auth, error) {
	if s.login != nil {
		return s.login(ctx, credential, password, userAgent, ip, deviceID)
	}
	return Auth{}, nil
}

func (s *stubAuthService) Refresh(ctx context.Context, refreshToken, userAgent, ip, deviceID string) (Auth, error) {
	if s.refresh != nil {
		return s.refresh(ctx, refreshToken, userAgent, ip, deviceID)
	}
	return Auth{}, nil
}

func (s *stubAuthService) Logout(ctx context.Context, refreshToken string, currentUserID uuid.UUID) error {
	if s.logout != nil {
		return s.logout(ctx, refreshToken, currentUserID)
	}
	return nil
}

// --- HELPERS ---

func setupTestHandler(t *testing.T, s AuthService) (*handler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cookieConfig := NewRefreshCookieConfig("localhost", false)
	h := NewHandler(s, logger, cookieConfig)

	r := gin.New()
	// Minimal routes for testing
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)
	r.POST("/refresh", h.Refresh)
	r.POST("/logout", func(c *gin.Context) {
		// Mock authentication middleware behavior
		c.Set("auth.principal", AuthPrincipal{UserID: uuid.NewString()})
		h.Logout(c)
	})

	return h, r
}

func performRequest(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
		}
		bodyReader = strings.NewReader(string(b))
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- TESTS ---

func TestHandler_Register(t *testing.T) {
	t.Parallel()

	const (
		username = "john.doe"
		email    = "john@example.com"
		password = "StrongPassword!123"
	)

	tests := []struct {
		name           string
		payload        any
		setupMock      func(s *stubAuthService)
		expectedStatus int
		verifyResponse func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name: "success",
			payload: RegisterRequest{
				Username: username,
				Email:    email,
				Password: password,
			},
			setupMock: func(s *stubAuthService) {
				s.register = func(ctx context.Context, u, e, p, userAgent, ip, deviceID string) (Auth, error) {
					if u != username || e != email || p != password {
						return Auth{}, errors.New("unexpected service arguments")
					}
					return Auth{AccessToken: "access", RefreshToken: "refresh", RefreshTTL: 3600}, nil
				}
			},
			expectedStatus: http.StatusCreated,
			verifyResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				if !strings.Contains(w.Body.String(), `"access_token":"access"`) {
					t.Errorf("expected access_token in body, got %s", w.Body.String())
				}
				cookie := w.Header().Get("Set-Cookie")
				if !strings.Contains(cookie, "refresh_token=refresh") {
					t.Errorf("expected refresh_token cookie, got %s", cookie)
				}
			},
		},
		{
			name: "bad request - invalid email",
			payload: RegisterRequest{
				Username: username,
				Email:    "invalid-email",
				Password: password,
			},
			expectedStatus: http.StatusBadRequest,
			verifyResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				if !strings.Contains(w.Body.String(), `"code":"VALIDATION_ERROR"`) {
					t.Errorf("expected VALIDATION_ERROR, got %s", w.Body.String())
				}
			},
		},
		{
			name: "conflict - email exists",
			payload: RegisterRequest{
				Username: username,
				Email:    email,
				Password: password,
			},
			setupMock: func(s *stubAuthService) {
				s.register = func(ctx context.Context, u, e, p, userAgent, ip, deviceID string) (Auth, error) {
					return Auth{}, ErrEmailAlreadyExists
				}
			},
			expectedStatus: http.StatusConflict,
			verifyResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				if !strings.Contains(w.Body.String(), `"email_exists":true`) {
					t.Errorf("expected email_exists:true, got %s", w.Body.String())
				}
			},
		},
		{
			name: "internal error",
			payload: RegisterRequest{
				Username: username,
				Email:    email,
				Password: password,
			},
			setupMock: func(s *stubAuthService) {
				s.register = func(ctx context.Context, u, e, p, userAgent, ip, deviceID string) (Auth, error) {
					return Auth{}, errors.New("internal error")
				}
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubAuthService{}
			if tt.setupMock != nil {
				tt.setupMock(stub)
			}
			_, r := setupTestHandler(t, stub)
			w := performRequest(t, r, http.MethodPost, "/register", tt.payload)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if tt.verifyResponse != nil {
				tt.verifyResponse(t, w)
			}
		})
	}
}

func TestHandler_Login(t *testing.T) {
	t.Parallel()

	const (
		credential = "john.doe"
		password   = "CorrectPass!123"
	)

	tests := []struct {
		name           string
		payload        interface{}
		setupMock      func(s *stubAuthService)
		expectedStatus int
		verifyResponse func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name: "success",
			payload: LoginRequest{
				Credential: credential,
				Password:   password,
			},
			setupMock: func(s *stubAuthService) {
				s.login = func(ctx context.Context, cred, pass, userAgent, ip, deviceID string) (Auth, error) {
					if cred != credential || pass != password {
						return Auth{}, errors.New("unexpected service arguments")
					}
					return Auth{AccessToken: "access", RefreshToken: "refresh", RefreshTTL: 3600}, nil
				}
			},
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				if !strings.Contains(w.Body.String(), `"access_token":"access"`) {
					t.Errorf("expected access_token in body, got %s", w.Body.String())
				}
			},
		},
		{
			name: "invalid credentials",
			payload: LoginRequest{
				Credential: credential,
				Password:   "WrongPass",
			},
			setupMock: func(s *stubAuthService) {
				s.login = func(ctx context.Context, cred, pass, userAgent, ip, deviceID string) (Auth, error) {
					return Auth{}, ErrInvalidCredentials
				}
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "user inactive",
			payload: LoginRequest{
				Credential: credential,
				Password:   password,
			},
			setupMock: func(s *stubAuthService) {
				s.login = func(ctx context.Context, cred, pass, userAgent, ip, deviceID string) (Auth, error) {
					return Auth{}, ErrUserInactive
				}
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "internal error",
			payload: LoginRequest{
				Credential: credential,
				Password:   password,
			},
			setupMock: func(s *stubAuthService) {
				s.login = func(ctx context.Context, cred, pass, userAgent, ip, deviceID string) (Auth, error) {
					return Auth{}, errors.New("internal error")
				}
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubAuthService{}
			if tt.setupMock != nil {
				tt.setupMock(stub)
			}
			_, r := setupTestHandler(t, stub)
			w := performRequest(t, r, http.MethodPost, "/login", tt.payload)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if tt.verifyResponse != nil {
				tt.verifyResponse(t, w)
			}
		})
	}
}

func TestHandler_Refresh(t *testing.T) {
	t.Parallel()

	const validToken = "valid-token"

	tests := []struct {
		name           string
		setupMock      func(s *stubAuthService)
		setupCookie    func(req *http.Request)
		expectedStatus int
	}{
		{
			name: "success",
			setupMock: func(s *stubAuthService) {
				s.refresh = func(ctx context.Context, refreshToken, userAgent, ip, deviceID string) (Auth, error) {
					if refreshToken != validToken {
						return Auth{}, errors.New("unexpected service arguments")
					}
					return Auth{AccessToken: "new-access", RefreshToken: "new-refresh", RefreshTTL: 3600}, nil
				}
			},
			setupCookie: func(req *http.Request) {
				req.AddCookie(&http.Cookie{Name: cookieName, Value: validToken})
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing cookie",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "token expired",
			setupMock: func(s *stubAuthService) {
				s.refresh = func(ctx context.Context, refreshToken, userAgent, ip, deviceID string) (Auth, error) {
					return Auth{}, WrapDomainErr(ErrTokenExpired)
				}
			},
			setupCookie: func(req *http.Request) {
				req.AddCookie(&http.Cookie{Name: cookieName, Value: "expired-token"})
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "internal error",
			setupMock: func(s *stubAuthService) {
				s.refresh = func(ctx context.Context, refreshToken, userAgent, ip, deviceID string) (Auth, error) {
					return Auth{}, errors.New("internal error")
				}
			},
			setupCookie: func(req *http.Request) {
				req.AddCookie(&http.Cookie{Name: cookieName, Value: validToken})
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubAuthService{}
			if tt.setupMock != nil {
				tt.setupMock(stub)
			}
			_, r := setupTestHandler(t, stub)

			req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
			if tt.setupCookie != nil {
				tt.setupCookie(req)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHandler_Logout(t *testing.T) {
	t.Parallel()

	var capturedToken string
	var capturedUserID uuid.UUID
	stub := &stubAuthService{}
	stub.logout = func(ctx context.Context, refreshToken string, currentUserID uuid.UUID) error {
		capturedToken = refreshToken
		capturedUserID = currentUserID
		return nil
	}
	_, r := setupTestHandler(t, stub)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "some-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	if capturedToken != "some-token" {
		t.Errorf("expected refresh_token to be some-token, got %s", capturedToken)
	}
	if capturedUserID == uuid.Nil {
		t.Errorf("expected user_id to be populated")
	}

	var cookieFound bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" {
			cookieFound = true
			if c.Value != "" || c.MaxAge > 0 {
				t.Errorf("expected refresh_token cookie to be cleared, got Value=%q, MaxAge=%d", c.Value, c.MaxAge)
			}
		}
	}
	if !cookieFound {
		t.Errorf("expected refresh_token cookie to be present in Set-Cookie header")
	}
}

func TestHandler_Logout_Failure(t *testing.T) {
	t.Parallel()

	stub := &stubAuthService{}
	stub.logout = func(ctx context.Context, refreshToken string, currentUserID uuid.UUID) error {
		return errors.New("db error")
	}
	_, r := setupTestHandler(t, stub)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "some-token"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d even when logout service fails", http.StatusNoContent, w.Code)
	}
}
