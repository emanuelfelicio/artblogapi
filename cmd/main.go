package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	loggercfg "github.com/emanuelfelicio/artblogapi/config/logger"
	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/emanuelfelicio/artblogapi/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "development"
	}
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = loggercfg.DefaultLevelByEnv(appEnv)
	}

	logger := loggercfg.New(appEnv, logLevel)
	slog.SetDefault(logger)
	logger.Info("starting_application", slog.String("env", appEnv), slog.String("log_level", logLevel))

	// Database configuration
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Error("missing_environment_variable", slog.String("name", "POSTGRES_CONNECTION"))
		os.Exit(1)
	}
	// init db
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logger.Error("database_connect_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("database_ping_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("database_connected")
	queries := dbgen.New(pool)

	// dependencies
	authRepo := auth.NewRepository(queries)
	jwtIssuer := os.Getenv("JWT_ISSUER")
	if jwtIssuer == "" {
		jwtIssuer = "artblogapi"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	authTokenProvider, err := auth.NewJWTTokenProvider([]byte(jwtSecret), jwtIssuer, 24*time.Hour)
	if err != nil {
		logger.Error("jwt_provider_config_invalid", slog.String("error", err.Error()))
		os.Exit(1)
	}
	authService := auth.NewService(authRepo, logger, authTokenProvider)
	authHandler := auth.NewHandler(authService)

	if strings.EqualFold(appEnv, "production") {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogger(logger))

	v1 := router.Group("/api/v1")
	{
		auth.Routes(v1, authHandler)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("server_listening", slog.String("port", port))
	if err := router.Run(":" + port); err != nil {
		logger.Error("server_start_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
