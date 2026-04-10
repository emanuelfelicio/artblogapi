package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

type responseWriter struct {
	gin.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: c.Writer, status: 200}
		c.Writer = wrapped

		c.Next()

		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("route", c.FullPath()),
			slog.Int("status", wrapped.status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		}

		if requestID := c.GetHeader("X-Request-Id"); requestID != "" {
			attrs = append(attrs, slog.String("request_id", requestID))
		}

		if len(c.Errors) > 0 {
			logger.Error("request_finished_with_error", slog.Any("errors", c.Errors.String()), slog.Attr{Key: "http", Value: slog.GroupValue(attrs...)})
			return
		}

		logger.Info("request_finished", slog.Attr{Key: "http", Value: slog.GroupValue(attrs...)})
	}
}
