package storage

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/emanuelfelicio/artblogapi/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	testDBPool *pgxpool.Pool
	testRepo   *repository
)

func TestMain(m *testing.M) {
	//gin for handler test
	gin.SetMode(gin.TestMode)

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

func mustCreateUser(t *testing.T, ctx context.Context) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	username := "u_" + uuid.NewString()[:8]
	email := username + "@example.com"
	_, err := testDBPool.Exec(ctx, `
		INSERT INTO users (id, username, email, password_hash, display_name, bio, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, true)
	`, userID, username, email, "hashed_pass", "Test User", "Test Bio")
	if err != nil {
		t.Fatalf("mustCreateUser: %v", err)
	}
	return userID
}

func newTestUpload(t *testing.T, userID uuid.UUID) Upload {
	t.Helper()
	u, err := NewUpload(userID, PurposeAVATAR, 1024, ContentTypePNG)
	if err != nil {
		t.Fatalf("newTestUpload: %v", err)
	}
	return u
}

func TestRepository_Create(t *testing.T) {
	t.Run("creates upload and returns generated ID", func(t *testing.T) {
		ctx := setup(t)
		userID := mustCreateUser(t, ctx)
		u := newTestUpload(t, userID)

		id, err := testRepo.Create(ctx, u)
		if err != nil {
			t.Fatalf("Create upload failed: %v", err)
		}
		if id != u.ID {
			t.Fatalf("expected ID %v, got %v", u.ID, id)
		}

		fetched, err := testRepo.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if fetched.ID != u.ID {
			t.Errorf("ID mismatch: got %v, want %v", fetched.ID, u.ID)
		}
		if fetched.UserID != u.UserID {
			t.Errorf("UserID mismatch: got %v, want %v", fetched.UserID, u.UserID)
		}
		if fetched.ObjectKey != u.ObjectKey {
			t.Errorf("ObjectKey mismatch: got %v, want %v", fetched.ObjectKey, u.ObjectKey)
		}
		if fetched.Status != u.Status {
			t.Errorf("Status mismatch: got %v, want %v", fetched.Status, u.Status)
		}
		if fetched.Purpose != u.Purpose {
			t.Errorf("Purpose mismatch: got %v, want %v", fetched.Purpose, u.Purpose)
		}
		if fetched.FileSize != u.FileSize {
			t.Errorf("FileSize mismatch: got %v, want %v", fetched.FileSize, u.FileSize)
		}
		if fetched.ContentType != u.ContentType {
			t.Errorf("ContentType mismatch: got %v, want %v", fetched.ContentType, u.ContentType)
		}
	})
}

func TestRepository_GetByID(t *testing.T) {
	t.Run("returns ErrUploadNotFound when ID does not exist", func(t *testing.T) {
		ctx := setup(t)

		_, err := testRepo.GetByID(ctx, uuid.New())
		if !errors.Is(err, ErrUploadNotFound) {
			t.Fatalf("expected ErrUploadNotFound, got %v", err)
		}
	})

	t.Run("fetches upload with failure reason mapped when present", func(t *testing.T) {
		ctx := setup(t)
		userID := mustCreateUser(t, ctx)
		u := newTestUpload(t, userID)

		_, err := testRepo.Create(ctx, u)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		reason := "invalid image header"
		if err := testRepo.RejectUpload(ctx, u.ID, reason); err != nil {
			t.Fatalf("RejectUpload: %v", err)
		}

		fetched, err := testRepo.GetByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if fetched.Status != UploadStatusREJECTED {
			t.Errorf("Status: got %v, want REJECTED", fetched.Status)
		}
		if fetched.FailureReason == nil || *fetched.FailureReason != reason {
			t.Errorf("FailureReason: got %v, want %s", fetched.FailureReason, reason)
		}
	})
}

func TestRepository_SetStatusProcessing(t *testing.T) {
	t.Run("updates upload status to PROCESSING", func(t *testing.T) {
		ctx := setup(t)
		userID := mustCreateUser(t, ctx)
		u := newTestUpload(t, userID)

		_, err := testRepo.Create(ctx, u)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		if err := testRepo.SetStatusProcessing(ctx, u.ID); err != nil {
			t.Fatalf("SetStatusProcessing: %v", err)
		}

		fetched, err := testRepo.GetByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if fetched.Status != UploadStatusPROCESSING {
			t.Fatalf("expected status PROCESSING, got %v", fetched.Status)
		}
	})
}

