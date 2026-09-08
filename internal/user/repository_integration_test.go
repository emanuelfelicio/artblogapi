package user

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"

	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/emanuelfelicio/artblogapi/internal/storage"
	"github.com/emanuelfelicio/artblogapi/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	testDBPool      *pgxpool.Pool
	testRepo        Repository
	testStorageRepo storage.Repository
)

type nopLogger struct{}

func (n *nopLogger) Printf(format string, v ...any) {}

func TestMain(m *testing.M) {
	ctx := context.Background()
	var container *postgres.PostgresContainer
	testDBPool, container = testutil.NewTestDB(ctx)
	defer testDBPool.Close()
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate container: %v", err)
		}
	}()

	queries := dbgen.New(testDBPool)
	testRepo = NewRepository(queries, testDBPool)
	testStorageRepo = storage.NewRepository(queries, testDBPool)
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

func mustCreateUploadWithPurpose(t *testing.T, ctx context.Context, id uuid.UUID, userID uuid.UUID, key string, status string, purpose string) {
	t.Helper()
	_, err := testDBPool.Exec(ctx, `
		INSERT INTO uploads (id, user_id, object_key, status, purpose, file_size, content_type)
		VALUES ($1, $2, $3, $4, $5, 1024, 'image/png')
	`, id, userID, key, status, purpose)
	if err != nil {
		t.Fatalf("mustCreateUploadWithPurpose: %v", err)
	}
}

func getUploadStatus(t *testing.T, ctx context.Context, id uuid.UUID) string {
	t.Helper()
	var status string
	err := testDBPool.QueryRow(ctx, `SELECT status FROM uploads WHERE id = $1`, id).Scan(&status)
	if err != nil {
		t.Fatalf("getUploadStatus: %v", err)
	}
	return status
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
	})

	t.Run("resolves avatar and banner keys if bound", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		avatarID := uuid.New()
		bannerID := uuid.New()
		mustCreateUploadWithPurpose(t, ctx, avatarID, u.ID, "avatars/my-avatar.png", "BOUND", "AVATAR")
		mustCreateUploadWithPurpose(t, ctx, bannerID, u.ID, "banners/my-banner.png", "BOUND", "BANNER")

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
}

func TestRepository_FindUserByIDForUpdate(t *testing.T) {
	t.Run("locks and returns user media IDs for update", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		avatarID := uuid.New()
		mustCreateUploadWithPurpose(t, ctx, avatarID, u.ID, "avatars/pic.png", "BOUND", "AVATAR")
		_, err := testDBPool.Exec(ctx, `UPDATE users SET avatar_upload_id = $1 WHERE id = $2`, avatarID, u.ID)
		if err != nil {
			t.Fatalf("failed to update user avatar: %v", err)
		}

		userEntity, err := testRepo.FindUserByIDForUpdate(ctx, u.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if userEntity.ID != u.ID || userEntity.AvatarUploadID == nil || *userEntity.AvatarUploadID != avatarID {
			t.Errorf("unexpected user entity: %+v", userEntity)
		}
	})
}

func TestRepository_UpdateAvatarAndBanner(t *testing.T) {
	t.Run("updates user avatar and banner FKs directly", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		avatarID := uuid.New()
		bannerID := uuid.New()
		mustCreateUploadWithPurpose(t, ctx, avatarID, u.ID, "avatars/pic.png", "BOUND", "AVATAR")
		mustCreateUploadWithPurpose(t, ctx, bannerID, u.ID, "banners/pic.png", "BOUND", "BANNER")

		if err := testRepo.UpdateAvatar(ctx, u.ID, avatarID); err != nil {
			t.Fatalf("UpdateAvatar: %v", err)
		}
		if err := testRepo.UpdateBanner(ctx, u.ID, bannerID); err != nil {
			t.Fatalf("UpdateBanner: %v", err)
		}

		userEntity, err := testRepo.FindUserByIDForUpdate(ctx, u.ID)
		if err != nil {
			t.Fatalf("FindUserByIDForUpdate: %v", err)
		}
		if userEntity.AvatarUploadID == nil || *userEntity.AvatarUploadID != avatarID {
			t.Errorf("expected AvatarUploadID %v, got %v", avatarID, userEntity.AvatarUploadID)
		}
		if userEntity.BannerUploadID == nil || *userEntity.BannerUploadID != bannerID {
			t.Errorf("expected BannerUploadID %v, got %v", bannerID, userEntity.BannerUploadID)
		}
	})
}

func TestRepository_WithTransaction(t *testing.T) {
	t.Run("executes operations inside a single transaction with commit", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		uploadID := uuid.New()
		mustCreateUploadWithPurpose(t, ctx, uploadID, u.ID, "avatars/pic.png", "COMPLETED", "AVATAR")

		err := testRepo.WithTransaction(ctx, func(txCtx context.Context) error {
			if err := testStorageRepo.UpdateUploadStatus(txCtx, uploadID, storage.UploadStatusBOUND); err != nil {
				return err
			}
			return testRepo.UpdateAvatar(txCtx, u.ID, uploadID)
		})
		if err != nil {
			t.Fatalf("unexpected transaction error: %v", err)
		}

		if status := getUploadStatus(t, ctx, uploadID); status != "BOUND" {
			t.Errorf("expected status BOUND, got %s", status)
		}
	})

	t.Run("rolls back transaction when callback returns an error", func(t *testing.T) {
		ctx := setup(t)
		u := newTestUser()
		mustCreateUser(t, ctx, u)

		uploadID := uuid.New()
		mustCreateUploadWithPurpose(t, ctx, uploadID, u.ID, "avatars/pic.png", "COMPLETED", "AVATAR")

		forcedErr := errors.New("forced rollback")
		err := testRepo.WithTransaction(ctx, func(txCtx context.Context) error {
			if err := testStorageRepo.UpdateUploadStatus(txCtx, uploadID, storage.UploadStatusBOUND); err != nil {
				return err
			}
			return forcedErr
		})
		if !errors.Is(err, forcedErr) {
			t.Fatalf("expected forcedErr, got %v", err)
		}

		if status := getUploadStatus(t, ctx, uploadID); status != "COMPLETED" {
			t.Errorf("expected status to remain COMPLETED due to rollback, got %s", status)
		}
	})
}
