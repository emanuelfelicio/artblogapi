package config

import (
	"log"
	"os"
	"strings"
	"time"

	loggercfg "github.com/emanuelfelicio/artblogapi/config/logger"
	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv              string
	LogLevel            string
	DatabaseURL         string
	JWTIssuer           string
	JWTSecret           string
	Port                string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	RefreshCookieDomain string
	RefreshCookieSecure bool
	CDNBaseURL          string
	DefaultAvatarURL    string
	DefaultBannerURL    string
	S3Endpoint          string
	S3Region            string
	S3Bucket            string
	S3AccessKey         string
	S3SecretKey         string
	S3PresignTTL        time.Duration
	S3ForcePathStyle    bool
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
		AppEnv:              appEnv,
		LogLevel:            logLevel,
		DatabaseURL:         databaseURL,
		JWTIssuer:           jwtIssuer,
		JWTSecret:           jwtSecret,
		Port:                getEnvOrDefault("PORT", "8080"),
		AccessTokenTTL:      15 * time.Minute,
		RefreshTokenTTL:     7 * 24 * time.Hour,
		RefreshCookieDomain: getEnvOrDefault("REFRESH_TOKEN_COOKIE_DOMAIN", ""),
		RefreshCookieSecure: getEnvOrDefault("REFRESH_TOKEN_COOKIE_SECURE", "false") == "true",
		CDNBaseURL:          getEnvOrDefault("CDN_BASE_URL", "http://localhost:9000/final"),
		DefaultAvatarURL:    getEnvOrDefault("DEFAULT_AVATAR_URL", "https://cdn.artblog.io/defaults/avatar.png"),
		DefaultBannerURL:    getEnvOrDefault("DEFAULT_BANNER_URL", "https://cdn.artblog.io/defaults/banner.png"),
		S3Endpoint:          getEnvOrDefault("S3_ENDPOINT", "http://localhost:9000"),
		S3Region:            getEnvOrDefault("S3_REGION", "us-east-1"),
		S3Bucket:            getEnvOrDefault("S3_BUCKET", "artblog"),
		S3AccessKey:         getEnvOrDefault("S3_ACCESS_KEY", ""),
		S3SecretKey:         getEnvOrDefault("S3_SECRET_KEY", ""),
		S3PresignTTL:        getEnvDurationOrDefault("S3_PRESIGN_TTL", 15*time.Minute),
		S3ForcePathStyle:    getEnvOrDefault("S3_FORCE_PATH_STYLE", "true") == "true",
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
