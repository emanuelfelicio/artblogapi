package comment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type serviceRepositoryStub struct {
	withTransaction func(context.Context, func(context.Context) error) error
	postExists      func(context.Context, uuid.UUID) error
	create          func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (Comment, error)
	listByPost      func(context.Context, uuid.UUID, int32, int32) ([]Comment, error)
	getForUpdate    func(context.Context, uuid.UUID, uuid.UUID) (Comment, error)
	update          func(context.Context, uuid.UUID, uuid.UUID, string) (Comment, error)
	softDelete      func(context.Context, uuid.UUID, uuid.UUID) error
}

func (s *serviceRepositoryStub) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	if s.withTransaction != nil {
		return s.withTransaction(ctx, fn)
	}
	return fn(ctx)
}

func (s *serviceRepositoryStub) PostExists(ctx context.Context, id uuid.UUID) error {
	if s.postExists != nil {
		return s.postExists(ctx, id)
	}
	return nil
}

func (s *serviceRepositoryStub) Create(ctx context.Context, id, postID, authorID uuid.UUID, content string) (Comment, error) {
	if s.create != nil {
		return s.create(ctx, id, postID, authorID, content)
	}
	return Comment{ID: id, PostID: postID, AuthorID: authorID, Content: content}, nil
}

func (s *serviceRepositoryStub) ListByPost(ctx context.Context, postID uuid.UUID, limit, offset int32) ([]Comment, error) {
	if s.listByPost != nil {
		return s.listByPost(ctx, postID, limit, offset)
	}
	return nil, nil
}

func (s *serviceRepositoryStub) GetForUpdate(ctx context.Context, id, postID uuid.UUID) (Comment, error) {
	if s.getForUpdate != nil {
		return s.getForUpdate(ctx, id, postID)
	}
	return Comment{}, ErrCommentNotFound
}

func (s *serviceRepositoryStub) Update(ctx context.Context, id, postID uuid.UUID, content string) (Comment, error) {
	if s.update != nil {
		return s.update(ctx, id, postID, content)
	}
	return Comment{ID: id, PostID: postID, Content: content}, nil
}

func (s *serviceRepositoryStub) SoftDelete(ctx context.Context, id, postID uuid.UUID) error {
	if s.softDelete != nil {
		return s.softDelete(ctx, id, postID)
	}
	return nil
}

func TestServiceCreate_TrimsContentAndChecksPost(t *testing.T) {
	postID := uuid.New()
	authorID := uuid.New()
	var gotPostID, gotAuthorID uuid.UUID
	var gotContent string
	repo := &serviceRepositoryStub{
		postExists: func(ctx context.Context, id uuid.UUID) error {
			gotPostID = id
			return nil
		},
		create: func(ctx context.Context, id, receivedPostID, receivedAuthorID uuid.UUID, content string) (Comment, error) {
			gotPostID = receivedPostID
			gotAuthorID = receivedAuthorID
			gotContent = content
			return Comment{ID: id, PostID: receivedPostID, AuthorID: receivedAuthorID, Content: content}, nil
		},
	}

	got, err := NewService(repo).Create(context.Background(), postID, authorID, "  comentário  ")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if gotPostID != postID || gotAuthorID != authorID {
		t.Fatalf("Create() passed wrong identities: post=%v author=%v", gotPostID, gotAuthorID)
	}
	if gotContent != "comentário" || got.Content != "comentário" {
		t.Fatalf("Create() content = %q, want trimmed content", gotContent)
	}
}

func TestServiceCreate_ReturnsPostNotFound(t *testing.T) {
	repo := &serviceRepositoryStub{
		postExists: func(context.Context, uuid.UUID) error { return ErrPostNotFound },
		create: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (Comment, error) {
			t.Fatal("create must not be called")
			return Comment{}, nil
		},
	}

	_, err := NewService(repo).Create(context.Background(), uuid.New(), uuid.New(), "content")
	if !errors.Is(err, ErrPostNotFound) {
		t.Fatalf("Create() error = %v, want ErrPostNotFound", err)
	}
}

