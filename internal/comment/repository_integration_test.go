package comment

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/emanuelfelicio/artblogapi/db/dbgen"
	"github.com/emanuelfelicio/artblogapi/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	integrationPool *pgxpool.Pool
	integrationRepo *repository
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	var container *postgres.PostgresContainer
	integrationPool, container = testutil.NewTestDB(ctx)
	defer integrationPool.Close()
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			log.Printf("failed to terminate postgres container: %v", err)
		}
	}()

	integrationRepo = NewRepository(dbgen.New(integrationPool), integrationPool)
	os.Exit(m.Run())
}

func integrationSetup(t *testing.T) context.Context {
	t.Helper()
	t.Cleanup(func() {
		if _, err := integrationPool.Exec(context.Background(), `TRUNCATE TABLE comments, posts, users RESTART IDENTITY CASCADE`); err != nil {
			t.Errorf("failed to clean integration tables: %v", err)
		}
	})
	return context.Background()
}

func mustCreateIntegrationUser(t *testing.T, ctx context.Context, id uuid.UUID, suffix string) {
	t.Helper()
	_, err := integrationPool.Exec(ctx, `
		INSERT INTO users (id, username, email, password_hash, display_name, bio, is_active)
		VALUES ($1, $2, $3, 'hash', 'Display', 'Bio', true)
	`, id, "comment_"+suffix, "comment_"+suffix+"@example.com")
	if err != nil {
		t.Fatalf("create integration user: %v", err)
	}
}

func mustCreateIntegrationPost(t *testing.T, ctx context.Context, id, authorID uuid.UUID) {
	t.Helper()
	_, err := integrationPool.Exec(ctx, `
		INSERT INTO posts (id, author_id, title, content)
		VALUES ($1, $2, 'Post title', 'Post content')
	`, id, authorID)
	if err != nil {
		t.Fatalf("create integration post: %v", err)
	}
}

func TestRepositoryIntegration_CreateAndGetComment(t *testing.T) {
	ctx := integrationSetup(t)
	authorID := uuid.New()
	postID := uuid.New()
	commentID := uuid.New()
	mustCreateIntegrationUser(t, ctx, authorID, "create")
	mustCreateIntegrationPost(t, ctx, postID, authorID)

	created, err := integrationRepo.Create(ctx, commentID, postID, authorID, "A useful comment")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != commentID || created.PostID != postID || created.AuthorID != authorID {
		t.Fatalf("unexpected created comment: %+v", created)
	}
	if created.Content != "A useful comment" || created.DeletedAt != nil {
		t.Fatalf("unexpected created content/deletion state: %+v", created)
	}

	found, err := integrationRepo.GetForUpdate(ctx, commentID, postID)
	if err != nil {
		t.Fatalf("GetForUpdate: %v", err)
	}
	if found.ID != commentID || found.Content != created.Content {
		t.Fatalf("unexpected fetched comment: %+v", found)
	}
	if _, err := integrationRepo.GetForUpdate(ctx, commentID, uuid.New()); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("GetForUpdate with wrong post error = %v, want ErrCommentNotFound", err)
	}
}

func TestRepositoryIntegration_PostExistsAndForeignKeys(t *testing.T) {
	ctx := integrationSetup(t)
	authorID := uuid.New()
	postID := uuid.New()
	mustCreateIntegrationUser(t, ctx, authorID, "fk")

	if err := integrationRepo.PostExists(ctx, postID); !errors.Is(err, ErrPostNotFound) {
		t.Fatalf("PostExists() error = %v, want ErrPostNotFound", err)
	}

	err := func() error {
		_, err := integrationRepo.Create(ctx, uuid.New(), postID, authorID, "orphan")
		return err
	}()
	assertForeignKeyViolation(t, err, "missing post")

	mustCreateIntegrationPost(t, ctx, postID, authorID)
	err = func() error {
		_, err := integrationRepo.Create(ctx, uuid.New(), postID, uuid.New(), "unknown author")
		return err
	}()
	assertForeignKeyViolation(t, err, "missing author")
}

func assertForeignKeyViolation(t *testing.T, err error, caseName string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Create accepted %s", caseName)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("Create %s error = %v, want foreign key violation", caseName, err)
	}
}

