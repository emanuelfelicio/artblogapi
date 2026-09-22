package post

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/emanuelfelicio/artblogapi/internal/storage"
	"github.com/google/uuid"
)

type stubMediaBinder struct {
	bind      func(ctx context.Context, uploadID, userID uuid.UUID, purpose storage.UploadPurpose) error
	supersede func(ctx context.Context, uploadID uuid.UUID) error
}

func (s *stubMediaBinder) Bind(ctx context.Context, uploadID, userID uuid.UUID, purpose storage.UploadPurpose) error {
	if s.bind != nil {
		return s.bind(ctx, uploadID, userID, purpose)
	}
	return nil
}

func (s *stubMediaBinder) Supersede(ctx context.Context, uploadID uuid.UUID) error {
	if s.supersede != nil {
		return s.supersede(ctx, uploadID)
	}
	return nil
}

type stubRepository struct {
	withTransaction          func(ctx context.Context, fn func(txCtx context.Context) error) error
	createPost               func(ctx context.Context, id, authorID uuid.UUID, title, content string) (Post, error)
	getPostWithImages        func(ctx context.Context, id uuid.UUID) (Post, error)
	getPostByIDForUpdate     func(ctx context.Context, id uuid.UUID) (Post, error)
	updatePost               func(ctx context.Context, id uuid.UUID, title, content *string) (Post, error)
	deletePost               func(ctx context.Context, id uuid.UUID) error
	batchInsertPostImages    func(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID, positions []int16) error
	deletePostImages         func(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID) error
	updatePostImagePositions func(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID, positions []int16) error
	deletePostImagesByPostID func(ctx context.Context, postID uuid.UUID) ([]uuid.UUID, error)
	getPostImagesByPostIDs   func(ctx context.Context, postIDs []uuid.UUID) (map[uuid.UUID][]PostImage, error)
	listRecentPosts          func(ctx context.Context, limit, offset int32) ([]Post, error)
	listPostsByAuthor        func(ctx context.Context, authorID uuid.UUID, limit, offset int32) ([]Post, error)
}

func (r *stubRepository) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	if r.withTransaction != nil {
		return r.withTransaction(ctx, fn)
	}
	return fn(ctx)
}

