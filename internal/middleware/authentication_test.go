package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/emanuelfelicio/artblogapi/internal/auth/token"
	"github.com/emanuelfelicio/artblogapi/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	testJWTSecretString = "test-secret-32-bytes"
	testJWTIssuer       = "artblog"
	testJWTTTL          = time.Hour
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}
func TestAuthentication_MissingHeader(t *testing.T) {
	t.Parallel()

	svc, err := token.NewJWT([]byte(testJWTSecretString), testJWTIssuer, testJWTTTL)
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}

	r := gin.New()
	r.GET("/protected", middleware.Authentication(*svc), func(c *gin.Context) {
		c.Header("X-Ran", "1")
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if v := w.Header().Get("X-Ran"); v != "" {
		t.Fatalf("handler executed unexpectedly (X-Ran=%q)", v)
	}
}

func TestAuthentication_InvalidToken(t *testing.T) {
	t.Parallel()

	svc, err := token.NewJWT([]byte(testJWTSecretString), testJWTIssuer, testJWTTTL)
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}

	r := gin.New()
	r.GET("/protected", middleware.Authentication(*svc), func(c *gin.Context) {
		c.Header("X-Ran", "1")
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer this.is.an.invalid.token")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if v := w.Header().Get("X-Ran"); v != "" {
		t.Fatalf("handler executed unexpectedly (X-Ran=%q)", v)
	}
}

func TestAuthentication_Success(t *testing.T) {
	t.Parallel()

	svc, err := token.NewJWT([]byte(testJWTSecretString), testJWTIssuer, testJWTTTL)
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}

	userID := uuid.New()
	user := auth.User{ID: userID}

	accessToken, err := svc.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	r := gin.New()
	r.GET("/protected", middleware.Authentication(*svc), func(c *gin.Context) {
		v, ok := c.Get(middleware.ContextAuthPrincipalKey)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		ap, ok := v.(token.AuthPrincipal)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Header("X-UserID", ap.UserID)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("X-UserID"); got != userID.String() {
		t.Fatalf("user id = %q, want %q", got, userID.String())
	}
}
