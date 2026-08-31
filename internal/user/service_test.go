package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- STUB ---

type stubRepository struct {
	findByUsername          func(ctx context.Context, username string) (User, error)
	findByID                func(ctx context.Context, id uuid.UUID) (User, error)
	updateProfile           func(ctx context.Context, id uuid.UUID, displayName, bio *string) (User, error)
	updateAvatar            func(ctx context.Context, userID, uploadID uuid.UUID) error
	updateBanner            func(ctx context.Context, userID, uploadID uuid.UUID) error
	withTransaction         func(ctx context.Context, fn func(repo Repository) error) error
	findUploadByIDForUpdate func(ctx context.Context, uploadID uuid.UUID) (UserUpload, error)
	findUserByIDForUpdate   func(ctx context.Context, userID uuid.UUID) (User, error)
	updateUploadStatus      func(ctx context.Context, uploadID uuid.UUID, status string) error
}

func (s *stubRepository) FindByUsername(ctx context.Context, username string) (User, error) {
	if s.findByUsername != nil {
		return s.findByUsername(ctx, username)
	}
	return User{}, nil
}

func (s *stubRepository) FindByID(ctx context.Context, id uuid.UUID) (User, error) {
	if s.findByID != nil {
		return s.findByID(ctx, id)
	}
	return User{}, nil
}

func (s *stubRepository) UpdateProfile(ctx context.Context, id uuid.UUID, displayName, bio *string) (User, error) {
	if s.updateProfile != nil {
		return s.updateProfile(ctx, id, displayName, bio)
	}
	return User{}, nil
}

func (s *stubRepository) UpdateAvatar(ctx context.Context, userID, uploadID uuid.UUID) error {
	if s.updateAvatar != nil {
		return s.updateAvatar(ctx, userID, uploadID)
	}
	return nil
}

func (s *stubRepository) UpdateBanner(ctx context.Context, userID, uploadID uuid.UUID) error {
	if s.updateBanner != nil {
		return s.updateBanner(ctx, userID, uploadID)
	}
	return nil
}

func (s *stubRepository) WithTransaction(ctx context.Context, fn func(repo Repository) error) error {
	if s.withTransaction != nil {
		return s.withTransaction(ctx, fn)
	}
	return fn(s)
}

func (s *stubRepository) FindUploadByIDForUpdate(ctx context.Context, uploadID uuid.UUID) (UserUpload, error) {
	if s.findUploadByIDForUpdate != nil {
		return s.findUploadByIDForUpdate(ctx, uploadID)
	}
	return UserUpload{}, nil
}

func (s *stubRepository) FindUserByIDForUpdate(ctx context.Context, userID uuid.UUID) (User, error) {
	if s.findUserByIDForUpdate != nil {
		return s.findUserByIDForUpdate(ctx, userID)
	}
	return User{}, nil
}

func (s *stubRepository) UpdateUploadStatus(ctx context.Context, uploadID uuid.UUID, status string) error {
	if s.updateUploadStatus != nil {
		return s.updateUploadStatus(ctx, uploadID, status)
	}
	return nil
}

// --- HELPERS ---