func (r *stubRepository) CreatePost(ctx context.Context, id, authorID uuid.UUID, title, content string) (Post, error) {
	if r.createPost != nil {
		return r.createPost(ctx, id, authorID, title, content)
	}
	return Post{ID: id, AuthorID: authorID, Title: title, Content: content, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (r *stubRepository) GetPostWithImages(ctx context.Context, id uuid.UUID) (Post, error) {
	if r.getPostWithImages != nil {
		return r.getPostWithImages(ctx, id)
	}
	return Post{ID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (r *stubRepository) GetPostByIDForUpdate(ctx context.Context, id uuid.UUID) (Post, error) {
	if r.getPostByIDForUpdate != nil {
		return r.getPostByIDForUpdate(ctx, id)
	}
	return Post{ID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (r *stubRepository) UpdatePost(ctx context.Context, id uuid.UUID, title, content *string) (Post, error) {
	if r.updatePost != nil {
		return r.updatePost(ctx, id, title, content)
	}
	return Post{ID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func (r *stubRepository) DeletePost(ctx context.Context, id uuid.UUID) error {
	if r.deletePost != nil {
		return r.deletePost(ctx, id)
	}
	return nil
}

func (r *stubRepository) BatchInsertPostImages(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID, positions []int16) error {
	if r.batchInsertPostImages != nil {
		return r.batchInsertPostImages(ctx, postID, uploadIDs, positions)
	}
	return nil
}

func (r *stubRepository) DeletePostImages(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID) error {
	if r.deletePostImages != nil {
		return r.deletePostImages(ctx, postID, uploadIDs)
	}
	return nil
}

func (r *stubRepository) UpdatePostImagePositions(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID, positions []int16) error {
	if r.updatePostImagePositions != nil {
		return r.updatePostImagePositions(ctx, postID, uploadIDs, positions)
	}
	return nil
}

func (r *stubRepository) DeletePostImagesByPostID(ctx context.Context, postID uuid.UUID) ([]uuid.UUID, error) {
	if r.deletePostImagesByPostID != nil {
		return r.deletePostImagesByPostID(ctx, postID)
	}
	return nil, nil
}

func (r *stubRepository) GetPostImagesByPostIDs(ctx context.Context, postIDs []uuid.UUID) (map[uuid.UUID][]PostImage, error) {
	if r.getPostImagesByPostIDs != nil {
		return r.getPostImagesByPostIDs(ctx, postIDs)
	}
	return make(map[uuid.UUID][]PostImage), nil
}

func (r *stubRepository) ListRecentPosts(ctx context.Context, limit, offset int32) ([]Post, error) {
	if r.listRecentPosts != nil {
		return r.listRecentPosts(ctx, limit, offset)
	}
	return nil, nil
}

func (r *stubRepository) ListPostsByAuthor(ctx context.Context, authorID uuid.UUID, limit, offset int32) ([]Post, error) {
	if r.listPostsByAuthor != nil {
		return r.listPostsByAuthor(ctx, authorID, limit, offset)
	}
	return nil, nil
}

func TestCreatePost_Success_NoImages(t *testing.T) {
	repo := &stubRepository{
		getPostWithImages: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return Post{ID: id, Title: "My First Post", Content: "Hello world content", Images: []PostImage{}}, nil
		},
	}
	media := &stubMediaBinder{}
	svc := NewService(repo, media)

	authorID := uuid.New()
	p, err := svc.CreatePost(context.Background(), authorID, "My First Post", "Hello world content", nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if p.Title != "My First Post" {
		t.Errorf("expected title 'My First Post', got %s", p.Title)
	}
	if len(p.Images) != 0 {
		t.Errorf("expected 0 images, got %d", len(p.Images))
	}
}

func TestCreatePost_Success_WithImages(t *testing.T) {
	uID1 := uuid.New()
	uID2 := uuid.New()
	boundUploads := make([]uuid.UUID, 0)
	var batchInsertedIDs []uuid.UUID
	var batchPositions []int16

	media := &stubMediaBinder{
		bind: func(ctx context.Context, uploadID, userID uuid.UUID, purpose storage.UploadPurpose) error {
			if purpose != storage.PurposePOSTIMAGE {
				t.Errorf("expected purpose POST_IMAGE, got %s", purpose)
			}
			boundUploads = append(boundUploads, uploadID)
			return nil
		},
	}

	repo := &stubRepository{
		batchInsertPostImages: func(ctx context.Context, postID uuid.UUID, uploadIDs []uuid.UUID, positions []int16) error {
			batchInsertedIDs = uploadIDs
			batchPositions = positions
			return nil
		},
		getPostWithImages: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return Post{
				ID: id,
				Images: []PostImage{
					{PostID: id, UploadID: uID1, Position: 0},
					{PostID: id, UploadID: uID2, Position: 1},
				},
			}, nil
		},
	}

	svc := NewService(repo, media)
	authorID := uuid.New()
	p, err := svc.CreatePost(context.Background(), authorID, "Art Gallery", "Check my paintings", []string{uID1.String(), uID2.String()})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(boundUploads) != 2 {
		t.Fatalf("expected 2 bound uploads, got %d", len(boundUploads))
	}
	if len(batchInsertedIDs) != 2 {
		t.Fatalf("expected 2 batch inserted IDs, got %d", len(batchInsertedIDs))
	}
	if len(batchPositions) != 2 || batchPositions[0] != 0 || batchPositions[1] != 1 {
		t.Errorf("expected positions [0, 1], got %v", batchPositions)
	}
	if len(p.Images) != 2 {
		t.Fatalf("expected 2 images on returned post, got %d", len(p.Images))
	}
}

func TestCreatePost_Validation_EmptyTitle(t *testing.T) {
	svc := NewService(&stubRepository{}, &stubMediaBinder{})
	_, err := svc.CreatePost(context.Background(), uuid.New(), "   ", "Content", nil)
	if !errors.Is(err, ErrInvalidPostTitle) {
		t.Errorf("expected ErrInvalidPostTitle, got %v", err)
	}
}

func TestCreatePost_Validation_EmptyContent(t *testing.T) {
	svc := NewService(&stubRepository{}, &stubMediaBinder{})
	_, err := svc.CreatePost(context.Background(), uuid.New(), "Valid Title", "   ", nil)
	if !errors.Is(err, ErrInvalidPostContent) {
		t.Errorf("expected ErrInvalidPostContent, got %v", err)
	}
}

func TestCreatePost_Validation_MaxImagesExceeded(t *testing.T) {
	svc := NewService(&stubRepository{}, &stubMediaBinder{})
	ids := make([]string, 11)
	for i := range 11 {
		ids[i] = uuid.New().String()
	}

	_, err := svc.CreatePost(context.Background(), uuid.New(), "Title", "Content", ids)
	if !errors.Is(err, ErrMaxImagesExceeded) {
		t.Errorf("expected ErrMaxImagesExceeded, got %v", err)
	}
}

func TestCreatePost_Validation_DuplicateUploadID(t *testing.T) {
	svc := NewService(&stubRepository{}, &stubMediaBinder{})
	sameID := uuid.New().String()
	_, err := svc.CreatePost(context.Background(), uuid.New(), "Title", "Content", []string{sameID, sameID})
	if !errors.Is(err, ErrDuplicateUploadID) {
		t.Errorf("expected ErrDuplicateUploadID, got %v", err)
	}
}

func TestCreatePost_MediaBindError(t *testing.T) {
	expectedErr := errors.New("bind failed")
	media := &stubMediaBinder{
		bind: func(ctx context.Context, uploadID, userID uuid.UUID, purpose storage.UploadPurpose) error {
			return expectedErr
		},
	}
	svc := NewService(&stubRepository{}, media)
	_, err := svc.CreatePost(context.Background(), uuid.New(), "Title", "Content", []string{uuid.New().String()})
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestGetPost_Success(t *testing.T) {
	postID := uuid.New()
	authorID := uuid.New()
	imgID := uuid.New()

	repo := &stubRepository{
		getPostWithImages: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return Post{
				ID:        id,
				AuthorID:  authorID,
				Title:     "Fetched Post",
				Content:   "Fetched Content",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Images: []PostImage{
					{PostID: id, UploadID: imgID, Position: 0, ObjectKey: "final/key.jpg"},
				},
			}, nil
		},
	}

	svc := NewService(repo, &stubMediaBinder{})
	p, err := svc.GetPost(context.Background(), postID)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if p.Title != "Fetched Post" {
		t.Errorf("expected 'Fetched Post', got %s", p.Title)
	}
	if p.AuthorID != authorID {
		t.Errorf("expected authorID %s, got %s", authorID, p.AuthorID)
	}
	if len(p.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(p.Images))
	}
	if p.Images[0].ObjectKey != "final/key.jpg" {
		t.Errorf("expected object_key 'final/key.jpg', got %s", p.Images[0].ObjectKey)
	}
}

func TestGetPost_NotFound(t *testing.T) {
	repo := &stubRepository{
		getPostWithImages: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return Post{}, ErrPostNotFound
		},
	}

	svc := NewService(repo, &stubMediaBinder{})
	_, err := svc.GetPost(context.Background(), uuid.New())
	if !errors.Is(err, ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

func TestListRecentPosts_Success(t *testing.T) {
	post1ID := uuid.New()
	post2ID := uuid.New()

	repo := &stubRepository{
		listRecentPosts: func(ctx context.Context, limit, offset int32) ([]Post, error) {
			return []Post{
				{ID: post1ID, Title: "Post 1"},
				{ID: post2ID, Title: "Post 2"},
			}, nil
		},
		getPostImagesByPostIDs: func(ctx context.Context, postIDs []uuid.UUID) (map[uuid.UUID][]PostImage, error) {
			m := make(map[uuid.UUID][]PostImage)
			m[post1ID] = []PostImage{{PostID: post1ID, UploadID: uuid.New(), Position: 0}}
			m[post2ID] = []PostImage{}
			return m, nil
		},
	}

	svc := NewService(repo, &stubMediaBinder{})
	posts, err := svc.ListRecentPosts(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if len(posts[0].Images) != 1 {
		t.Errorf("expected post 1 to have 1 image, got %d", len(posts[0].Images))
	}
	if len(posts[1].Images) != 0 {
		t.Errorf("expected post 2 to have 0 images, got %d", len(posts[1].Images))
	}
}

func TestListPostsByAuthor_Success(t *testing.T) {
	authorID := uuid.New()
	postID := uuid.New()

	repo := &stubRepository{
		listPostsByAuthor: func(ctx context.Context, aID uuid.UUID, limit, offset int32) ([]Post, error) {
			if aID != authorID {
				t.Errorf("expected authorID %v, got %v", authorID, aID)
			}
			return []Post{{ID: postID, AuthorID: authorID, Title: "Author Post"}}, nil
		},
		getPostImagesByPostIDs: func(ctx context.Context, postIDs []uuid.UUID) (map[uuid.UUID][]PostImage, error) {
			m := make(map[uuid.UUID][]PostImage)
			m[postID] = []PostImage{{PostID: postID, UploadID: uuid.New(), Position: 0}}
			return m, nil
		},
	}

	svc := NewService(repo, &stubMediaBinder{})
	posts, err := svc.ListPostsByAuthor(context.Background(), authorID, 10, 0)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(posts))
	}
	if len(posts[0].Images) != 1 {
		t.Errorf("expected 1 image on author post, got %d", len(posts[0].Images))
	}
	if posts[0].AuthorID != authorID {
		t.Errorf("expected authorID %s, got %s", authorID, posts[0].AuthorID)
	}
}

func TestUpdatePost_Success_TextAndReconcileImages(t *testing.T) {
	postID := uuid.New()
	authorID := uuid.New()

	imgA := uuid.New()
	imgB := uuid.New()
	imgC := uuid.New()

	currentImages := []PostImage{
		{PostID: postID, UploadID: imgA, Position: 0},
		{PostID: postID, UploadID: imgB, Position: 1},
	}

	superseded := make([]uuid.UUID, 0)
	bound := make([]uuid.UUID, 0)
	var deletedIDs []uuid.UUID
	var insertedIDs []uuid.UUID
	var insertedPositions []int16
	var updatedIDs []uuid.UUID
	var updatedPositions []int16

	media := &stubMediaBinder{
		supersede: func(ctx context.Context, uploadID uuid.UUID) error {
			superseded = append(superseded, uploadID)
			return nil
		},
		bind: func(ctx context.Context, uploadID, userID uuid.UUID, purpose storage.UploadPurpose) error {
			bound = append(bound, uploadID)
			return nil
		},
	}

	repo := &stubRepository{
		getPostByIDForUpdate: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return Post{ID: id, AuthorID: authorID, Title: "Old Title", Content: "Old Content"}, nil
		},
		updatePost: func(ctx context.Context, id uuid.UUID, title, content *string) (Post, error) {
			return Post{ID: id, AuthorID: authorID, Title: *title, Content: *content}, nil
		},
		getPostImagesByPostIDs: func(ctx context.Context, postIDs []uuid.UUID) (map[uuid.UUID][]PostImage, error) {
			m := make(map[uuid.UUID][]PostImage)
			m[postID] = currentImages
			return m, nil
		},
		deletePostImages: func(ctx context.Context, pID uuid.UUID, uploadIDs []uuid.UUID) error {
			deletedIDs = uploadIDs
			return nil
		},
		batchInsertPostImages: func(ctx context.Context, pID uuid.UUID, uploadIDs []uuid.UUID, positions []int16) error {
			insertedIDs = uploadIDs
			insertedPositions = positions
			return nil
		},
		updatePostImagePositions: func(ctx context.Context, pID uuid.UUID, uploadIDs []uuid.UUID, positions []int16) error {
			updatedIDs = uploadIDs
			updatedPositions = positions
			return nil
		},
	}

	svc := NewService(repo, media)
	newTitle := "New Title"
	newContent := "New Content"
	desiredIDs := []string{imgC.String(), imgA.String()}

	p, err := svc.UpdatePost(context.Background(), postID, authorID, &newTitle, &newContent, desiredIDs)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if p.Title != "New Title" {
		t.Errorf("expected title 'New Title', got %s", p.Title)
	}
	if p.AuthorID != authorID {
		t.Errorf("expected authorID %s, got %s", authorID, p.AuthorID)
	}

	if len(superseded) != 1 || superseded[0] != imgB {
		t.Errorf("expected imgB to be superseded, got %v", superseded)
	}
	if len(deletedIDs) != 1 || deletedIDs[0] != imgB {
		t.Errorf("expected imgB in batch deletePostImages, got %v", deletedIDs)
	}

	if len(bound) != 1 || bound[0] != imgC {
		t.Errorf("expected imgC to be bound, got %v", bound)
	}
	if len(insertedIDs) != 1 || insertedIDs[0] != imgC {
		t.Errorf("expected imgC in batchInsertPostImages, got %v", insertedIDs)
	}
	if len(insertedPositions) != 1 || insertedPositions[0] != 0 {
		t.Errorf("expected imgC position 0, got %v", insertedPositions)
	}

	if len(updatedIDs) != 1 || updatedIDs[0] != imgA {
		t.Errorf("expected imgA in updatePostImagePositions, got %v", updatedIDs)
	}
	if len(updatedPositions) != 1 || updatedPositions[0] != 1 {
		t.Errorf("expected imgA new position 1, got %v", updatedPositions)
	}
}

func TestUpdatePost_Forbidden_NotAuthor(t *testing.T) {
	postID := uuid.New()
	authorID := uuid.New()
	otherUserID := uuid.New()

	repo := &stubRepository{
		getPostByIDForUpdate: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return Post{ID: id, AuthorID: authorID}, nil
		},
	}

	svc := NewService(repo, &stubMediaBinder{})
	title := "Updated"
	_, err := svc.UpdatePost(context.Background(), postID, otherUserID, &title, nil, nil)
	if !errors.Is(err, ErrPostForbidden) {
		t.Errorf("expected ErrPostForbidden, got %v", err)
	}
}

func TestDeletePost_Success(t *testing.T) {
	postID := uuid.New()
	authorID := uuid.New()
	img1 := uuid.New()
	img2 := uuid.New()

	superseded := make([]uuid.UUID, 0)
	deletedPost := false

	media := &stubMediaBinder{
		supersede: func(ctx context.Context, uploadID uuid.UUID) error {
			superseded = append(superseded, uploadID)
			return nil
		},
	}

	repo := &stubRepository{
		getPostByIDForUpdate: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return Post{ID: id, AuthorID: authorID}, nil
		},
		deletePostImagesByPostID: func(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
			return []uuid.UUID{img1, img2}, nil
		},
		deletePost: func(ctx context.Context, id uuid.UUID) error {
			deletedPost = true
			return nil
		},
	}

	svc := NewService(repo, media)
	err := svc.DeletePost(context.Background(), postID, authorID)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(superseded) != 2 {
		t.Errorf("expected 2 images superseded, got %d", len(superseded))
	}
	if !deletedPost {
		t.Errorf("expected post to be deleted")
	}
}

func TestDeletePost_Forbidden_NotAuthor(t *testing.T) {
	postID := uuid.New()
	authorID := uuid.New()
	otherUserID := uuid.New()

	repo := &stubRepository{
		getPostByIDForUpdate: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return Post{ID: id, AuthorID: authorID}, nil
		},
	}

	svc := NewService(repo, &stubMediaBinder{})
	err := svc.DeletePost(context.Background(), postID, otherUserID)
	if !errors.Is(err, ErrPostForbidden) {
		t.Errorf("expected ErrPostForbidden, got %v", err)
	}
}

func TestValidateTitle(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		expectErr bool
	}{
		{"valid title", "My Painting", false},
		{"trimmed title", "  Valid  ", false},
		{"empty title", "", true},
		{"whitespace only", "   ", true},
		{"title exceeds max length", strings.Repeat("a", MaxTitleLength+1), true},
		{"title at max length", strings.Repeat("a", MaxTitleLength), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateTitle(tt.title)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateTitle(%q) err = %v, expectErr = %v", tt.title, err, tt.expectErr)
			}
		})
	}
}

