package post

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"

	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/emanuelfelicio/artblogapi/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	testDBPool *pgxpool.Pool
	testRepo   *repository
)

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
	os.Exit(m.Run())
}

func setup(t *testing.T) context.Context {
	t.Helper()
	t.Cleanup(func() {
		_, err := testDBPool.Exec(context.Background(), `TRUNCATE TABLE users, uploads, posts, post_images RESTART IDENTITY CASCADE`)
		if err != nil {
			t.Errorf("failed to truncate tables: %v", err)
		}
	})
	return context.Background()
}

func mustCreateUser(t *testing.T, ctx context.Context, id uuid.UUID, username string) {
	t.Helper()
	_, err := testDBPool.Exec(ctx, `
		INSERT INTO users (id, username, email, password_hash, display_name, bio, is_active)
		VALUES ($1, $2, $3, 'hash', 'Display', 'Bio', true)
	`, id, username, username+"@example.com")
	if err != nil {
		t.Fatalf("mustCreateUser: %v", err)
	}
}

func mustCreateUpload(t *testing.T, ctx context.Context, id, userID uuid.UUID, key string) {
	t.Helper()
	_, err := testDBPool.Exec(ctx, `
		INSERT INTO uploads (id, user_id, object_key, status, purpose, file_size, content_type)
		VALUES ($1, $2, $3, 'BOUND', 'POST_IMAGE', 1024, 'image/jpeg')
	`, id, userID, key)
	if err != nil {
		t.Fatalf("mustCreateUpload: %v", err)
	}
}

func TestRepository_CreateAndGetPost(t *testing.T) {
	ctx := setup(t)
	authorID := uuid.New()
	mustCreateUser(t, ctx, authorID, "author_one")

	postID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7: %v", err)
	}

	created, err := testRepo.CreatePost(ctx, postID, authorID, "Title 1", "Content 1")
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	if created.ID != postID {
		t.Errorf("expected post ID %s, got %s", postID, created.ID)
	}
	if created.Title != "Title 1" {
		t.Errorf("expected Title 'Title 1', got %s", created.Title)
	}

	fetched, err := testRepo.GetPostWithImages(ctx, postID)
	if err != nil {
		t.Fatalf("GetPostWithImages: %v", err)
	}

	if fetched.ID != postID {
		t.Errorf("expected fetched ID %s, got %s", postID, fetched.ID)
	}
	if fetched.AuthorID != authorID {
		t.Errorf("expected author ID %s, got %s", authorID, fetched.AuthorID)
	}
}

func TestRepository_PostWithImages(t *testing.T) {
	ctx := setup(t)
	authorID := uuid.New()
	mustCreateUser(t, ctx, authorID, "art_lover")

	postID, _ := uuid.NewV7()
	_, err := testRepo.CreatePost(ctx, postID, authorID, "Art Post", "With Images")
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	uID1 := uuid.New()
	uID2 := uuid.New()
	mustCreateUpload(t, ctx, uID1, authorID, "final/art1.jpg")
	mustCreateUpload(t, ctx, uID2, authorID, "final/art2.jpg")

	if err := testRepo.BatchInsertPostImages(ctx, postID, []uuid.UUID{uID1, uID2}, []int16{0, 1}); err != nil {
		t.Fatalf("BatchInsertPostImages: %v", err)
	}

	postWithImages, err := testRepo.GetPostWithImages(ctx, postID)
	if err != nil {
		t.Fatalf("GetPostWithImages: %v", err)
	}
	if len(postWithImages.Images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(postWithImages.Images))
	}
	if postWithImages.Images[0].Position != 0 || postWithImages.Images[0].ObjectKey != "final/art1.jpg" {
		t.Errorf("unexpected image 0: %+v", postWithImages.Images[0])
	}
	if postWithImages.Images[1].Position != 1 || postWithImages.Images[1].ObjectKey != "final/art2.jpg" {
		t.Errorf("unexpected image 1: %+v", postWithImages.Images[1])
	}

	batchMap, err := testRepo.GetPostImagesByPostIDs(ctx, []uuid.UUID{postID})
	if err != nil {
		t.Fatalf("GetPostImagesByPostIDs: %v", err)
	}
	if len(batchMap[postID]) != 2 {
		t.Fatalf("expected 2 images in batch map, got %d", len(batchMap[postID]))
	}
}

func TestRepository_UpdatePost(t *testing.T) {
	ctx := setup(t)
	authorID := uuid.New()
	mustCreateUser(t, ctx, authorID, "editor")

	postID, _ := uuid.NewV7()
	_, err := testRepo.CreatePost(ctx, postID, authorID, "Original Title", "Original Content")
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	newTitle := "Updated Title"
	newContent := "Updated Content"
	updated, err := testRepo.UpdatePost(ctx, postID, &newTitle, &newContent)
	if err != nil {
		t.Fatalf("UpdatePost: %v", err)
	}
	if updated.Title != newTitle || updated.Content != newContent {
		t.Errorf("unexpected updated post: %+v", updated)
	}

	partialTitle := "Partial Title Only"
	updatedPartial, err := testRepo.UpdatePost(ctx, postID, &partialTitle, nil)
	if err != nil {
		t.Fatalf("UpdatePost partial: %v", err)
	}
	if updatedPartial.Title != partialTitle || updatedPartial.Content != newContent {
		t.Errorf("expected content to be preserved: %+v", updatedPartial)
	}
}

