package user

import (
	"context"

	"github.com/emanuelfelicio/artblogapi/internal/storage"
	"github.com/google/uuid"
)

type Repository interface {
	FindByUsername(ctx context.Context, username string) (User, error)
	FindByID(ctx context.Context, id uuid.UUID) (User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, displayName, bio *string) (User, error)
	UpdateAvatar(ctx context.Context, userID, uploadID uuid.UUID) error
	UpdateBanner(ctx context.Context, userID, uploadID uuid.UUID) error

	WithTransaction(ctx context.Context, fn func(repo Repository) error) error
	FindUploadByIDForUpdate(ctx context.Context, uploadID uuid.UUID) (UserUpload, error)
	FindUserByIDForUpdate(ctx context.Context, userID uuid.UUID) (User, error)
	UpdateUploadStatus(ctx context.Context, uploadID uuid.UUID, status string) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) *service {
	return &service{repo: repo}
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
		// returns not found even for invalid UUID to avoid leaking rejection reason (resource enumeration)
		return ErrUploadNotFound
	}

	return s.bindMedia(ctx, userID, uploadID, string(storage.PurposeAVATAR), func(txRepo Repository) error {
		return txRepo.UpdateAvatar(ctx, userID, uploadID)
	}, func(u User) *uuid.UUID {
		return u.AvatarUploadID
	})
}

func (s *service) UpdateBanner(ctx context.Context, userID uuid.UUID, uploadIDStr string) error {
	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		// returns not found even for invalid UUID to avoid leaking rejection reason (resource enumeration)
		return ErrUploadNotFound
	}

	return s.bindMedia(ctx, userID, uploadID, string(storage.PurposeBANNER), func(txRepo Repository) error {
		return txRepo.UpdateBanner(ctx, userID, uploadID)
	}, func(u User) *uuid.UUID {
		return u.BannerUploadID
	})
}

func (s *service) bindMedia(
	ctx context.Context,
	userID, uploadID uuid.UUID,
	expectedPurpose string,
	updateFK func(txRepo Repository) error,
	getOldUploadID func(u User) *uuid.UUID,
) error {
	return s.repo.WithTransaction(ctx, func(txRepo Repository) error {
		upload, err := txRepo.FindUploadByIDForUpdate(ctx, uploadID)
		if err != nil {
			return err
		}

		if upload.UserID != userID {
			return ErrUploadNotFound
		}

		if upload.Status != string(storage.UploadStatusCOMPLETED) && upload.Status != string(storage.UploadStatusBOUND) {
			return ErrUploadNotCompleted
		}

		if upload.Purpose != expectedPurpose {
			return ErrUploadInvalidPurpose
		}

		userEntity, err := txRepo.FindUserByIDForUpdate(ctx, userID)
		if err != nil {
			return err
		}

		oldUploadID := getOldUploadID(userEntity)
		if oldUploadID != nil && *oldUploadID != uploadID {
			if err := txRepo.UpdateUploadStatus(ctx, *oldUploadID, string(storage.UploadStatusSUPERSEDED)); err != nil {
				return err
			}
		}

		if err := updateFK(txRepo); err != nil {
			return err
		}

		if upload.Status != string(storage.UploadStatusBOUND) {
			if err := txRepo.UpdateUploadStatus(ctx, uploadID, string(storage.UploadStatusBOUND)); err != nil {
				return err
			}
		}

		return nil
	})
}