func newUser() User {
	return User{
		ID:          uuid.New(),
		Username:    "testuser",
		Email:       "test@example.com",
		DisplayName: "Test User",
		Bio:         "bio",
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// --- TESTS ---

func TestService_GetPublicProfile_Found(t *testing.T) {
	expected := newUser()
	svc := NewService(&stubRepository{
		findByUsername: func(_ context.Context, username string) (User, error) {
			if username != expected.Username {
				t.Fatalf("expected username %q, got %q", expected.Username, username)
			}
			return expected, nil
		},
	})

	got, err := svc.GetPublicProfile(context.Background(), expected.Username)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != expected.ID {
		t.Errorf("expected ID %v, got %v", expected.ID, got.ID)
	}
}

func TestService_GetPublicProfile_NotFound(t *testing.T) {
	svc := NewService(&stubRepository{
		findByUsername: func(_ context.Context, _ string) (User, error) {
			return User{}, ErrUserNotFound
		},
	})

	_, err := svc.GetPublicProfile(context.Background(), "ghost")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestService_GetMyProfile(t *testing.T) {
	expected := newUser()
	svc := NewService(&stubRepository{
		findByID: func(_ context.Context, id uuid.UUID) (User, error) {
			return expected, nil
		},
	})

	got, err := svc.GetMyProfile(context.Background(), expected.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Email != expected.Email {
		t.Errorf("expected email %q, got %q", expected.Email, got.Email)
	}
}

func TestService_UpdateProfile_AllFields(t *testing.T) {
	userID := uuid.New()
	name := "New Name"
	bio := "New bio"

	svc := NewService(&stubRepository{
		updateProfile: func(_ context.Context, id uuid.UUID, dn, b *string) (User, error) {
			if id != userID {
				t.Errorf("unexpected userID: %v", id)
			}
			if dn == nil || *dn != name {
				t.Errorf("expected display_name %q, got %v", name, dn)
			}
			if b == nil || *b != bio {
				t.Errorf("expected bio %q, got %v", bio, b)
			}
			return User{DisplayName: *dn, Bio: *b}, nil
		},
	})

	got, err := svc.UpdateProfile(context.Background(), userID, &name, &bio)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DisplayName != name {
		t.Errorf("expected display_name %q, got %q", name, got.DisplayName)
	}
}

func TestService_UpdateAvatar_Success(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	updatedStatus := false
	updatedAvatar := false

	svc := NewService(&stubRepository{
		findUploadByIDForUpdate: func(_ context.Context, uid uuid.UUID) (UserUpload, error) {
			return UserUpload{
				ID:      uploadID,
				UserID:  userID,
				Status:  "COMPLETED",
				Purpose: "AVATAR",
			}, nil
		},
		findUserByIDForUpdate: func(_ context.Context, uid uuid.UUID) (User, error) {
			return User{ID: userID}, nil
		},
		updateAvatar: func(_ context.Context, uid, upID uuid.UUID) error {
			if uid != userID || upID != uploadID {
				t.Errorf("unexpected IDs on update: user=%v upload=%v", uid, upID)
			}
			updatedAvatar = true
			return nil
		},
		updateUploadStatus: func(_ context.Context, uid uuid.UUID, status string) error {
			if uid != uploadID || status != "BOUND" {
				t.Errorf("unexpected status update: upload=%v status=%s", uid, status)
			}
			updatedStatus = true
			return nil
		},
	})

	if err := svc.UpdateAvatar(context.Background(), userID, uploadID.String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updatedAvatar || !updatedStatus {
		t.Error("expected updateAvatar and updateUploadStatus to be called")
	}
}

func TestService_UpdateAvatar_SupersedesOldAvatar(t *testing.T) {
	userID := uuid.New()
	oldAvatarID := uuid.New()
	newAvatarID := uuid.New()
	supersededCalled := false

	svc := NewService(&stubRepository{
		findUploadByIDForUpdate: func(_ context.Context, uid uuid.UUID) (UserUpload, error) {
			return UserUpload{
				ID:      newAvatarID,
				UserID:  userID,
				Status:  "COMPLETED",
				Purpose: "AVATAR",
			}, nil
		},
		findUserByIDForUpdate: func(_ context.Context, uid uuid.UUID) (User, error) {
			return User{ID: userID, AvatarUploadID: &oldAvatarID}, nil
		},
		updateUploadStatus: func(_ context.Context, uid uuid.UUID, status string) error {
			if uid == oldAvatarID && status == "SUPERSEDED" {
				supersededCalled = true
			}
			return nil
		},
	})

	if err := svc.UpdateAvatar(context.Background(), userID, newAvatarID.String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !supersededCalled {
		t.Error("expected old avatar to be marked SUPERSEDED")
	}
}

func TestService_UpdateAvatar_NotOwned(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	otherUserID := uuid.New()

	svc := NewService(&stubRepository{
		findUploadByIDForUpdate: func(_ context.Context, uid uuid.UUID) (UserUpload, error) {
			return UserUpload{
				ID:      uploadID,
				UserID:  otherUserID,
				Status:  "COMPLETED",
				Purpose: "AVATAR",
			}, nil
		},
	})

	err := svc.UpdateAvatar(context.Background(), userID, uploadID.String())
	if !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("expected ErrUploadNotFound, got %v", err)
	}
}

func TestService_UpdateAvatar_NotCompleted(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()

	svc := NewService(&stubRepository{
		findUploadByIDForUpdate: func(_ context.Context, uid uuid.UUID) (UserUpload, error) {
			return UserUpload{
				ID:      uploadID,
				UserID:  userID,
				Status:  "PENDING",
				Purpose: "AVATAR",
			}, nil
		},
	})

	err := svc.UpdateAvatar(context.Background(), userID, uploadID.String())
	if !errors.Is(err, ErrUploadNotCompleted) {
		t.Fatalf("expected ErrUploadNotCompleted, got %v", err)
	}
}

func TestService_UpdateAvatar_InvalidPurpose(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()

	svc := NewService(&stubRepository{
		findUploadByIDForUpdate: func(_ context.Context, uid uuid.UUID) (UserUpload, error) {
			return UserUpload{
				ID:      uploadID,
				UserID:  userID,
				Status:  "COMPLETED",
				Purpose: "BANNER",
			}, nil
		},
	})

	err := svc.UpdateAvatar(context.Background(), userID, uploadID.String())
	if !errors.Is(err, ErrUploadInvalidPurpose) {
		t.Fatalf("expected ErrUploadInvalidPurpose, got %v", err)
	}
}

func TestService_UpdateBanner_Success(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()
	updatedBanner := false

	svc := NewService(&stubRepository{
		findUploadByIDForUpdate: func(_ context.Context, uid uuid.UUID) (UserUpload, error) {
			return UserUpload{
				ID:      uploadID,
				UserID:  userID,
				Status:  "COMPLETED",
				Purpose: "BANNER",
			}, nil
		},
		findUserByIDForUpdate: func(_ context.Context, uid uuid.UUID) (User, error) {
			return User{ID: userID}, nil
		},
		updateBanner: func(_ context.Context, uid, upID uuid.UUID) error {
			updatedBanner = true
			return nil
		},
	})

	if err := svc.UpdateBanner(context.Background(), userID, uploadID.String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updatedBanner {
		t.Error("expected updateBanner to be called")
	}
}
