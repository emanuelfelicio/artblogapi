package post

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/emanuelfelicio/artblogapi/internal/testutil/testauth"
	"github.com/emanuelfelicio/artblogapi/internal/testutil/testhttp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type stubPostService struct {
	createPost        func(ctx context.Context, authorID uuid.UUID, title, content string, imageUploadIDs []string) (Post, error)
	getPost           func(ctx context.Context, id uuid.UUID) (Post, error)
	listRecentPosts   func(ctx context.Context, limit, offset int32) ([]Post, error)
	listPostsByAuthor func(ctx context.Context, authorID uuid.UUID, limit, offset int32) ([]Post, error)
	updatePost        func(ctx context.Context, postID, authorID uuid.UUID, title, content *string, imageUploadIDs []string) (Post, error)
	deletePost        func(ctx context.Context, postID, authorID uuid.UUID) error
}

func (s *stubPostService) CreatePost(ctx context.Context, authorID uuid.UUID, title, content string, imageUploadIDs []string) (Post, error) {
	if s.createPost != nil {
		return s.createPost(ctx, authorID, title, content, imageUploadIDs)
	}
	return Post{}, nil
}

func (s *stubPostService) GetPost(ctx context.Context, id uuid.UUID) (Post, error) {
	if s.getPost != nil {
		return s.getPost(ctx, id)
	}
	return Post{}, nil
}

func (s *stubPostService) ListRecentPosts(ctx context.Context, limit, offset int32) ([]Post, error) {
	if s.listRecentPosts != nil {
		return s.listRecentPosts(ctx, limit, offset)
	}
	return nil, nil
}

func (s *stubPostService) ListPostsByAuthor(ctx context.Context, authorID uuid.UUID, limit, offset int32) ([]Post, error) {
	if s.listPostsByAuthor != nil {
		return s.listPostsByAuthor(ctx, authorID, limit, offset)
	}
	return nil, nil
}

func (s *stubPostService) UpdatePost(ctx context.Context, postID, authorID uuid.UUID, title, content *string, imageUploadIDs []string) (Post, error) {
	if s.updatePost != nil {
		return s.updatePost(ctx, postID, authorID, title, content, imageUploadIDs)
	}
	return Post{}, nil
}

func (s *stubPostService) DeletePost(ctx context.Context, postID, authorID uuid.UUID) error {
	if s.deletePost != nil {
		return s.deletePost(ctx, postID, authorID)
	}
	return nil
}

func setupTestRouter(svc PostService, authMiddleware gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(svc, logger, "http://cdn.example.com", "http://cdn.example.com/default/avatar.png")

	r := gin.New()
	v1 := r.Group("/api/v1")
	Routes(v1, h, authMiddleware)
	return r
}

func samplePost() Post {
	postID := uuid.New()
	authorID := uuid.New()
	key := "final/image.jpg"
	return Post{
		ID:        postID,
		AuthorID:  authorID,
		Title:     "Sample Title",
		Content:   "Sample Content",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Author: PostAuthor{
			ID:          authorID,
			Username:    "artist",
			DisplayName: "Artist Name",
		},
		Images: []PostImage{
			{
				PostID:      postID,
				UploadID:    uuid.New(),
				Position:    0,
				ObjectKey:   key,
				ContentType: "image/jpeg",
				CreatedAt:   time.Now(),
			},
		},
	}
}

func TestHandler_CreatePost_201(t *testing.T) {
	expectedPost := samplePost()
	svc := &stubPostService{
		createPost: func(ctx context.Context, authorID uuid.UUID, title, content string, imageUploadIDs []string) (Post, error) {
			return expectedPost, nil
		},
	}

	r := setupTestRouter(svc, testauth.WithPrincipal(expectedPost.AuthorID.String()))
	body := CreatePostRequest{Title: "Sample Title", Content: "Sample Content"}
	w := testhttp.DoRequest(t, r, http.MethodPost, "/api/v1/posts", body)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := testhttp.DecodeResponse[PostResponse](t, w)
	if resp.Data.Title != expectedPost.Title {
		t.Errorf("expected title %q, got %q", expectedPost.Title, resp.Data.Title)
	}
	if len(resp.Data.Images) != 1 {
		t.Errorf("expected 1 image, got %d", len(resp.Data.Images))
	}
	if !strings.HasPrefix(resp.Data.Images[0].URL, "http://cdn.example.com/") {
		t.Errorf("expected cdn prefix on image URL, got %q", resp.Data.Images[0].URL)
	}
}

