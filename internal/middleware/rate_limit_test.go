package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emanuelfelicio/artblogapi/config/response"
	"github.com/emanuelfelicio/artblogapi/internal/middleware"
	"github.com/gin-gonic/gin"
)

func TestRateLimiterMiddleware(t *testing.T) {
	limiter, err := middleware.NewRateLimiter(middleware.RateLimitPolicy{
		Limit:  2,
		Window: time.Minute,
	})
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}

	r := gin.New()
	r.GET("/", limiter.Middleware(func(_ *http.Request) string { return "client" }), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	for index, wantStatus := range []int{http.StatusNoContent, http.StatusNoContent, http.StatusTooManyRequests} {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		r.ServeHTTP(resp, req)

		if resp.Code != wantStatus {
			t.Fatalf("request %d status = %d, want %d", index+1, resp.Code, wantStatus)
		}
	}

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(resp, req)

	if got := resp.Header().Get("Retry-After"); got != "30" {
		t.Fatalf("Retry-After = %q, want %q", got, "30")
	}

	var body response.ErrorResponse[any]
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body.Error == nil || body.Error.Code != response.RateLimitedCode {
		t.Fatalf("error code = %v, want %q", body.Error, response.RateLimitedCode)
	}
}

func TestRateLimiterRetryAfterDefault(t *testing.T) {
	limiter, err := middleware.NewRateLimiter(middleware.RateLimitPolicy{
		Limit:  1,
		Window: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}

	r := gin.New()
	r.GET("/", limiter.Middleware(func(_ *http.Request) string { return "client" }), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	// First request succeeds
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("first request status = %d, want %d", resp.Code, http.StatusNoContent)
	}

	// Immediate second request exceeds limit, gets 429 and minRetryAfterSeconds ("1")
	resp = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(resp, req)
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want %d", resp.Code, http.StatusTooManyRequests)
	}
	if got := resp.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want %q", got, "1")
	}
}

func TestClientIPUsesForwardedForFromTrustedProxy(t *testing.T) {
	clientIP, err := middleware.ClientIP([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("ClientIP() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "192.0.2.10, 10.0.0.2")
	if got := clientIP(request); got != "192.0.2.10" {
		t.Fatalf("trusted client IP = %q, want %q", got, "192.0.2.10")
	}
}

func TestClientIPIgnoresForwardedForFromUntrustedSource(t *testing.T) {
	clientIP, err := middleware.ClientIP([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("ClientIP() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.20:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.10")
	if got := clientIP(request); got != "192.0.2.20" {
		t.Fatalf("untrusted client IP = %q, want %q", got, "192.0.2.20")
	}
}

func TestClientIPSupportsTrustedIPv6Proxy(t *testing.T) {
	clientIP, err := middleware.ClientIP([]string{"2001:db8::/32"})
	if err != nil {
		t.Fatalf("ClientIP() IPv6 error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "[2001:db8::1]:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.195")
	if got := clientIP(request); got != "203.0.113.195" {
		t.Fatalf("trusted IPv6 proxy client IP = %q, want %q", got, "203.0.113.195")
	}
}

func TestClientIPPrefersForwardedForOverRealIP(t *testing.T) {
	clientIP, err := middleware.ClientIP([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("ClientIP() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "192.0.2.30, 10.0.0.2")
	request.Header.Set("X-Real-IP", "198.51.100.30")
	if got := clientIP(request); got != "192.0.2.30" {
		t.Fatalf("X-Forwarded-For client IP = %q, want %q", got, "192.0.2.30")
	}
}

func TestClientIPInvalidCIDR(t *testing.T) {
	_, err := middleware.ClientIP([]string{"invalid-cidr"})
	if err == nil {
		t.Fatal("expected error for invalid CIDR, got nil")
	}
}

func TestNewRateLimiterRejectsInvalidPolicy(t *testing.T) {
	for _, test := range []struct {
		limit  int
		window time.Duration
	}{
		{limit: 0, window: time.Minute},
		{limit: -5, window: time.Minute},
		{limit: 1, window: 0},
		{limit: 1, window: -time.Minute},
	} {
		policy := middleware.RateLimitPolicy{Limit: test.limit, Window: test.window}
		if _, err := middleware.NewRateLimiter(policy); err == nil {
			t.Fatalf("NewRateLimiter(%+v) returned nil error", policy)
		}
	}
}

func TestRateLimiterConcurrency(t *testing.T) {
	limiter, err := middleware.NewRateLimiter(middleware.RateLimitPolicy{
		Limit:  100,
		Window: time.Hour,
	})
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}

	r := gin.New()
	r.GET("/", limiter.Middleware(func(_ *http.Request) string { return "concurrent-client" }), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	var wg sync.WaitGroup
	var allowed, rejected atomic.Int32
	workers := 20
	requestsPerWorker := 20

	for range workers {
		wg.Go(func() {
			for range requestsPerWorker {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				resp := httptest.NewRecorder()
				r.ServeHTTP(resp, req)
				switch resp.Code {
				case http.StatusOK:
					allowed.Add(1)
				case http.StatusTooManyRequests:
					rejected.Add(1)
				default:
					t.Errorf("unexpected status = %d", resp.Code)
				}
			}
		})
	}

	wg.Wait()
	if got := allowed.Load(); got != 100 {
		t.Fatalf("allowed requests = %d, want %d", got, 100)
	}
	if got := rejected.Load(); got != 300 {
		t.Fatalf("rejected requests = %d, want %d", got, 300)
	}
}
