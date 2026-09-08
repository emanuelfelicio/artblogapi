package user

import (
	"context"

	"github.com/emanuelfelicio/artblogapi/internal/storage"
	"github.com/google/uuid"
)

type MediaBinder interface {
	Bind(ctx context.Context, uploadID, userID uuid.UUID, purpose storage.UploadPurpose) error
	Supersede(ctx context.Context, uploadID uuid.UUID) error
}

type Repository interface {
	FindByUsername(ctx context.Context, username string) (User, error)
	FindByID(ctx context.Context, id uuid.UUID) (User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, displayName, bio *string) (User, error)
	UpdateAvatar(ctx context.Context, userID, uploadID uuid.UUID) error
	UpdateBanner(ctx context.Context, userID, uploadID uuid.UUID) error

	WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
	FindUserByIDForUpdate(ctx context.Context, userID uuid.UUID) (User, error)
}

type service struct {
	repo  Repository
	media MediaBinder
}

func NewService(repo Repository, media MediaBinder) *service {
	return &service{repo: repo, media: media}
}

func (s *service) GetPublicProfile(ctx context.Context, username string) (User, error) {
	return s.repo.FindByUsername(ctx, username)
}

func (s *service) GetMyProfile(ctx context.Context, userID uuid.UUID) (User, error) {
	return s.repo.FindByID(ctx, userID)
}

func (s *service) UpdateProfile(ctx context.Context, userID uuid.UUID, displayName, bio *string) (User, error) {
	return s.repo.UpdateProfile(ctx, userID, displayName, bio)
}

func (s *service) UpdateAvatar(ctx context.Context, userID uuid.UUID, uploadIDStr string) error {
	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		return ErrUploadNotFound
	}

	return s.bindMedia(ctx, userID, uploadID, storage.PurposeAVATAR, func(txCtx context.Context) error {
		return s.repo.UpdateAvatar(txCtx, userID, uploadID)
	}, func(u User) *uuid.UUID {
		return u.AvatarUploadID
	})
}

func (s *service) UpdateBanner(ctx context.Context, userID uuid.UUID, uploadIDStr string) error {
	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		return ErrUploadNotFound
	}

	return s.bindMedia(ctx, userID, uploadID, storage.PurposeBANNER, func(txCtx context.Context) error {
		return s.repo.UpdateBanner(txCtx, userID, uploadID)
	}, func(u User) *uuid.UUID {
		return u.BannerUploadID
	})
}

func (s *service) bindMedia(
	ctx context.Context,
	userID, uploadID uuid.UUID,
	expectedPurpose storage.UploadPurpose,
	updateFK func(txCtx context.Context) error,
	getOldUploadID func(u User) *uuid.UUID,
) error {
	return s.repo.WithTransaction(ctx, func(txCtx context.Context) error {
		userEntity, err := s.repo.FindUserByIDForUpdate(txCtx, userID)
		if err != nil {
			return err
		}

		oldUploadID := getOldUploadID(userEntity)
		if oldUploadID != nil && *oldUploadID == uploadID {
			return nil
		}

		if err := s.media.Bind(txCtx, uploadID, userID, expectedPurpose); err != nil {
			return err
		}

		if oldUploadID != nil {
			if err := s.media.Supersede(txCtx, *oldUploadID); err != nil {
				return err
			}
		}

		return updateFK(txCtx)
	})
}