func TestRepositoryIntegration_ListFiltersDeletedAndPaginates(t *testing.T) {
	ctx := integrationSetup(t)
	authorID := uuid.New()
	postID := uuid.New()
	mustCreateIntegrationUser(t, ctx, authorID, "list")
	mustCreateIntegrationPost(t, ctx, postID, authorID)

	olderID := uuid.New()
	newerID := uuid.New()
	deletedID := uuid.New()
	for id, content := range map[uuid.UUID]string{
		olderID:   "older",
		newerID:   "newer",
		deletedID: "deleted",
	} {
		if _, err := integrationRepo.Create(ctx, id, postID, authorID, content); err != nil {
			t.Fatalf("Create(%s): %v", content, err)
		}
	}
	_, err := integrationPool.Exec(ctx, `
		UPDATE comments
		SET created_at = CASE id
			WHEN $1 THEN '2026-01-01T00:00:00Z'::timestamptz
			WHEN $2 THEN '2026-01-02T00:00:00Z'::timestamptz
			ELSE created_at
		END,
		deleted_at = CASE WHEN id = $3 THEN now() ELSE deleted_at END
		WHERE id = ANY($4::uuid[])
	`, olderID, newerID, deletedID, []uuid.UUID{olderID, newerID, deletedID})
	if err != nil {
		t.Fatalf("prepare list rows: %v", err)
	}

	comments, err := integrationRepo.ListByPost(ctx, postID, 10, 0)
	if err != nil {
		t.Fatalf("ListByPost: %v", err)
	}
	if len(comments) != 2 || comments[0].ID != newerID || comments[1].ID != olderID {
		t.Fatalf("unexpected ordered comments: %+v", comments)
	}

	page, err := integrationRepo.ListByPost(ctx, postID, 1, 1)
	if err != nil {
		t.Fatalf("ListByPost page: %v", err)
	}
	if len(page) != 1 || page[0].ID != olderID {
		t.Fatalf("unexpected paginated comments: %+v", page)
	}
}

func TestRepositoryIntegration_UpdateAndSoftDelete(t *testing.T) {
	ctx := integrationSetup(t)
	authorID := uuid.New()
	postID := uuid.New()
	commentID := uuid.New()
	mustCreateIntegrationUser(t, ctx, authorID, "mutate")
	mustCreateIntegrationPost(t, ctx, postID, authorID)
	if _, err := integrationRepo.Create(ctx, commentID, postID, authorID, "original"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := integrationPool.Exec(ctx, `UPDATE comments SET created_at = '2026-01-01T00:00:00Z' WHERE id = $1`, commentID); err != nil {
		t.Fatalf("set comment creation time: %v", err)
	}

	updated, err := integrationRepo.Update(ctx, commentID, postID, "updated")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Content != "updated" || !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Fatalf("unexpected updated comment: %+v", updated)
	}

	if err := integrationRepo.SoftDelete(ctx, commentID, postID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	deleted, err := integrationRepo.GetForUpdate(ctx, commentID, postID)
	if err != nil {
		t.Fatalf("GetForUpdate deleted: %v", err)
	}
	if deleted.DeletedAt == nil {
		t.Fatal("soft-deleted comment has nil DeletedAt")
	}
	if comments, err := integrationRepo.ListByPost(ctx, postID, 10, 0); err != nil {
		t.Fatalf("ListByPost after delete: %v", err)
	} else if len(comments) != 0 {
		t.Fatalf("expected deleted comment to be hidden, got %+v", comments)
	}
	if _, err := integrationRepo.Update(ctx, commentID, postID, "must fail"); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("Update deleted error = %v, want ErrCommentNotFound", err)
	}
	if err := integrationRepo.SoftDelete(ctx, commentID, postID); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("SoftDelete deleted error = %v, want ErrCommentNotFound", err)
	}
}

func TestRepositoryIntegration_PostDeleteCascadesComments(t *testing.T) {
	ctx := integrationSetup(t)
	authorID := uuid.New()
	postID := uuid.New()
	commentID := uuid.New()
	mustCreateIntegrationUser(t, ctx, authorID, "cascade")
	mustCreateIntegrationPost(t, ctx, postID, authorID)
	if _, err := integrationRepo.Create(ctx, commentID, postID, authorID, "will cascade"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := integrationPool.Exec(ctx, `DELETE FROM posts WHERE id = $1`, postID); err != nil {
		t.Fatalf("delete post: %v", err)
	}
	if _, err := integrationRepo.GetForUpdate(ctx, commentID, postID); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("comment after post deletion error = %v, want ErrCommentNotFound", err)
	}
}

func TestRepositoryIntegration_TransactionRollsBack(t *testing.T) {
	ctx := integrationSetup(t)
	authorID := uuid.New()
	postID := uuid.New()
	commentID := uuid.New()
	mustCreateIntegrationUser(t, ctx, authorID, "tx")
	mustCreateIntegrationPost(t, ctx, postID, authorID)

	expected := errors.New("force rollback")
	err := integrationRepo.WithTransaction(ctx, func(txCtx context.Context) error {
		if _, err := integrationRepo.Create(txCtx, commentID, postID, authorID, "rolled back"); err != nil {
			return err
		}
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("WithTransaction() error = %v, want rollback error", err)
	}
	if _, err := integrationRepo.GetForUpdate(ctx, commentID, postID); !errors.Is(err, ErrCommentNotFound) {
		t.Fatalf("comment after rollback error = %v, want ErrCommentNotFound", err)
	}
}

func TestRepositoryIntegration_ContentConstraint(t *testing.T) {
	ctx := integrationSetup(t)
	authorID := uuid.New()
	postID := uuid.New()
	mustCreateIntegrationUser(t, ctx, authorID, "constraint")
	mustCreateIntegrationPost(t, ctx, postID, authorID)

	for name, content := range map[string]string{
		"blank":    "   ",
		"too long": strings.Repeat("a", MaxContentLength+1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := integrationRepo.Create(ctx, uuid.New(), postID, authorID, content)
			if err == nil {
				t.Fatal("Create accepted invalid content")
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
				t.Fatalf("Create error = %v, want check constraint violation", err)
			}
		})
	}
}
