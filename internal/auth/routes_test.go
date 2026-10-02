package auth_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/gin-gonic/gin"
)

func TestRoutes_AppliesRateLimitMiddlewareWhenProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	v1 := r.Group("/api/v1")

	rateLimitedPaths := make(map[string]bool)
	rateLimiter := func(c *gin.Context) {
		rateLimitedPaths[c.FullPath()] = true
		c.AbortWithStatus(http.StatusTooManyRequests)
	}

	authMiddlewareCalled := false
	authMiddleware := func(c *gin.Context) {
		authMiddlewareCalled = true
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	cookieConfig := auth.NewRefreshCookieConfig("localhost", false)
	h := auth.NewHandler(nil, slog.Default(), cookieConfig)

	auth.Routes(v1, h, authMiddleware, rateLimiter)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/v1/auth/register"},
		{method: http.MethodPost, path: "/api/v1/auth/login"},
		{method: http.MethodPost, path: "/api/v1/auth/refresh"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp := httptest.NewRecorder()
		r.ServeHTTP(resp, req)

		if !rateLimitedPaths[tc.path] {
			t.Errorf("expected rateLimitMiddleware to be called for %s", tc.path)
		}
		if resp.Code != http.StatusTooManyRequests {
			t.Errorf("expected status %d for %s, got %d", http.StatusTooManyRequests, tc.path, resp.Code)
		}
	}

	// /logout should call authMiddleware, not rateLimiter
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if !authMiddlewareCalled {
		t.Errorf("expected authMiddleware to be called for /logout")
	}
	if rateLimitedPaths["/api/v1/auth/logout"] {
		t.Errorf("rateLimitMiddleware should not be called for /logout")
	}
}

func TestRoutes_WithoutRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	v1 := r.Group("/api/v1")

	authMiddleware := func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	cookieConfig := auth.NewRefreshCookieConfig("localhost", false)
	h := auth.NewHandler(nil, slog.Default(), cookieConfig)

	// Should not panic or fail when rateLimitMiddleware is nil
	auth.Routes(v1, h, authMiddleware, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	// Without rate limiting, the request reaches the handler (which returns 400 for empty body)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for empty register body, got %d", http.StatusBadRequest, resp.Code)
	}
}
