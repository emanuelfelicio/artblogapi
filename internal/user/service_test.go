package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/emanuelfelicio/artblogapi/internal/storage"
	"github.com/google/uuid"
)

// --- STUB ---

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
	findByUsername          func(ctx context.Context, username string) (User, error)
	findByID                func(ctx context.Context, id uuid.UUID) (User, error)
	findPublicProfilesByIDs func(ctx context.Context, ids []uuid.UUID) ([]UserSummary, error)
	updateProfile           func(ctx context.Context, id uuid.UUID, displayName, bio *string) (User, error)
	updateAvatar            func(ctx context.Context, userID, uploadID uuid.UUID) error
	deleteAvatar            func(ctx context.Context, userID uuid.UUID) error
	updateBanner            func(ctx context.Context, userID, uploadID uuid.UUID) error
	deleteBanner            func(ctx context.Context, userID uuid.UUID) error
	withTransaction         func(ctx context.Context, fn func(txCtx context.Context) error) error
	findUserByIDForUpdate   func(ctx context.Context, userID uuid.UUID) (User, error)
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

func (s *stubRepository) FindPublicProfilesByIDs(ctx context.Context, ids []uuid.UUID) ([]UserSummary, error) {
	if s.findPublicProfilesByIDs != nil {
		return s.findPublicProfilesByIDs(ctx, ids)
	}
	return nil, nil
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

func (s *stubRepository) DeleteAvatar(ctx context.Context, userID uuid.UUID) error {
	if s.deleteAvatar != nil {
		return s.deleteAvatar(ctx, userID)
	}
	return nil
}

func (s *stubRepository) UpdateBanner(ctx context.Context, userID, uploadID uuid.UUID) error {
	if s.updateBanner != nil {
		return s.updateBanner(ctx, userID, uploadID)
	}
	return nil
}

func (s *stubRepository) DeleteBanner(ctx context.Context, userID uuid.UUID) error {
	if s.deleteBanner != nil {
		return s.deleteBanner(ctx, userID)
	}
	return nil
}

func (s *stubRepository) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	if s.withTransaction != nil {
		return s.withTransaction(ctx, fn)
	}
	return fn(ctx)
}

func (s *stubRepository) FindUserByIDForUpdate(ctx context.Context, userID uuid.UUID) (User, error) {
	if s.findUserByIDForUpdate != nil {
		return s.findUserByIDForUpdate(ctx, userID)
	}
	return User{}, nil
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
	}, &stubMediaBinder{})

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
	}, &stubMediaBinder{})

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
	}, &stubMediaBinder{})

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
	}, &stubMediaBinder{})

	got, err := svc.UpdateProfile(context.Background(), userID, &name, &bio)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DisplayName != name {
		t.Errorf("expected display_name %q, got %q", name, got.DisplayName)
	}
}

type fakeMediaBinder struct {
	boundUploads      map[uuid.UUID]storage.UploadPurpose
	supersededUploads map[uuid.UUID]bool
	bindErr           error
	supersedeErr      error
}

func newFakeMediaBinder() *fakeMediaBinder {
	return &fakeMediaBinder{
		boundUploads:      make(map[uuid.UUID]storage.UploadPurpose),
		supersededUploads: make(map[uuid.UUID]bool),
	}
}

func (f *fakeMediaBinder) Bind(_ context.Context, uploadID, userID uuid.UUID, purpose storage.UploadPurpose) error {
	if f.bindErr != nil {
		return f.bindErr
	}
	f.boundUploads[uploadID] = purpose
	return nil
}

func (f *fakeMediaBinder) Supersede(_ context.Context, uploadID uuid.UUID) error {
	if f.supersedeErr != nil {
		return f.supersedeErr
	}
	f.supersededUploads[uploadID] = true
	return nil
}

type fakeUserRepository struct {
	users map[uuid.UUID]User
}

func newFakeUserRepository(users ...User) *fakeUserRepository {
	m := make(map[uuid.UUID]User)
	for _, u := range users {
		m[u.ID] = u
	}
	return &fakeUserRepository{users: m}
}

