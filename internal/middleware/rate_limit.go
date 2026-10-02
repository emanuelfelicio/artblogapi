package middleware

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emanuelfelicio/artblogapi/config/response"
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

func (l *RateLimiter) Middleware(clientIP func(*http.Request) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := clientIP(c.Request)
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

func ClientIP(trustedProxyCIDRs []string) (func(*http.Request) string, error) {
	trustedProxies, err := parseTrustedProxyCIDRs(trustedProxyCIDRs)
	if err != nil {
		return nil, err
	}

	return func(request *http.Request) string {
		remoteIP := remoteIP(request.RemoteAddr)
		if len(trustedProxies) == 0 || remoteIP == nil || !containsIP(trustedProxies, remoteIP) {
			return remoteIPString(remoteIP, request.RemoteAddr)
		}

		if forwardedIP := forwardedClientIP(request, trustedProxies); forwardedIP != nil {
			return forwardedIP.String()
		}
		return remoteIPString(remoteIP, request.RemoteAddr)
	}, nil
}

func parseTrustedProxyCIDRs(cidrs []string) ([]*net.IPNet, error) {
	proxies := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", cidr, err)
		}
		proxies = append(proxies, network)
	}
	return proxies, nil
}

func forwardedClientIP(request *http.Request, trustedProxies []*net.IPNet) net.IP {
	var candidates []net.IP
	for value := range strings.SplitSeq(request.Header.Get("X-Forwarded-For"), ",") {
		if ip := net.ParseIP(strings.TrimSpace(value)); ip != nil {
			candidates = append(candidates, ip)
		}
	}

	for index := len(candidates) - 1; index >= 0; index-- {
		if !containsIP(trustedProxies, candidates[index]) {
			return candidates[index]
		}
	}

	if realIP := net.ParseIP(strings.TrimSpace(request.Header.Get("X-Real-IP"))); realIP != nil {
		return realIP
	}
	return nil
}

func remoteIP(address string) net.IP {
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return net.ParseIP(host)
	}
	return net.ParseIP(address)
}

func remoteIPString(ip net.IP, fallback string) string {
	if ip != nil {
		return ip.String()
	}
	return fallback
}

func containsIP(networks []*net.IPNet, ip net.IP) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