func TestRepository_GetNextProcessingJob(t *testing.T) {
	t.Run("returns job when upload is in PROCESSING state", func(t *testing.T) {
		ctx := setup(t)
		userID := mustCreateUser(t, ctx)
		u := newTestUpload(t, userID)

		_, err := testRepo.Create(ctx, u)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		if err := testRepo.SetStatusProcessing(ctx, u.ID); err != nil {
			t.Fatalf("SetStatusProcessing: %v", err)
		}

		job, found, err := testRepo.GetNextProcessingJob(ctx, 3, 30*time.Second)
		if err != nil {
			t.Fatalf("GetNextProcessingJob: %v", err)
		}
		if !found {
			t.Fatalf("expected job to be found")
		}
		if job.ID != u.ID {
			t.Errorf("job ID mismatch: got %v, want %v", job.ID, u.ID)
		}

		// Second fetch should find no more jobs because the job is locked / heartbeat updated
		_, foundAgain, err := testRepo.GetNextProcessingJob(ctx, 3, 30*time.Second)
		if err != nil {
			t.Fatalf("GetNextProcessingJob second call: %v", err)
		}
		if foundAgain {
			t.Fatalf("expected foundAgain to be false")
		}
	})

	t.Run("returns found=false when no processing jobs exist", func(t *testing.T) {
		ctx := setup(t)

		_, found, err := testRepo.GetNextProcessingJob(ctx, 3, 30*time.Second)
		if err != nil {
			t.Fatalf("GetNextProcessingJob: %v", err)
		}
		if found {
			t.Fatalf("expected found to be false")
		}
	})
}

func TestRepository_HeartbeatUploadProcessing(t *testing.T) {
	t.Run("updates heartbeat_at for processing upload", func(t *testing.T) {
		ctx := setup(t)
		userID := mustCreateUser(t, ctx)
		u := newTestUpload(t, userID)

		_, err := testRepo.Create(ctx, u)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		if err := testRepo.SetStatusProcessing(ctx, u.ID); err != nil {
			t.Fatalf("SetStatusProcessing: %v", err)
		}

		if err := testRepo.HeartbeatUploadProcessing(ctx, u.ID); err != nil {
			t.Fatalf("HeartbeatUploadProcessing failed: %v", err)
		}
	})
}

func TestRepository_UpdateUploadCompletion(t *testing.T) {
	t.Run("updates upload with final object key, content type and status", func(t *testing.T) {
		ctx := setup(t)
		userID := mustCreateUser(t, ctx)
		u := newTestUpload(t, userID)

		_, err := testRepo.Create(ctx, u)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		finalKey := BuildFinalKey(u.ID)
		if err := testRepo.UpdateUploadCompletion(ctx, u.ID, finalKey, ContentTypeWebP, UploadStatusCOMPLETED); err != nil {
			t.Fatalf("UpdateUploadCompletion: %v", err)
		}

		fetched, err := testRepo.GetByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if fetched.Status != UploadStatusCOMPLETED {
			t.Errorf("Status: got %v, want COMPLETED", fetched.Status)
		}
		if fetched.ObjectKey != finalKey {
			t.Errorf("ObjectKey: got %v, want %s", fetched.ObjectKey, finalKey)
		}
		if fetched.ContentType != ContentTypeWebP {
			t.Errorf("ContentType: got %v, want image/webp", fetched.ContentType)
		}
	})
}

func TestRepository_IncrementRetry(t *testing.T) {
	t.Run("increments retry count and sets backoff", func(t *testing.T) {
		ctx := setup(t)
		userID := mustCreateUser(t, ctx)
		u := newTestUpload(t, userID)

		_, err := testRepo.Create(ctx, u)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		if err := testRepo.SetStatusProcessing(ctx, u.ID); err != nil {
			t.Fatalf("SetStatusProcessing: %v", err)
		}

		backoff := 10 * time.Second
		if err := testRepo.IncrementRetry(ctx, u.ID, backoff); err != nil {
			t.Fatalf("IncrementRetry: %v", err)
		}

		// Re-fetch processing job with 0 stale threshold -> should not be eligible yet due to future next_retry_at
		_, found, err := testRepo.GetNextProcessingJob(ctx, 3, 0)
		if err != nil {
			t.Fatalf("GetNextProcessingJob: %v", err)
		}
		if found {
			t.Errorf("expected job to be backed off and not immediately eligible")
		}
	})
}
