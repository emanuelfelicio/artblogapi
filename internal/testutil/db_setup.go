package testutil

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type nopLogger struct{}

func (n *nopLogger) Printf(format string, v ...any) {}

// NewTestDB starts a PostgreSQL test container, applies all migrations,
// and returns a ready-to-use pgx connection pool along with the container.
func NewTestDB(ctx context.Context) (*pgxpool.Pool, *postgres.PostgresContainer) {

	debug := os.Getenv("TEST_DEBUG") == "true"

	var containerOpts []testcontainers.ContainerCustomizer
	containerOpts = append(containerOpts,
		postgres.WithDatabase("artblog_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)

	if !debug {
		containerOpts = append(containerOpts, testcontainers.WithLogger(&nopLogger{}))
		goose.SetLogger(goose.NopLogger())
	}

	postgresContainer, err := postgres.Run(ctx, "postgres:16-alpine", containerOpts...)
	if err != nil {
		log.Fatalf("failed to start postgres container: %v\n", err)
	}

	dbURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("failed to get connection string: %v", err)
	}

	testDBPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to create pgx pool: %v", err)
	}
	sqlDB := stdlib.OpenDB(*testDBPool.Config().ConnConfig)
	defer sqlDB.Close()

	gooseDir := "../../db/migrations"
	if err = goose.Up(sqlDB, gooseDir); err != nil {
		log.Fatalf("goose up failed: %v", err)
	}

	return testDBPool, postgresContainer
}
