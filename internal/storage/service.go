package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, u Upload) (uuid.UUID, error)
}

type Service struct {
	repo       Repository
	provider   StorageProvider
	processor  UploadProcessor
	presignTTL time.Duration
	logger     *slog.Logger
}

func NewService(repo Repository, provider StorageProvider, processor UploadProcessor, presignTTL time.Duration, logger *slog.Logger) *Service {
	return &Service{repo: repo, provider: provider, processor: processor, presignTTL: presignTTL, logger: logger}
}

func (s *Service) InitUpload(ctx context.Context, userID uuid.UUID, purpose string, fileSize int, contentType string) (uuid.UUID, string, error) {
	upPurpose := UploadPurpose(purpose)
	if !upPurpose.Valid() {
		return uuid.Nil, "", ErrInvalidPurpose
	}

	imgContentType := ImageContentType(contentType)

	if !imgContentType.Valid() {
		return uuid.Nil, "", ErrInvaliImageContentType

	}

	uploadID, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("generate_uuid_v7: %w", err)
	}

	objectKey := BucketPrefixQuarantine + uploadID.String()
	presignedURL, err := s.provider.GenerateUploadURL(ctx, objectKey, s.presignTTL)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("generate_upload_url: %w", err)
	}

	upload := Upload{
		ID:          uploadID,
		UserID:      userID,
		ObjectKey:   objectKey,
		Status:      UploadStatusPENDING,
		Purpose:     upPurpose,
		FileSize:    fileSize,
		ContentType: imgContentType,
	}

	uploadID, err = s.repo.Create(ctx, upload)
	if err != nil {
		return uuid.Nil, "", err
	}

	return uploadID, presignedURL, nil
}
