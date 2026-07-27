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
	findByUsername             func(ctx context.Context, username string) (User, error)
	findByID                   func(ctx context.Context, id uuid.UUID) (User, error)
	updateProfile              func(ctx context.Context, id uuid.UUID, displayName, bio *string) (User, error)
	updateAvatar               func(ctx context.Context, userID, uploadID uuid.UUID) error
	updateBanner               func(ctx context.Context, userID, uploadID uuid.UUID) error
	findCompletedUploadByOwner func(ctx context.Context, uploadID, userID uuid.UUID) error
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

func (s *stubRepository) FindCompletedUploadByOwner(ctx context.Context, uploadID, userID uuid.UUID) error {
	if s.findCompletedUploadByOwner != nil {
		return s.findCompletedUploadByOwner(ctx, uploadID, userID)
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

	got, err := svc.UpdateProfile(context.Background(), userID, new(name), new(bio))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DisplayName != name {
		t.Errorf("expected display_name %q, got %q", name, got.DisplayName)
	}
}

func TestService_UpdateProfile_OnlyDisplayName(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	name := "Partial Update"

	svc := NewService(&stubRepository{
		updateProfile: func(_ context.Context, _ uuid.UUID, dn, b *string) (User, error) {
			if dn == nil || *dn != name {
				t.Errorf("expected display_name %q", name)
			}
			if b != nil {
				t.Errorf("expected bio nil, got %v", b)
			}
			return User{DisplayName: *dn}, nil
		},
	})

	_, err := svc.UpdateProfile(context.Background(), userID, new(name), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_UpdateProfile_BothNil(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	called := false

	svc := NewService(&stubRepository{
		updateProfile: func(_ context.Context, _ uuid.UUID, dn, b *string) (User, error) {
			called = true
			if dn != nil || b != nil {
				t.Errorf("expected both nil, got dn=%v bio=%v", dn, b)
			}
			return User{}, nil
		},
	})

	_, err := svc.UpdateProfile(context.Background(), userID, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected UpdateProfile to be called on repository")
	}
}

func TestService_UpdateAvatar_Success(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	uploadID := uuid.New()

	svc := NewService(&stubRepository{
		findCompletedUploadByOwner: func(_ context.Context, uid, ownerID uuid.UUID) error {
			if uid != uploadID || ownerID != userID {
				t.Errorf("unexpected IDs: upload=%v owner=%v", uid, ownerID)
			}
			return nil
		},
		updateAvatar: func(_ context.Context, uid, upID uuid.UUID) error {
			if uid != userID || upID != uploadID {
				t.Errorf("unexpected IDs on update: user=%v upload=%v", uid, upID)
			}
			return nil
		},
	})

	if err := svc.UpdateAvatar(context.Background(), userID, uploadID.String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_UpdateAvatar_InvalidUUID(t *testing.T) {
	t.Parallel()
	svc := NewService(&stubRepository{})

	err := svc.UpdateAvatar(context.Background(), uuid.New(), "not-a-uuid")
	if !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("expected ErrUploadNotFound, got %v", err)
	}
}

func TestService_UpdateAvatar_UploadNotFound(t *testing.T) {
	t.Parallel()
	svc := NewService(&stubRepository{
		findCompletedUploadByOwner: func(_ context.Context, _, _ uuid.UUID) error {
			return ErrUploadNotFound
		},
	})

	err := svc.UpdateAvatar(context.Background(), uuid.New(), uuid.New().String())
	if !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("expected ErrUploadNotFound, got %v", err)
	}
}

func TestService_UpdateBanner_Success(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	uploadID := uuid.New()

	svc := NewService(&stubRepository{
		findCompletedUploadByOwner: func(_ context.Context, _, _ uuid.UUID) error {
			return nil
		},
		updateBanner: func(_ context.Context, uid, upID uuid.UUID) error {
			if uid != userID || upID != uploadID {
				t.Errorf("unexpected IDs: user=%v upload=%v", uid, upID)
			}
			return nil
		},
	})

	if err := svc.UpdateBanner(context.Background(), userID, uploadID.String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_UpdateBanner_UploadNotFound(t *testing.T) {
	t.Parallel()
	svc := NewService(&stubRepository{
		findCompletedUploadByOwner: func(_ context.Context, _, _ uuid.UUID) error {
			return ErrUploadNotFound
		},
	})

	err := svc.UpdateBanner(context.Background(), uuid.New(), uuid.New().String())
	if !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("expected ErrUploadNotFound, got %v", err)
	}
}
