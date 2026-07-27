package user

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testDBPool *pgxpool.Pool
	testRepo   Repository
)

type nopLogger struct{}

func (n *nopLogger) Printf(format string, v ...any) {}

func TestMain(m *testing.M) {
	ctx := context.Background()

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
		os.Stderr.WriteString("failed to start postgres container: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			os.Stderr.WriteString("failed to terminate container: " + err.Error() + "\n")
		}
	}()

	dbURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		os.Stderr.WriteString("failed to get connection string: " + err.Error() + "\n")
		os.Exit(1)
	}

	testDBPool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		os.Stderr.WriteString("failed to create pgx pool: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer testDBPool.Close()

	sqlDB := stdlib.OpenDB(*testDBPool.Config().ConnConfig)
	defer sqlDB.Close()

	gooseDir := "../../db/migrations"
	if err = goose.Up(sqlDB, gooseDir); err != nil {
		os.Stderr.WriteString("goose up failed: " + err.Error() + "\n")
		os.Exit(1)
	}

	queries := dbgen.New(testDBPool)
	testRepo = NewRepository(queries)

	os.Exit(m.Run())
}

func setup(t *testing.T) context.Context {
	t.Helper()
	t.Cleanup(func() {
		_, err := testDBPool.Exec(context.Background(), `TRUNCATE TABLE users, uploads RESTART IDENTITY CASCADE`)
		if err != nil {
			t.Errorf("failed to truncate tables: %v", err)
		}
	})
	return context.Background()
}

func newTestUser() User {
	return User{
		ID:          uuid.New(),
		Username:    "u_" + uuid.NewString()[:8],
		Email:       "u_" + uuid.NewString()[:8] + "@example.com",
		DisplayName: "Test User",
		Bio:         "Test Bio",
		IsActive:    true,
	}
}

