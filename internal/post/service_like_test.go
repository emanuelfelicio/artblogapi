package post

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestLikePost_MakesPostLikedByUser(t *testing.T) {
	postID := uuid.New()
	userID := uuid.New()
	likes := make(map[uuid.UUID]map[uuid.UUID]bool)
	repo := &stubRepository{
		getPostByIDForUpdate: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return Post{ID: id}, nil
		},
		likePost: func(ctx context.Context, gotPostID, gotUserID uuid.UUID) error {
			if likes[gotPostID] == nil {
				likes[gotPostID] = make(map[uuid.UUID]bool)
			}
			likes[gotPostID][gotUserID] = true
			return nil
		},
	}

	svc := NewService(repo, &stubMediaBinder{})
	if err := svc.LikePost(context.Background(), postID, userID); err != nil {
		t.Fatalf("LikePost: %v", err)
	}
	if !likes[postID][userID] {
		t.Fatal("expected post to be liked by user")
	}
}

func TestUnlikePost_RemovesPostLike(t *testing.T) {
	postID := uuid.New()
	userID := uuid.New()
	likes := map[uuid.UUID]map[uuid.UUID]bool{
		postID: {userID: true},
	}
	repo := &stubRepository{
		getPostByIDForUpdate: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return Post{ID: id}, nil
		},
		unlikePost: func(ctx context.Context, postID, userID uuid.UUID) error {
			delete(likes[postID], userID)
			return nil
		},
	}

	svc := NewService(repo, &stubMediaBinder{})
	if err := svc.UnlikePost(context.Background(), postID, userID); err != nil {
		t.Fatalf("UnlikePost: %v", err)
	}
	if likes[postID][userID] {
		t.Fatal("expected post like to be removed")
	}
}

func TestUnlikePost_MissingPostDoesNotChangeLikes(t *testing.T) {
	postID := uuid.New()
	userID := uuid.New()
	likes := map[uuid.UUID]map[uuid.UUID]bool{
		postID: {userID: true},
	}
	repo := &stubRepository{
		getPostByIDForUpdate: func(ctx context.Context, id uuid.UUID) (Post, error) {
			return Post{}, ErrPostNotFound
		},
		unlikePost: func(ctx context.Context, postID, userID uuid.UUID) error {
			delete(likes[postID], userID)
			return nil
		},
	}

	svc := NewService(repo, &stubMediaBinder{})
	if err := svc.UnlikePost(context.Background(), postID, userID); !errors.Is(err, ErrPostNotFound) {
		t.Fatalf("expected ErrPostNotFound, got %v", err)
	}
	if !likes[postID][userID] {
		t.Fatal("expected missing post not to change likes")
	}
}