func (f *fakeUserRepository) FindByUsername(_ context.Context, username string) (User, error) {
	for _, u := range f.users {
		if u.Username == username {
			return u, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (f *fakeUserRepository) FindByID(_ context.Context, id uuid.UUID) (User, error) {
	u, ok := f.users[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUserRepository) UpdateProfile(_ context.Context, id uuid.UUID, dn, b *string) (User, error) {
	u, ok := f.users[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	if dn != nil {
		u.DisplayName = *dn
	}
	if b != nil {
		u.Bio = *b
	}
	f.users[id] = u
	return u, nil
}

func (f *fakeUserRepository) UpdateAvatar(_ context.Context, userID, uploadID uuid.UUID) error {
	u, ok := f.users[userID]
	if !ok {
		return ErrUserNotFound
	}
	u.AvatarUploadID = &uploadID
	f.users[userID] = u
	return nil
}

func (f *fakeUserRepository) DeleteAvatar(_ context.Context, userID uuid.UUID) error {
	u, ok := f.users[userID]
	if !ok {
		return ErrUserNotFound
	}
	u.AvatarUploadID = nil
	f.users[userID] = u
	return nil
}

func (f *fakeUserRepository) UpdateBanner(_ context.Context, userID, uploadID uuid.UUID) error {
	u, ok := f.users[userID]
	if !ok {
		return ErrUserNotFound
	}
	u.BannerUploadID = &uploadID
	f.users[userID] = u
	return nil
}

func (f *fakeUserRepository) DeleteBanner(_ context.Context, userID uuid.UUID) error {
	u, ok := f.users[userID]
	if !ok {
		return ErrUserNotFound
	}
	u.BannerUploadID = nil
	f.users[userID] = u
	return nil
}

func (f *fakeUserRepository) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

func (f *fakeUserRepository) FindUserByIDForUpdate(_ context.Context, userID uuid.UUID) (User, error) {
	u, ok := f.users[userID]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUserRepository) FindPublicProfilesByIDs(_ context.Context, ids []uuid.UUID) ([]UserSummary, error) {
	return nil, nil
}

func TestService_UpdateAvatar_Success(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()

	repo := newFakeUserRepository(User{ID: userID})
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	if err := svc.UpdateAvatar(context.Background(), userID, uploadID.String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updatedUser, _ := repo.FindByID(context.Background(), userID)
	if updatedUser.AvatarUploadID == nil || *updatedUser.AvatarUploadID != uploadID {
		t.Errorf("expected user avatar to be %v, got %v", uploadID, updatedUser.AvatarUploadID)
	}
	if media.boundUploads[uploadID] != storage.PurposeAVATAR {
		t.Errorf("expected upload %v to be bound as AVATAR", uploadID)
	}
	if len(media.supersededUploads) != 0 {
		t.Errorf("expected no superseded uploads, got %v", media.supersededUploads)
	}
}

func TestService_UpdateAvatar_Idempotent(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()

	repo := newFakeUserRepository(User{ID: userID, AvatarUploadID: &uploadID})
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	if err := svc.UpdateAvatar(context.Background(), userID, uploadID.String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updatedUser, _ := repo.FindByID(context.Background(), userID)
	if updatedUser.AvatarUploadID == nil || *updatedUser.AvatarUploadID != uploadID {
		t.Errorf("expected avatar to remain %v", uploadID)
	}
	if len(media.boundUploads) > 0 || len(media.supersededUploads) > 0 {
		t.Error("expected no media state changes when avatar is already active")
	}
}

func TestService_UpdateAvatar_SupersedesOldAvatar(t *testing.T) {
	userID := uuid.New()
	oldAvatarID := uuid.New()
	newAvatarID := uuid.New()

	repo := newFakeUserRepository(User{ID: userID, AvatarUploadID: &oldAvatarID})
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	if err := svc.UpdateAvatar(context.Background(), userID, newAvatarID.String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updatedUser, _ := repo.FindByID(context.Background(), userID)
	if updatedUser.AvatarUploadID == nil || *updatedUser.AvatarUploadID != newAvatarID {
		t.Errorf("expected user avatar to be %v, got %v", newAvatarID, updatedUser.AvatarUploadID)
	}
	if !media.supersededUploads[oldAvatarID] {
		t.Errorf("expected previous avatar %v to be superseded", oldAvatarID)
	}
	if media.boundUploads[newAvatarID] != storage.PurposeAVATAR {
		t.Errorf("expected new avatar %v to be bound as AVATAR", newAvatarID)
	}
}

func TestService_UpdateAvatar_InvalidUUID(t *testing.T) {
	userID := uuid.New()
	repo := newFakeUserRepository(User{ID: userID})
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	err := svc.UpdateAvatar(context.Background(), userID, "invalid-uuid")
	if !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("expected ErrUploadNotFound, got %v", err)
	}

	user, _ := repo.FindByID(context.Background(), userID)
	if user.AvatarUploadID != nil {
		t.Errorf("expected avatar to remain nil on invalid UUID format, got %v", user.AvatarUploadID)
	}
}

func TestService_UpdateAvatar_MediaErrors(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()

	tests := []struct {
		name    string
		bindErr error
		wantErr error
	}{
		{
			name:    "not found error",
			bindErr: ErrUploadNotFound,
			wantErr: ErrUploadNotFound,
		},
		{
			name:    "not completed error",
			bindErr: ErrUploadNotCompleted,
			wantErr: ErrUploadNotCompleted,
		},
		{
			name:    "invalid purpose error",
			bindErr: ErrUploadInvalidPurpose,
			wantErr: ErrUploadInvalidPurpose,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeUserRepository(User{ID: userID})
			media := newFakeMediaBinder()
			media.bindErr = tc.bindErr
			svc := NewService(repo, media)

			err := svc.UpdateAvatar(context.Background(), userID, uploadID.String())
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}

			user, _ := repo.FindByID(context.Background(), userID)
			if user.AvatarUploadID != nil {
				t.Errorf("expected user avatar to remain unchanged on error, got %v", user.AvatarUploadID)
			}
		})
	}
}

func TestService_UpdateBanner_Success(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()

	repo := newFakeUserRepository(User{ID: userID})
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	if err := svc.UpdateBanner(context.Background(), userID, uploadID.String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updatedUser, _ := repo.FindByID(context.Background(), userID)
	if updatedUser.BannerUploadID == nil || *updatedUser.BannerUploadID != uploadID {
		t.Errorf("expected user banner to be %v, got %v", uploadID, updatedUser.BannerUploadID)
	}
	if media.boundUploads[uploadID] != storage.PurposeBANNER {
		t.Errorf("expected upload %v to be bound as BANNER", uploadID)
	}
	if len(media.supersededUploads) != 0 {
		t.Errorf("expected no superseded uploads, got %v", media.supersededUploads)
	}
}

func TestService_UpdateBanner_SupersedesOldBanner(t *testing.T) {
	userID := uuid.New()
	oldBannerID := uuid.New()
	newBannerID := uuid.New()

	repo := newFakeUserRepository(User{ID: userID, BannerUploadID: &oldBannerID})
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	if err := svc.UpdateBanner(context.Background(), userID, newBannerID.String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updatedUser, _ := repo.FindByID(context.Background(), userID)
	if updatedUser.BannerUploadID == nil || *updatedUser.BannerUploadID != newBannerID {
		t.Errorf("expected user banner to be %v, got %v", newBannerID, updatedUser.BannerUploadID)
	}
	if !media.supersededUploads[oldBannerID] {
		t.Errorf("expected previous banner %v to be superseded", oldBannerID)
	}
	if media.boundUploads[newBannerID] != storage.PurposeBANNER {
		t.Errorf("expected new banner %v to be bound as BANNER", newBannerID)
	}
}

func TestService_UpdateBanner_Idempotent(t *testing.T) {
	userID := uuid.New()
	uploadID := uuid.New()

	repo := newFakeUserRepository(User{ID: userID, BannerUploadID: &uploadID})
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	if err := svc.UpdateBanner(context.Background(), userID, uploadID.String()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updatedUser, _ := repo.FindByID(context.Background(), userID)
	if updatedUser.BannerUploadID == nil || *updatedUser.BannerUploadID != uploadID {
		t.Errorf("expected banner to remain %v", uploadID)
	}
	if len(media.boundUploads) > 0 || len(media.supersededUploads) > 0 {
		t.Error("expected no media state changes when banner is already active")
	}
}

func TestService_UpdateBanner_InvalidUUID(t *testing.T) {
	userID := uuid.New()
	repo := newFakeUserRepository(User{ID: userID})
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	err := svc.UpdateBanner(context.Background(), userID, "invalid-uuid")
	if !errors.Is(err, ErrUploadNotFound) {
		t.Fatalf("expected ErrUploadNotFound, got %v", err)
	}
}

func TestService_DeleteAvatar_Success(t *testing.T) {
	userID := uuid.New()
	avatarID := uuid.New()

	repo := newFakeUserRepository(User{ID: userID, AvatarUploadID: &avatarID})
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	if err := svc.DeleteAvatar(context.Background(), userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updatedUser, _ := repo.FindByID(context.Background(), userID)
	if updatedUser.AvatarUploadID != nil {
		t.Errorf("expected avatar to be nil, got %v", updatedUser.AvatarUploadID)
	}
	if !media.supersededUploads[avatarID] {
		t.Errorf("expected avatar %v to be marked superseded", avatarID)
	}
}

func TestService_DeleteAvatar_Idempotent(t *testing.T) {
	userID := uuid.New()

	repo := newFakeUserRepository(User{ID: userID, AvatarUploadID: nil})
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	if err := svc.DeleteAvatar(context.Background(), userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(media.supersededUploads) > 0 {
		t.Errorf("expected no superseded calls on nil avatar, got %v", media.supersededUploads)
	}
}

func TestService_DeleteAvatar_UserNotFound(t *testing.T) {
	userID := uuid.New()
	repo := newFakeUserRepository()
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	err := svc.DeleteAvatar(context.Background(), userID)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestService_DeleteAvatar_SupersedeError(t *testing.T) {
	userID := uuid.New()
	avatarID := uuid.New()

	repo := newFakeUserRepository(User{ID: userID, AvatarUploadID: &avatarID})
	media := newFakeMediaBinder()
	media.supersedeErr = errors.New("storage error")
	svc := NewService(repo, media)

	err := svc.DeleteAvatar(context.Background(), userID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	user, _ := repo.FindByID(context.Background(), userID)
	if user.AvatarUploadID == nil || *user.AvatarUploadID != avatarID {
		t.Errorf("expected avatar to remain %v on error, got %v", avatarID, user.AvatarUploadID)
	}
}

func TestService_DeleteBanner_Success(t *testing.T) {
	userID := uuid.New()
	bannerID := uuid.New()

	repo := newFakeUserRepository(User{ID: userID, BannerUploadID: &bannerID})
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	if err := svc.DeleteBanner(context.Background(), userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updatedUser, _ := repo.FindByID(context.Background(), userID)
	if updatedUser.BannerUploadID != nil {
		t.Errorf("expected banner to be nil, got %v", updatedUser.BannerUploadID)
	}
	if !media.supersededUploads[bannerID] {
		t.Errorf("expected banner %v to be marked superseded", bannerID)
	}
}

func TestService_DeleteBanner_Idempotent(t *testing.T) {
	userID := uuid.New()

	repo := newFakeUserRepository(User{ID: userID, BannerUploadID: nil})
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	if err := svc.DeleteBanner(context.Background(), userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(media.supersededUploads) > 0 {
		t.Errorf("expected no superseded calls on nil banner, got %v", media.supersededUploads)
	}
}

func TestService_DeleteBanner_UserNotFound(t *testing.T) {
	userID := uuid.New()
	repo := newFakeUserRepository()
	media := newFakeMediaBinder()
	svc := NewService(repo, media)

	err := svc.DeleteBanner(context.Background(), userID)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestService_DeleteBanner_SupersedeError(t *testing.T) {
	userID := uuid.New()
	bannerID := uuid.New()

	repo := newFakeUserRepository(User{ID: userID, BannerUploadID: &bannerID})
	media := newFakeMediaBinder()
	media.supersedeErr = errors.New("storage error")
	svc := NewService(repo, media)

	err := svc.DeleteBanner(context.Background(), userID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	user, _ := repo.FindByID(context.Background(), userID)
	if user.BannerUploadID == nil || *user.BannerUploadID != bannerID {
		t.Errorf("expected banner to remain %v on error, got %v", bannerID, user.BannerUploadID)
	}
}

func TestService_GetPublicProfilesByIDs_Deduplication(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()

	var capturedIDs []uuid.UUID
	repo := &stubRepository{
		findPublicProfilesByIDs: func(_ context.Context, ids []uuid.UUID) ([]UserSummary, error) {
			capturedIDs = ids
			return []UserSummary{{ID: id1}, {ID: id2}}, nil
		},
	}

	svc := NewService(repo, &stubMediaBinder{})
	result, err := svc.GetPublicProfilesByIDs(context.Background(), []string{
		id1.String(), id2.String(), id1.String(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedIDs) != 2 {
		t.Errorf("expected 2 unique IDs sent to repo, got %d", len(capturedIDs))
	}
	if len(result) != 2 {
		t.Errorf("expected 2 summaries, got %d", len(result))
	}
}

func TestService_GetPublicProfilesByIDs_SkipsInvalidUUIDs(t *testing.T) {
	id1 := uuid.New()

	var capturedIDs []uuid.UUID
	repo := &stubRepository{
		findPublicProfilesByIDs: func(_ context.Context, ids []uuid.UUID) ([]UserSummary, error) {
			capturedIDs = ids
			return []UserSummary{{ID: id1}}, nil
		},
	}

	svc := NewService(repo, &stubMediaBinder{})
	result, err := svc.GetPublicProfilesByIDs(context.Background(), []string{
		id1.String(), "not-a-uuid", "also-invalid",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedIDs) != 1 {
		t.Errorf("expected 1 valid ID sent to repo, got %d", len(capturedIDs))
	}
	if len(result) != 1 {
		t.Errorf("expected 1 summary, got %d", len(result))
	}
}
