package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/emanuelfelicio/artblogapi/config"
	loggercfg "github.com/emanuelfelicio/artblogapi/config/logger"
	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/emanuelfelicio/artblogapi/internal/auth/token"
	"github.com/emanuelfelicio/artblogapi/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.LoadConfig()

	logger := loggercfg.New(cfg.AppEnv, cfg.LogLevel)
	slog.SetDefault(logger)
	logger.Info("starting_application", slog.String("env", cfg.AppEnv), slog.String("log_level", cfg.LogLevel))

	// Database configuration
	// init db
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
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
	authRepo := auth.NewRepository(queries, logger, pool)

	authTokenProvider, err := token.NewTokenProvider([]byte(cfg.JWTSecret), cfg.JWTIssuer, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	if err != nil {
		logger.Error("token_provider_config_invalid", slog.String("error", err.Error()))
		os.Exit(1)
	}
	authService := auth.NewService(authRepo, logger, authTokenProvider)
	refreshCookieCfg := auth.NewRefreshCookieConfig(cfg.RefreshCookieDomain, cfg.RefreshCookieSecure)
	authHandler := auth.NewHandler(authService, logger, refreshCookieCfg)

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestLogger(logger))

	authMiddleware := middleware.Authentication(authTokenProvider)
	v1 := router.Group("/api/v1")
	{
		auth.Routes(v1, authHandler, authMiddleware)
	}

	logger.Info("server_listening", slog.String("port", cfg.Port))
	if err := router.Run(":" + cfg.Port); err != nil {
		logger.Error("server_start_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
