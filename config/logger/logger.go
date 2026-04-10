package logger

import (
	"log/slog"
	"os"
	"strings"
)

func New(env string, level string) *slog.Logger {
	resolvedLevel := parseLevel(level)
	handlerOptions := &slog.HandlerOptions{Level: resolvedLevel}

	if strings.EqualFold(env, "production") {
		return slog.New(slog.NewJSONHandler(os.Stdout, handlerOptions))
	}

	return slog.New(slog.NewTextHandler(os.Stdout, handlerOptions))
}

func DefaultLevelByEnv(env string) string {
	if strings.EqualFold(env, "production") {
		return "WARN"
	}
	return "INFO"
}

func parseLevel(level string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
