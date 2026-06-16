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
	}
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
