package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/emanuelfelicio/artblogapi/internal/middleware/requestcontext"
	"github.com/gin-gonic/gin"
)

func ClientIPMiddleware(trustedProxyCIDRs []string) (gin.HandlerFunc, error) {
	clientIP, err := ClientIP(trustedProxyCIDRs)
	if err != nil {
		return nil, err
	}

	return func(c *gin.Context) {
		ip := clientIP(c.Request)
		ctx := requestcontext.WithClientIP(c.Request.Context(), ip)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}, nil
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
