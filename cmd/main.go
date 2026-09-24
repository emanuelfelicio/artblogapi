package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/emanuelfelicio/artblogapi/config"
	loggercfg "github.com/emanuelfelicio/artblogapi/config/logger"
	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	_ "github.com/emanuelfelicio/artblogapi/docs/swagger"
	"github.com/emanuelfelicio/artblogapi/internal/auth"
	"github.com/emanuelfelicio/artblogapi/internal/auth/token"
	"github.com/emanuelfelicio/artblogapi/internal/middleware"
	"github.com/emanuelfelicio/artblogapi/internal/post"
	"github.com/emanuelfelicio/artblogapi/internal/user"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/emanuelfelicio/artblogapi/internal/storage"
	storageImage "github.com/emanuelfelicio/artblogapi/internal/storage/image"
	storageS3 "github.com/emanuelfelicio/artblogapi/internal/storage/s3"
	"github.com/emanuelfelicio/artblogapi/internal/storage/worker"
)

// @title						Artblog API
// @version					1.0
// @description				Social media API for posts, authentication, and feed management
// @host						localhost:8080
// @BasePath					/api/v1
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
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
	userRepo := user.NewRepository(queries, pool)

	authTokenProvider, err := token.NewTokenProvider([]byte(cfg.JWTSecret), cfg.JWTIssuer, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	if err != nil {
		logger.Error("token_provider_config_invalid", slog.String("error", err.Error()))
		os.Exit(1)
	}
	authService := auth.NewService(authRepo, logger, authTokenProvider)
	refreshCookieCfg := auth.NewRefreshCookieConfig(cfg.RefreshCookieDomain, cfg.RefreshCookieSecure)
	authHandler := auth.NewHandler(authService, logger, refreshCookieCfg)
	// S3 Client Bootstrap
	s3Client, err := storageS3.InitS3Client(
		ctx,
		cfg.S3Endpoint,
		cfg.S3Region,
		cfg.S3AccessKey,
		cfg.S3SecretKey,
		cfg.S3ForcePathStyle,
	)
	if err != nil {
		logger.Error("s3_client_init_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("s3_client_initialized")

	// Storage Dependencies & Background Worker
	storageRepo := storage.NewRepository(queries, pool)
	storageProvider := storageS3.NewS3StorageProvider(
		s3Client,
		awsS3.NewPresignClient(s3Client),
		cfg.S3Bucket,
	)
	imageProcessor := storageImage.NewImagingProcessor()
	workerTriggerChan := make(chan struct{}, cfg.WorkerConcurrency)
	workerCfg := worker.Config{
		Concurrency:       cfg.WorkerConcurrency,
		TickerInterval:    cfg.WorkerTickerInterval,
		StaleThreshold:    cfg.WorkerStaleThreshold,
		HeartbeatInterval: cfg.WorkerHeartbeatInterval,
		BackoffInterval:   cfg.WorkerBackoffInterval,
		MaxRetries:        cfg.WorkerMaxRetries,
	}
	imageWorker := worker.New(
		storageRepo,
		storageProvider,
		imageProcessor,
		workerTriggerChan,
		workerCfg,
		logger,
	)

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	go imageWorker.Start(workerCtx)

	storageService := storage.NewService(
		storageRepo,
		storageProvider,
		imageWorker,
		cfg.S3PresignTTL,
		logger,
	)
	storageHandler := storage.NewHandler(storageService, logger)

	userService := user.NewService(userRepo, storageService)
	userHandler := user.NewHandler(userService, logger, cfg.StoragePublicURL, cfg.DefaultAvatarURL, cfg.DefaultBannerURL)

	postRepo := post.NewRepository(queries, pool)
	postService := post.NewService(postRepo, storageService)
	postHandler := post.NewHandler(postService, logger, cfg.StoragePublicURL)

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
		user.Routes(v1, userHandler, authMiddleware)
		storage.Routes(v1, storageHandler, authMiddleware)
		post.Routes(v1, postHandler, authMiddleware)
	}
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		logger.Info("server_listening", slog.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server_start_failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Gracefull shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting_down_application")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server_shutdown_error", slog.String("error", err.Error()))
	}

	cancelWorker()
	imageWorker.Wait()
	logger.Info("application_stopped")
}
