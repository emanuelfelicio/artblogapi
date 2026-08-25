package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	loggercfg "github.com/emanuelfelicio/artblogapi/config/logger"
	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv                  string
	LogLevel                string
	DatabaseURL             string
	JWTIssuer               string
	JWTSecret               string
	Port                    string
	AccessTokenTTL          time.Duration
	RefreshTokenTTL         time.Duration
	RefreshCookieDomain     string
	RefreshCookieSecure     bool
	CDNBaseURL              string
	DefaultAvatarURL        string
	DefaultBannerURL        string
	S3Endpoint              string
	S3Region                string
	S3Bucket                string
	S3AccessKey             string
	S3SecretKey             string
	S3PresignTTL            time.Duration
	S3ForcePathStyle        bool
	WorkerConcurrency       int
	WorkerTickerInterval    time.Duration
	WorkerStaleThreshold    time.Duration
	WorkerHeartbeatInterval time.Duration
	WorkerBackoffInterval   time.Duration
	WorkerMaxRetries        int
}

func LoadConfig() Config {
	_ = godotenv.Load()

	appEnv := getEnvOrDefault("APP_ENV", "development")
	logLevel := getEnvOrDefault("LOG_LEVEL", loggercfg.DefaultLevelByEnv(appEnv))
	databaseURL := getEnvOrDefault("DATABASE_URL", "")
	if strings.TrimSpace(databaseURL) == "" {
		log.Fatalf("missing required environment variable: DATABASE_URL")
	}

	jwtIssuer := getEnvOrDefault("JWT_ISSUER", "artblogapi")
	jwtSecret := getEnvOrDefault("JWT_SECRET", "")
	if strings.TrimSpace(jwtSecret) == "" {
		log.Fatalf("missing required environment variable: JWT_SECRET")
	}

	return Config{
		AppEnv:                  appEnv,
		LogLevel:                logLevel,
		DatabaseURL:             databaseURL,
		JWTIssuer:               jwtIssuer,
		JWTSecret:               jwtSecret,
		Port:                    getEnvOrDefault("PORT", "8080"),
		AccessTokenTTL:          15 * time.Minute,
		RefreshTokenTTL:         7 * 24 * time.Hour,
		RefreshCookieDomain:     getEnvOrDefault("REFRESH_TOKEN_COOKIE_DOMAIN", ""),
		RefreshCookieSecure:     getEnvOrDefault("REFRESH_TOKEN_COOKIE_SECURE", "false") == "true",
		CDNBaseURL:              normalizeBaseURL(getEnvOrDefault("CDN_BASE_URL", "http://localhost:9000")),
		DefaultAvatarURL:        getEnvOrDefault("DEFAULT_AVATAR_URL", "https://cdn.artblog.io/defaults/avatar.png"),
		DefaultBannerURL:        getEnvOrDefault("DEFAULT_BANNER_URL", "https://cdn.artblog.io/defaults/banner.png"),
		S3Endpoint:              getEnvOrDefault("S3_ENDPOINT", "http://localhost:9000"),
		S3Region:                getEnvOrDefault("S3_REGION", "us-east-1"),
		S3Bucket:                getEnvOrDefault("S3_BUCKET", "artblog"),
		S3AccessKey:             getEnvOrDefault("S3_ACCESS_KEY", ""),
		S3SecretKey:             getEnvOrDefault("S3_SECRET_KEY", ""),
		S3PresignTTL:            getEnvDurationOrDefault("S3_PRESIGN_TTL", 15*time.Minute),
		S3ForcePathStyle:        getEnvOrDefault("S3_FORCE_PATH_STYLE", "true") == "true",
		WorkerConcurrency:       getEnvIntOrDefault("STORAGE_WORKER_CONCURRENCY", 3),
		WorkerTickerInterval:    getEnvDurationOrDefault("STORAGE_WORKER_TICKER_INTERVAL", 60*time.Second),
		WorkerStaleThreshold:    getEnvDurationOrDefault("STORAGE_WORKER_STALE_THRESHOLD", 30*time.Second),
		WorkerHeartbeatInterval: getEnvDurationOrDefault("STORAGE_WORKER_HEARTBEAT_INTERVAL", 10*time.Second),
		WorkerBackoffInterval:   getEnvDurationOrDefault("STORAGE_WORKER_BACKOFF_INTERVAL", 10*time.Second),
		WorkerMaxRetries:        getEnvIntOrDefault("STORAGE_MAX_RETRIES", 3),
	}
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvDurationOrDefault(key string, fallback time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(val)
	if err != nil {
		log.Fatalf("invalid duration for %s: %v", key, err)
	}
	return parsed
}

func getEnvIntOrDefault(key string, fallback int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(val)
	if err != nil {
		log.Fatalf("invalid integer for %s: %v", key, err)
	}
	return parsed
}

func normalizeBaseURL(url string) string {
	return strings.TrimRight(url, "/")
}
