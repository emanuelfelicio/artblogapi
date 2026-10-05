package comment

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/emanuelfelicio/artblogapi/internal/testutil/testauth"
	"github.com/emanuelfelicio/artblogapi/internal/testutil/testhttp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type handlerServiceStub struct {
	create     func(context.Context, uuid.UUID, uuid.UUID, string) (Comment, error)
	listByPost func(context.Context, uuid.UUID, int32, int32) ([]Comment, error)
	update     func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (Comment, error)
	delete     func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
}

func (s *handlerServiceStub) Create(ctx context.Context, postID, authorID uuid.UUID, content string) (Comment, error) {
	return s.create(ctx, postID, authorID, content)
}

func (s *handlerServiceStub) ListByPost(ctx context.Context, postID uuid.UUID, limit, offset int32) ([]Comment, error) {
	return s.listByPost(ctx, postID, limit, offset)
}

func (s *handlerServiceStub) Update(ctx context.Context, postID, commentID, authorID uuid.UUID, content string) (Comment, error) {
	return s.update(ctx, postID, commentID, authorID, content)
}

func (s *handlerServiceStub) Delete(ctx context.Context, postID, commentID, authorID uuid.UUID) error {
	return s.delete(ctx, postID, commentID, authorID)
}

func newCommentRouter(service Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	Routes(router.Group("/api/v1"), handler, testauth.WithPrincipal(uuid.New().String()))
	return router
}

func TestHandlerList_ReturnsCommentsAndPagination(t *testing.T) {
	postID := uuid.New()
	var gotLimit, gotOffset int32
	var returned Comment
	router := newCommentRouter(&handlerServiceStub{
		listByPost: func(ctx context.Context, receivedPostID uuid.UUID, limit, offset int32) ([]Comment, error) {
			gotLimit, gotOffset = limit, offset
			returned = Comment{ID: uuid.New(), PostID: receivedPostID, AuthorID: uuid.New(), Content: "hello", CreatedAt: time.Now(), UpdatedAt: time.Now()}
			return []Comment{returned}, nil
		},
	})

	w := testhttp.DoRequest(t, router, http.MethodGet, "/api/v1/posts/"+postID.String()+"/comments?limit=10&offset=5", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET comments status = %d, want %d", w.Code, http.StatusOK)
	}
	if gotLimit != 10 || gotOffset != 5 {
		t.Fatalf("pagination = %d, %d, want 10, 5", gotLimit, gotOffset)
	}
	resp := testhttp.DecodeResponse[[]CommentResponse](t, w)
	if len(resp.Data) != 1 || resp.Data[0].ID != returned.ID.String() || resp.Data[0].PostID != postID.String() || resp.Data[0].Content != returned.Content {
		t.Fatalf("unexpected response data: %+v", resp.Data)
	}
}

func TestHandlerCreate_RequiresValidContentAndPassesPrincipal(t *testing.T) {
	postID := uuid.New()
	var gotAuthorID uuid.UUID
	var gotPostID uuid.UUID
	var gotContent string
	router := newCommentRouter(&handlerServiceStub{
		create: func(ctx context.Context, receivedPostID, authorID uuid.UUID, content string) (Comment, error) {
			gotPostID, gotAuthorID, gotContent = receivedPostID, authorID, content
			return Comment{ID: uuid.New(), PostID: receivedPostID, AuthorID: authorID, Content: content}, nil
		},
	})

	w := testhttp.DoRequest(t, router, http.MethodPost, "/api/v1/posts/"+postID.String()+"/comments", CommentRequest{Content: "hello"})
	if w.Code != http.StatusCreated {
		t.Fatalf("POST comments status = %d, want %d", w.Code, http.StatusCreated)
	}
	if gotPostID != postID || gotAuthorID == uuid.Nil || gotContent != "hello" {
		t.Fatalf("service received post=%v author=%v content=%q", gotPostID, gotAuthorID, gotContent)
	}
	resp := testhttp.DecodeResponse[CommentResponse](t, w)
	if resp.Data.PostID != postID.String() || resp.Data.AuthorID != gotAuthorID.String() || resp.Data.Content != gotContent {
		t.Fatalf("unexpected create response: %+v", resp.Data)
	}
}

func TestHandlerCreate_RejectsInvalidContent(t *testing.T) {
	router := newCommentRouter(&handlerServiceStub{
		create: func(context.Context, uuid.UUID, uuid.UUID, string) (Comment, error) {
			t.Fatal("service must not be called for invalid content")
			return Comment{}, nil
		},
	})

	w := testhttp.DoRequest(t, router, http.MethodPost, "/api/v1/posts/"+uuid.New().String()+"/comments", CommentRequest{Content: "   "})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST invalid comment status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