func TestRepository_DeletePost_Cascade(t *testing.T) {
	ctx := setup(t)
	authorID := uuid.New()
	mustCreateUser(t, ctx, authorID, "deleter")

	postID, _ := uuid.NewV7()
	_, err := testRepo.CreatePost(ctx, postID, authorID, "To Delete", "Content")
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	uID := uuid.New()
	mustCreateUpload(t, ctx, uID, authorID, "final/del.jpg")
	if err := testRepo.BatchInsertPostImages(ctx, postID, []uuid.UUID{uID}, []int16{0}); err != nil {
		t.Fatalf("BatchInsertPostImages: %v", err)
	}

	deletedIDs, err := testRepo.DeletePostImagesByPostID(ctx, postID)
	if err != nil {
		t.Fatalf("DeletePostImagesByPostID: %v", err)
	}
	if len(deletedIDs) != 1 || deletedIDs[0] != uID {
		t.Errorf("expected deletedID %s, got %v", uID, deletedIDs)
	}

	if err := testRepo.DeletePost(ctx, postID); err != nil {
		t.Fatalf("DeletePost: %v", err)
	}

	_, err = testRepo.GetPostWithImages(ctx, postID)
	if !errors.Is(err, ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound after delete, got %v", err)
	}
}

func TestRepository_ListRecentAndAuthorPosts(t *testing.T) {
	ctx := setup(t)
	author1 := uuid.New()
	author2 := uuid.New()
	mustCreateUser(t, ctx, author1, "author_a")
	mustCreateUser(t, ctx, author2, "author_b")

	p1, _ := uuid.NewV7()
	p2, _ := uuid.NewV7()
	p3, _ := uuid.NewV7()

	_, _ = testRepo.CreatePost(ctx, p1, author1, "Post A1", "Content")
	_, _ = testRepo.CreatePost(ctx, p2, author2, "Post B1", "Content")
	_, _ = testRepo.CreatePost(ctx, p3, author1, "Post A2", "Content")

	recent, err := testRepo.ListRecentPosts(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListRecentPosts: %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent posts, got %d", len(recent))
	}

	author1Posts, err := testRepo.ListPostsByAuthor(ctx, author1, 10, 0)
	if err != nil {
		t.Fatalf("ListPostsByAuthor: %v", err)
	}
	if len(author1Posts) != 2 {
		t.Fatalf("expected 2 posts for author1, got %d", len(author1Posts))
	}
}

func TestRepository_Constraint_MaxPosition(t *testing.T) {
	ctx := setup(t)
	authorID := uuid.New()
	mustCreateUser(t, ctx, authorID, "chk_user")

	postID, _ := uuid.NewV7()
	_, _ = testRepo.CreatePost(ctx, postID, authorID, "Check Post", "Content")

	uID := uuid.New()
	mustCreateUpload(t, ctx, uID, authorID, "final/pos10.jpg")

	err := testRepo.BatchInsertPostImages(ctx, postID, []uuid.UUID{uID}, []int16{10})
	if err == nil {
		t.Fatalf("expected check constraint error for position 10, got nil")
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.ConstraintName != "chk_post_images_position" {
			t.Errorf("expected constraint chk_post_images_position, got %s", pgErr.ConstraintName)
		}
	}
}

func TestRepository_Constraint_UniqueUploadID(t *testing.T) {
	ctx := setup(t)
	authorID := uuid.New()
	mustCreateUser(t, ctx, authorID, "uq_user")

	p1, _ := uuid.NewV7()
	p2, _ := uuid.NewV7()
	_, _ = testRepo.CreatePost(ctx, p1, authorID, "Post 1", "Content")
	_, _ = testRepo.CreatePost(ctx, p2, authorID, "Post 2", "Content")

	uID := uuid.New()
	mustCreateUpload(t, ctx, uID, authorID, "final/shared.jpg")

	if err := testRepo.BatchInsertPostImages(ctx, p1, []uuid.UUID{uID}, []int16{0}); err != nil {
		t.Fatalf("first insert failed: %v", err)
	}

	err := testRepo.BatchInsertPostImages(ctx, p2, []uuid.UUID{uID}, []int16{0})
	if err == nil {
		t.Fatalf("expected unique upload constraint violation, got nil")
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.ConstraintName != "uq_post_images_upload_id" {
			t.Errorf("expected constraint uq_post_images_upload_id, got %s", pgErr.ConstraintName)
		}
	}
}

func TestRepository_DeferredPositionUniqueness(t *testing.T) {
	ctx := setup(t)
	authorID := uuid.New()
	mustCreateUser(t, ctx, authorID, "defer_user")

	postID, _ := uuid.NewV7()
	_, _ = testRepo.CreatePost(ctx, postID, authorID, "Deferred Post", "Content")

	uID1 := uuid.New()
	uID2 := uuid.New()
	mustCreateUpload(t, ctx, uID1, authorID, "final/d1.jpg")
	mustCreateUpload(t, ctx, uID2, authorID, "final/d2.jpg")

	if err := testRepo.BatchInsertPostImages(ctx, postID, []uuid.UUID{uID1, uID2}, []int16{0, 1}); err != nil {
		t.Fatalf("initial batch insert failed: %v", err)
	}

	err := testRepo.WithTransaction(ctx, func(txCtx context.Context) error {
		return testRepo.UpdatePostImagePositions(txCtx, postID, []uuid.UUID{uID1, uID2}, []int16{1, 0})
	})
	if err != nil {
		t.Fatalf("deferred position swap failed: %v", err)
	}

	postWithImages, err := testRepo.GetPostWithImages(ctx, postID)
	if err != nil {
		t.Fatalf("GetPostWithImages: %v", err)
	}
	if postWithImages.Images[0].UploadID != uID2 || postWithImages.Images[0].Position != 0 {
		t.Errorf("expected uID2 at position 0, got %+v", postWithImages.Images[0])
	}
	if postWithImages.Images[1].UploadID != uID1 || postWithImages.Images[1].Position != 1 {
		t.Errorf("expected uID1 at position 1, got %+v", postWithImages.Images[1])
	}
}