func TestHandler_CreatePost_400_Validation(t *testing.T) {
	svc := &stubPostService{}
	r := setupTestRouter(svc, testauth.WithPrincipal(uuid.NewString()))
	body := CreatePostRequest{Title: "", Content: ""}
	w := testhttp.DoRequest(t, r, http.MethodPost, "/api/v1/posts", body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_CreatePost_401_Unauthorized(t *testing.T) {
	svc := &stubPostService{}
	r := setupTestRouter(svc, testauth.WithoutPrincipal())
	body := CreatePostRequest{Title: "Title", Content: "Content"}
	w := testhttp.DoRequest(t, r, http.MethodPost, "/api/v1/posts", body)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandler_GetPost_200(t *testing.T) {
	expectedPost := samplePost()
	svc := &stubPostService{
		getPost: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return expectedPost, nil
		},
	}

	r := setupTestRouter(svc, testauth.WithoutPrincipal())
	w := testhttp.DoRequest(t, r, http.MethodGet, "/api/v1/posts/"+expectedPost.ID.String(), nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := testhttp.DecodeResponse[PostResponse](t, w)
	if resp.Data.ID != expectedPost.ID.String() {
		t.Errorf("expected ID %q, got %q", expectedPost.ID.String(), resp.Data.ID)
	}
}

func TestHandler_GetPost_404(t *testing.T) {
	svc := &stubPostService{
		getPost: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return Post{}, ErrPostNotFound
		},
	}

	r := setupTestRouter(svc, testauth.WithoutPrincipal())
	w := testhttp.DoRequest(t, r, http.MethodGet, "/api/v1/posts/"+uuid.NewString(), nil)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetPost_400_InvalidUUID(t *testing.T) {
	svc := &stubPostService{}
	r := setupTestRouter(svc, testauth.WithoutPrincipal())
	w := testhttp.DoRequest(t, r, http.MethodGet, "/api/v1/posts/not-a-uuid", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandler_ListRecentPosts_200(t *testing.T) {
	p1 := samplePost()
	p2 := samplePost()
	svc := &stubPostService{
		listRecentPosts: func(ctx context.Context, limit, offset int32) ([]Post, error) {
			return []Post{p1, p2}, nil
		},
	}

	r := setupTestRouter(svc, testauth.WithoutPrincipal())
	w := testhttp.DoRequest(t, r, http.MethodGet, "/api/v1/posts/recent?limit=10&offset=0", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := testhttp.DecodeResponse[[]PostResponse](t, w)
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(resp.Data))
	}
}

func TestHandler_ListPostsByAuthor_200(t *testing.T) {
	authorID := uuid.New()
	p1 := samplePost()
	p1.AuthorID = authorID
	svc := &stubPostService{
		listPostsByAuthor: func(ctx context.Context, aID uuid.UUID, limit, offset int32) ([]Post, error) {
			if aID != authorID {
				t.Errorf("expected authorID %v, got %v", authorID, aID)
			}
			return []Post{p1}, nil
		},
	}

	r := setupTestRouter(svc, testauth.WithoutPrincipal())
	w := testhttp.DoRequest(t, r, http.MethodGet, "/api/v1/posts/author/"+authorID.String()+"?limit=10&offset=0", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := testhttp.DecodeResponse[[]PostResponse](t, w)
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 post, got %d", len(resp.Data))
	}
}

func TestHandler_ListPostsByAuthor_400_InvalidUUID(t *testing.T) {
	svc := &stubPostService{}
	r := setupTestRouter(svc, testauth.WithoutPrincipal())
	w := testhttp.DoRequest(t, r, http.MethodGet, "/api/v1/posts/author/invalid-uuid", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandler_UpdatePost_200(t *testing.T) {
	p := samplePost()
	p.Title = "Updated Title"

	svc := &stubPostService{
		updatePost: func(ctx context.Context, postID, authorID uuid.UUID, title, content *string, imageUploadIDs []string) (Post, error) {
			return p, nil
		},
	}

	r := setupTestRouter(svc, testauth.WithPrincipal(p.AuthorID.String()))
	newTitle := "Updated Title"
	body := UpdatePostRequest{Title: &newTitle}
	w := testhttp.DoRequest(t, r, http.MethodPut, "/api/v1/posts/"+p.ID.String(), body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := testhttp.DecodeResponse[PostResponse](t, w)
	if resp.Data.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %q", resp.Data.Title)
	}
}

func TestHandler_UpdatePost_403_Forbidden(t *testing.T) {
	svc := &stubPostService{
		updatePost: func(ctx context.Context, postID, authorID uuid.UUID, title, content *string, imageUploadIDs []string) (Post, error) {
			return Post{}, ErrPostForbidden
		},
	}

	r := setupTestRouter(svc, testauth.WithPrincipal(uuid.NewString()))
	newTitle := "Updated Title"
	body := UpdatePostRequest{Title: &newTitle}
	w := testhttp.DoRequest(t, r, http.MethodPut, "/api/v1/posts/"+uuid.NewString(), body)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_UpdatePost_404_NotFound(t *testing.T) {
	svc := &stubPostService{
		updatePost: func(ctx context.Context, postID, authorID uuid.UUID, title, content *string, imageUploadIDs []string) (Post, error) {
			return Post{}, ErrPostNotFound
		},
	}

	r := setupTestRouter(svc, testauth.WithPrincipal(uuid.NewString()))
	newTitle := "Updated Title"
	body := UpdatePostRequest{Title: &newTitle}
	w := testhttp.DoRequest(t, r, http.MethodPut, "/api/v1/posts/"+uuid.NewString(), body)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandler_DeletePost_204(t *testing.T) {
	svc := &stubPostService{
		deletePost: func(ctx context.Context, postID, authorID uuid.UUID) error {
			return nil
		},
	}

	r := setupTestRouter(svc, testauth.WithPrincipal(uuid.NewString()))
	w := testhttp.DoRequest(t, r, http.MethodDelete, "/api/v1/posts/"+uuid.NewString(), nil)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_DeletePost_403_Forbidden(t *testing.T) {
	svc := &stubPostService{
		deletePost: func(ctx context.Context, postID, authorID uuid.UUID) error {
			return ErrPostForbidden
		},
	}

	r := setupTestRouter(svc, testauth.WithPrincipal(uuid.NewString()))
	w := testhttp.DoRequest(t, r, http.MethodDelete, "/api/v1/posts/"+uuid.NewString(), nil)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_DeletePost_404_NotFound(t *testing.T) {
	svc := &stubPostService{
		deletePost: func(ctx context.Context, postID, authorID uuid.UUID) error {
			return ErrPostNotFound
		},
	}

	r := setupTestRouter(svc, testauth.WithPrincipal(uuid.NewString()))
	w := testhttp.DoRequest(t, r, http.MethodDelete, "/api/v1/posts/"+uuid.NewString(), nil)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