func TestValidateContent(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		expectErr bool
	}{
		{"valid content", "Some art description", false},
		{"empty content", "", true},
		{"whitespace only", "    ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateContent(tt.content)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateContent(%q) err = %v, expectErr = %v", tt.content, err, tt.expectErr)
			}
		})
	}
}

func TestValidateImagesCount(t *testing.T) {
	if err := ValidateImagesCount(0); err != nil {
		t.Errorf("expected 0 images to be valid, got %v", err)
	}
	if err := ValidateImagesCount(MaxImagesPerPost); err != nil {
		t.Errorf("expected max images (%d) to be valid, got %v", MaxImagesPerPost, err)
	}
	if err := ValidateImagesCount(MaxImagesPerPost + 1); !errors.Is(err, ErrMaxImagesExceeded) {
		t.Errorf("expected ErrMaxImagesExceeded, got %v", err)
	}
}

func TestNormalizePagination(t *testing.T) {
	l, o := NormalizePagination(0, -5)
	if l != DefaultPageLimit || o != 0 {
		t.Errorf("expected (%d, 0), got (%d, %d)", DefaultPageLimit, l, o)
	}

	l, o = NormalizePagination(100, 10)
	if l != DefaultPageLimit || o != 10 {
		t.Errorf("expected (%d, 10), got (%d, %d)", DefaultPageLimit, l, o)
	}

	l, o = NormalizePagination(30, 15)
	if l != 30 || o != 15 {
		t.Errorf("expected (30, 15), got (%d, %d)", l, o)
	}
}