func mustCreateUser(t *testing.T, ctx context.Context, u User) {
	t.Helper()
	_, err := testDBPool.Exec(ctx, `
		INSERT INTO users (id, username, email, password_hash, display_name, bio, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, u.ID, u.Username, u.Email, "hashed_password", u.DisplayName, u.Bio, u.IsActive)
	if err != nil {
		t.Fatalf("mustCreateUser: %v", err)
	}
}

func mustCreateUpload(t *testing.T, ctx context.Context, id uuid.UUID, userID uuid.UUID, key string, status string) {
	t.Helper()
	_, err := testDBPool.Exec(ctx, `
		INSERT INTO uploads (id, user_id, object_key, status, file_size, content_type)
		VALUES ($1, $2, $3, $4, 1024, 'image/png')
	`, id, userID, key, status)
	if err != nil {
		t.Fatalf("mustCreateUpload: %v", err)
	}
}

func TestRepository_FindByUsername(t *testing.T) {
	t.Run("returns user correctly when active", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		found, err := testRepo.FindByUsername(ctx, u.Username)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if found.ID != u.ID {
			t.Errorf("expected ID %v, got %v", u.ID, found.ID)
		}
		if found.Username != u.Username {
			t.Errorf("expected Username %s, got %s", u.Username, found.Username)
		}
		if found.Email != u.Email {
			t.Errorf("expected Email %s, got %s", u.Email, found.Email)
		}
		if found.DisplayName != u.DisplayName {
			t.Errorf("expected DisplayName %s, got %s", u.DisplayName, found.DisplayName)
		}
		if found.Bio != u.Bio {
			t.Errorf("expected Bio %s, got %s", u.Bio, found.Bio)
		}
		if found.AvatarKey != nil {
			t.Errorf("expected AvatarKey to be nil, got %s", *found.AvatarKey)
		}
		if found.BannerKey != nil {
			t.Errorf("expected BannerKey to be nil, got %s", *found.BannerKey)
		}
	})

	t.Run("returns ErrUserNotFound when inactive", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		u.IsActive = false
		mustCreateUser(t, ctx, u)

		_, err := testRepo.FindByUsername(ctx, u.Username)
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("returns ErrUserNotFound when username does not exist", func(t *testing.T) {
		ctx := setup(t)
		_, err := testRepo.FindByUsername(ctx, "nonexistent")
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("resolves avatar and banner keys if completed", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		avatarID := uuid.New()
		bannerID := uuid.New()
		mustCreateUpload(t, ctx, avatarID, u.ID, "avatars/my-avatar.png", "COMPLETED")
		mustCreateUpload(t, ctx, bannerID, u.ID, "banners/my-banner.png", "COMPLETED")

		_, err := testDBPool.Exec(ctx, `
			UPDATE users SET avatar_upload_id = $1, banner_upload_id = $2 WHERE id = $3
		`, avatarID, bannerID, u.ID)
		if err != nil {
			t.Fatalf("failed to update user uploads: %v", err)
		}

		found, err := testRepo.FindByUsername(ctx, u.Username)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if found.AvatarKey == nil || *found.AvatarKey != "avatars/my-avatar.png" {
			t.Errorf("expected AvatarKey 'avatars/my-avatar.png', got %v", found.AvatarKey)
		}
		if found.BannerKey == nil || *found.BannerKey != "banners/my-banner.png" {
			t.Errorf("expected BannerKey 'banners/my-banner.png', got %v", found.BannerKey)
		}
	})

	t.Run("does not resolve keys if upload is not completed", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		avatarID := uuid.New()
		mustCreateUpload(t, ctx, avatarID, u.ID, "avatars/my-avatar.png", "PENDING")

		_, err := testDBPool.Exec(ctx, `
			UPDATE users SET avatar_upload_id = $1 WHERE id = $2
		`, avatarID, u.ID)
		if err != nil {
			t.Fatalf("failed to update user upload: %v", err)
		}

		found, err := testRepo.FindByUsername(ctx, u.Username)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if found.AvatarKey != nil {
			t.Errorf("expected AvatarKey to be nil because upload is not completed, got %v", *found.AvatarKey)
		}
	})
}

func TestRepository_FindByID(t *testing.T) {
	t.Run("returns user correctly when exists", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		found, err := testRepo.FindByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if found.ID != u.ID {
			t.Errorf("expected ID %v, got %v", u.ID, found.ID)
		}
		if found.Username != u.Username {
			t.Errorf("expected Username %s, got %s", u.Username, found.Username)
		}
	})

	t.Run("returns user even if inactive", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		u.IsActive = false
		mustCreateUser(t, ctx, u)

		found, err := testRepo.FindByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found.ID != u.ID {
			t.Errorf("expected ID %v, got %v", u.ID, found.ID)
		}
	})

	t.Run("returns ErrUserNotFound when ID does not exist", func(t *testing.T) {
		ctx := setup(t)
		_, err := testRepo.FindByID(ctx, uuid.New())
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestRepository_UpdateProfile(t *testing.T) {
	t.Run("updates display name and bio correctly", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		newDisplayName := "Updated Name"
		newBio := "Updated Bio"

		updated, err := testRepo.UpdateProfile(ctx, u.ID, &newDisplayName, &newBio)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updated.DisplayName != newDisplayName {
			t.Errorf("expected DisplayName %s, got %s", newDisplayName, updated.DisplayName)
		}
		if updated.Bio != newBio {
			t.Errorf("expected Bio %s, got %s", newBio, updated.Bio)
		}
	})

	t.Run("updates only display name when bio is nil", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		newDisplayName := "Just Display Name"

		updated, err := testRepo.UpdateProfile(ctx, u.ID, &newDisplayName, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updated.DisplayName != newDisplayName {
			t.Errorf("expected DisplayName %s, got %s", newDisplayName, updated.DisplayName)
		}
		if updated.Bio != u.Bio {
			t.Errorf("expected Bio to remain %s, got %s", u.Bio, updated.Bio)
		}
	})

	t.Run("updates only bio when display name is nil", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		newBio := "Just Bio"

		updated, err := testRepo.UpdateProfile(ctx, u.ID, nil, &newBio)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updated.DisplayName != u.DisplayName {
			t.Errorf("expected DisplayName to remain %s, got %s", u.DisplayName, updated.DisplayName)
		}
		if updated.Bio != newBio {
			t.Errorf("expected Bio %s, got %s", newBio, updated.Bio)
		}
	})

	t.Run("returns error when user does not exist", func(t *testing.T) {
		ctx := setup(t)
		nonExistentID := uuid.New()
		newDisplayName := "Nobody"

		_, err := testRepo.UpdateProfile(ctx, nonExistentID, &newDisplayName, nil)
		if err == nil {
			t.Fatal("expected error when user does not exist, got nil")
		}
	})
}

func TestRepository_UpdateAvatar(t *testing.T) {
	t.Run("updates avatar upload id successfully", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		avatarID := uuid.New()
		mustCreateUpload(t, ctx, avatarID, u.ID, "avatars/my-avatar.png", "COMPLETED")

		err := testRepo.UpdateAvatar(ctx, u.ID, avatarID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		updatedUser, err := testRepo.FindByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("failed to find user: %v", err)
		}

		if updatedUser.AvatarKey == nil || *updatedUser.AvatarKey != "avatars/my-avatar.png" {
			t.Errorf("expected AvatarKey 'avatars/my-avatar.png', got %v", updatedUser.AvatarKey)
		}
	})

	t.Run("returns error when upload does not exist", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		nonExistentUploadID := uuid.New()

		err := testRepo.UpdateAvatar(ctx, u.ID, nonExistentUploadID)
		if err == nil {
			t.Fatal("expected error when setting non-existent upload as avatar, got nil")
		}
	})
}

func TestRepository_UpdateBanner(t *testing.T) {
	t.Run("updates banner upload id successfully", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		bannerID := uuid.New()
		mustCreateUpload(t, ctx, bannerID, u.ID, "banners/my-banner.png", "COMPLETED")

		err := testRepo.UpdateBanner(ctx, u.ID, bannerID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		updatedUser, err := testRepo.FindByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("failed to find user: %v", err)
		}

		if updatedUser.BannerKey == nil || *updatedUser.BannerKey != "banners/my-banner.png" {
			t.Errorf("expected BannerKey 'banners/my-banner.png', got %v", updatedUser.BannerKey)
		}
	})

	t.Run("returns error when upload does not exist", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		nonExistentUploadID := uuid.New()

		err := testRepo.UpdateBanner(ctx, u.ID, nonExistentUploadID)
		if err == nil {
			t.Fatal("expected error when setting non-existent upload as banner, got nil")
		}
	})
}

func TestRepository_FindCompletedUploadByOwner(t *testing.T) {
	t.Run("succeeds when completed upload owned by user exists", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		uploadID := uuid.New()
		mustCreateUpload(t, ctx, uploadID, u.ID, "images/pic.png", "COMPLETED")

		err := testRepo.FindCompletedUploadByOwner(ctx, uploadID, u.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("returns ErrUploadNotFound when status is not COMPLETED", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		uploadID := uuid.New()
		mustCreateUpload(t, ctx, uploadID, u.ID, "images/pic.png", "PENDING")

		err := testRepo.FindCompletedUploadByOwner(ctx, uploadID, u.ID)
		if !errors.Is(err, ErrUploadNotFound) {
			t.Fatalf("expected ErrUploadNotFound, got %v", err)
		}
	})

	t.Run("returns ErrUploadNotFound when owned by another user", func(t *testing.T) {
		ctx := setup(t)
		u1 := newTestUser()
		mustCreateUser(t, ctx, u1)

		u2 := newTestUser()
		mustCreateUser(t, ctx, u2)

		uploadID := uuid.New()
		mustCreateUpload(t, ctx, uploadID, u1.ID, "images/pic.png", "COMPLETED")

		err := testRepo.FindCompletedUploadByOwner(ctx, uploadID, u2.ID)
		if !errors.Is(err, ErrUploadNotFound) {
			t.Fatalf("expected ErrUploadNotFound, got %v", err)
		}
	})

	t.Run("returns ErrUploadNotFound when upload does not exist", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		err := testRepo.FindCompletedUploadByOwner(ctx, uuid.New(), u.ID)
		if !errors.Is(err, ErrUploadNotFound) {
			t.Fatalf("expected ErrUploadNotFound, got %v", err)
		}
	})
}