func TestServiceList_NormalizesPagination(t *testing.T) {
	postID := uuid.New()
	var checkedPostID uuid.UUID
	var gotLimit, gotOffset int32
	repo := &serviceRepositoryStub{
		postExists: func(ctx context.Context, receivedPostID uuid.UUID) error {
			checkedPostID = receivedPostID
			return nil
		},
		listByPost: func(ctx context.Context, postID uuid.UUID, limit, offset int32) ([]Comment, error) {
			gotLimit, gotOffset = limit, offset
			return []Comment{}, nil
		},
	}

	_, err := NewService(repo).ListByPost(context.Background(), postID, 999, -2)
	if err != nil {
		t.Fatalf("ListByPost() error = %v", err)
	}
	if gotLimit != DefaultPageLimit || gotOffset != 0 {
		t.Fatalf("ListByPost() pagination = %d, %d", gotLimit, gotOffset)
	}
	if checkedPostID != postID {
		t.Fatalf("ListByPost() checked post = %v, want %v", checkedPostID, postID)
	}
}

func TestServiceUpdate_RejectsDifferentAuthor(t *testing.T) {
	repo := &serviceRepositoryStub{
		getForUpdate: func(context.Context, uuid.UUID, uuid.UUID) (Comment, error) {
			return Comment{AuthorID: uuid.New()}, nil
		},
		update: func(context.Context, uuid.UUID, uuid.UUID, string) (Comment, error) {
			t.Fatal("update must not be called for a different author")
			return Comment{}, nil
		},
	}

	err := func() error {
		_, err := NewService(repo).Update(context.Background(), uuid.New(), uuid.New(), uuid.New(), "new content")
		return err
	}()
	if !errors.Is(err, ErrCommentForbidden) {
		t.Fatalf("Update() error = %v, want ErrCommentForbidden", err)
	}
}

func TestServiceDelete_DeletesOnlyOwnedComment(t *testing.T) {
	authorID := uuid.New()
	postID := uuid.New()
	commentID := uuid.New()
	var deletedPostID, deletedCommentID uuid.UUID
	var deleted bool
	repo := &serviceRepositoryStub{
		getForUpdate: func(context.Context, uuid.UUID, uuid.UUID) (Comment, error) {
			return Comment{AuthorID: authorID}, nil
		},
		softDelete: func(_ context.Context, receivedCommentID, receivedPostID uuid.UUID) error {
			deletedCommentID, deletedPostID = receivedCommentID, receivedPostID
			deleted = true
			return nil
		},
	}

	if err := NewService(repo).Delete(context.Background(), postID, commentID, authorID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Fatal("Delete() did not soft-delete the comment")
	}
	if deletedPostID != postID || deletedCommentID != commentID {
		t.Fatalf("Delete() IDs = comment %v, post %v; want comment %v, post %v", deletedCommentID, deletedPostID, commentID, postID)
	}
}

func TestServiceUpdate_RejectsDeletedComment(t *testing.T) {
	deletedAt := time.Now()
	repo := &serviceRepositoryStub{
		getForUpdate: func(context.Context, uuid.UUID, uuid.UUID) (Comment, error) {
			return Comment{AuthorID: uuid.Nil, DeletedAt: &deletedAt}, nil
		},
		update: func(context.Context, uuid.UUID, uuid.UUID, string) (Comment, error) {
			t.Fatal("update must not be called for a deleted comment")
			return Comment{}, nil
		},
	}

	_, err := NewService(repo).Update(context.Background(), uuid.New(), uuid.New(), uuid.Nil, "content")
	if !errors.Is(err, ErrCommentDeleted) {
		t.Fatalf("Update() error = %v, want ErrCommentDeleted", err)
	}
}

func TestServiceUpdate_PropagatesTransactionContext(t *testing.T) {
	wantContext := context.WithValue(context.Background(), struct{}{}, "transaction")
	var gotContext context.Context
	repo := &serviceRepositoryStub{
		withTransaction: func(_ context.Context, fn func(context.Context) error) error {
			return fn(wantContext)
		},
		getForUpdate: func(ctx context.Context, _, _ uuid.UUID) (Comment, error) {
			gotContext = ctx
			return Comment{AuthorID: uuid.Nil}, nil
		},
	}

	_, err := NewService(repo).Update(context.Background(), uuid.New(), uuid.New(), uuid.Nil, "content")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if gotContext != wantContext {
		t.Fatal("Update() did not propagate transaction context")
	}
}
