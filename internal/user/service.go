package user

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	FindByUsername(ctx context.Context, username string) (User, error)
	FindByID(ctx context.Context, id uuid.UUID) (User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, displayName, bio *string) (User, error)
	UpdateAvatar(ctx context.Context, userID, uploadID uuid.UUID) error
	UpdateBanner(ctx context.Context, userID, uploadID uuid.UUID) error
	FindCompletedUploadByOwner(ctx context.Context, uploadID, userID uuid.UUID) error
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

	if err := s.repo.FindCompletedUploadByOwner(ctx, uploadID, userID); err != nil {
		return err
	}

	return s.repo.UpdateAvatar(ctx, userID, uploadID)
}

func (s *service) UpdateBanner(ctx context.Context, userID uuid.UUID, uploadIDStr string) error {
	uploadID, err := uuid.Parse(uploadIDStr)
	if err != nil {
		// returns not found even for invalid UUID to avoid leaking rejection reason (resource enumeration)
		return ErrUploadNotFound
	}

	if err := s.repo.FindCompletedUploadByOwner(ctx, uploadID, userID); err != nil {
		return err
	}

	return s.repo.UpdateBanner(ctx, userID, uploadID)
}
