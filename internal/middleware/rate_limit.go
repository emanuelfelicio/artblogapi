package middleware

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/emanuelfelicio/artblogapi/config/response"
	"github.com/emanuelfelicio/artblogapi/internal/middleware/requestcontext"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type RateLimitPolicy struct {
	Limit  int
	Window time.Duration
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu           sync.Mutex
	policy       RateLimitPolicy
	limit        rate.Limit
	burst        int
	retryAfter   int
	cleanupEvery time.Duration
	lastCleanup  time.Time
	limiters     map[string]*limiterEntry
}

const (
	// minRetryAfterSeconds defines the minimum positive integer delay in seconds
	// for the HTTP Retry-After header per RFC 6585 / RFC 7231.
	minRetryAfterSeconds = 1

	// inactivityWindowMultiple defines how many window durations a client entry
	// is kept before being considered completely stale and eligible for purge.
	inactivityWindowMultiple = 3

	// minCleanupInterval prevents excessive map scans when configured with very short windows.
	minCleanupInterval = time.Minute
)

func NewRateLimiter(policy RateLimitPolicy) (*RateLimiter, error) {
	if policy.Limit <= 0 {
		return nil, fmt.Errorf("rate limit must be positive")
	}
	if policy.Window <= 0 {
		return nil, fmt.Errorf("rate limit window must be positive")
	}

	retryAfterSeconds := max(int(math.Ceil(policy.Window.Seconds()/float64(policy.Limit))), minRetryAfterSeconds)

	cleanupInterval := max(policy.Window*inactivityWindowMultiple, minCleanupInterval)

	return &RateLimiter{
		policy:       policy,
		limit:        rate.Limit(float64(policy.Limit) / policy.Window.Seconds()),
		burst:        policy.Limit,
		retryAfter:   retryAfterSeconds,
		cleanupEvery: cleanupInterval,
		lastCleanup:  time.Now(),
		limiters:     make(map[string]*limiterEntry),
	}, nil
}

func (l *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key, ok := requestcontext.ClientIP(c.Request.Context())
		if !ok {
			err := fmt.Errorf("client IP missing from request context")
			response.Fail(c, http.StatusInternalServerError, response.InternalServerCode, "internal server error")
			c.Error(err)
			c.Abort()
			return
		}

		if !l.allow(key) {
			c.Header("Retry-After", strconv.Itoa(l.retryAfter))
			response.Fail(c, http.StatusTooManyRequests, response.RateLimitedCode, "rate limit exceeded")
			c.Abort()
			return
		}

		c.Next()
	}
}

func (l *RateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.cleanupLocked(now)

	entry, ok := l.limiters[key]
	if !ok {
		entry = &limiterEntry{
			limiter: rate.NewLimiter(l.limit, l.burst),
		}
		l.limiters[key] = entry
	}
	entry.lastSeen = now
	return entry.limiter.Allow()
}

func (l *RateLimiter) cleanupLocked(now time.Time) {
	if now.Sub(l.lastCleanup) < l.cleanupEvery {
		return
	}
	cutoff := now.Add(-l.cleanupEvery)
	for key, entry := range l.limiters {
		if entry.lastSeen.Before(cutoff) {
			delete(l.limiters, key)
		}
	}
	l.lastCleanup = now
}
