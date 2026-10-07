package post

import (
	"testing"

	"github.com/google/uuid"
)

func TestRepository_PostLikes_AreIdempotentAndVisibleToViewer(t *testing.T) {
	ctx := setup(t)
	authorID := uuid.New()
	viewerID := uuid.New()
	otherViewerID := uuid.New()
	mustCreateUser(t, ctx, authorID, "likes_author")
	mustCreateUser(t, ctx, viewerID, "likes_viewer")
	mustCreateUser(t, ctx, otherViewerID, "likes_other")

	postID := uuid.New()
	if _, err := testRepo.CreatePost(ctx, postID, authorID, "Liked post", "Content"); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if err := testRepo.LikePost(ctx, postID, viewerID); err != nil {
		t.Fatalf("LikePost: %v", err)
	}
	if err := testRepo.LikePost(ctx, postID, viewerID); err != nil {
		t.Fatalf("repeated LikePost: %v", err)
	}
	if err := testRepo.LikePost(ctx, postID, otherViewerID); err != nil {
		t.Fatalf("LikePost other viewer: %v", err)
	}

	viewed, err := testRepo.GetPostWithImages(ctx, postID, &viewerID)
	if err != nil {
		t.Fatalf("GetPostWithImages liked viewer: %v", err)
	}
	if viewed.LikesCount != 2 || !viewed.LikedByMe {
		t.Fatalf("expected two likes and liked_by_me=true, got count=%d liked=%v", viewed.LikesCount, viewed.LikedByMe)
	}

	unlikedView, err := testRepo.GetPostWithImages(ctx, postID, &authorID)
	if err != nil {
		t.Fatalf("GetPostWithImages non-liked viewer: %v", err)
	}
	if unlikedView.LikesCount != 2 || unlikedView.LikedByMe {
		t.Fatalf("expected two likes and liked_by_me=false, got count=%d liked=%v", unlikedView.LikesCount, unlikedView.LikedByMe)
	}

	anonymousView, err := testRepo.GetPostWithImages(ctx, postID, nil)
	if err != nil {
		t.Fatalf("GetPostWithImages anonymous viewer: %v", err)
	}
	if anonymousView.LikesCount != 2 || anonymousView.LikedByMe {
		t.Fatalf("expected anonymous viewer to see count only, got count=%d liked=%v", anonymousView.LikesCount, anonymousView.LikedByMe)
	}

	if err := testRepo.UnlikePost(ctx, postID, viewerID); err != nil {
		t.Fatalf("UnlikePost: %v", err)
	}
	if err := testRepo.UnlikePost(ctx, postID, viewerID); err != nil {
		t.Fatalf("repeated UnlikePost: %v", err)
	}

	afterUnlike, err := testRepo.GetPostWithImages(ctx, postID, &viewerID)
	if err != nil {
		t.Fatalf("GetPostWithImages after unlike: %v", err)
	}
	if afterUnlike.LikesCount != 1 || afterUnlike.LikedByMe {
		t.Fatalf("expected one like and liked_by_me=false, got count=%d liked=%v", afterUnlike.LikesCount, afterUnlike.LikedByMe)
	}
}

func TestRepository_PostLikes_CascadeOnPostDelete(t *testing.T) {
	ctx := setup(t)
	authorID := uuid.New()
	viewerID := uuid.New()
	mustCreateUser(t, ctx, authorID, "cascade_author")
	mustCreateUser(t, ctx, viewerID, "cascade_viewer")

	postID := uuid.New()
	if _, err := testRepo.CreatePost(ctx, postID, authorID, "Cascade post", "Content"); err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if err := testRepo.LikePost(ctx, postID, viewerID); err != nil {
		t.Fatalf("LikePost: %v", err)
	}
	if err := testRepo.DeletePost(ctx, postID); err != nil {
		t.Fatalf("DeletePost: %v", err)
	}

	var count int
	if err := testDBPool.QueryRow(ctx, `SELECT COUNT(*) FROM post_likes WHERE post_id = $1`, postID).Scan(&count); err != nil {
		t.Fatalf("count post likes: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected cascaded likes to be deleted, got %d", count)
	}
}
