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
	AppEnv         string
	LogLevel       string
	DatabaseURL    string
	JWTIssuer      string
	JWTSecret      string
	Port           string
	AccessTokenTTL time.Duration
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
		AppEnv:         appEnv,
		LogLevel:       logLevel,
		DatabaseURL:    databaseURL,
		JWTIssuer:      jwtIssuer,
		JWTSecret:      jwtSecret,
		Port:           getEnvOrDefault("PORT", "8080"),
		AccessTokenTTL: 24 * time.Hour,
	}
}

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
