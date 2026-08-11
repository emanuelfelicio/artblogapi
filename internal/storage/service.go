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
	GetByID(ctx context.Context, id uuid.UUID) (Upload, error)
	SetStatusProcessing(ctx context.Context, id uuid.UUID) error
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

	if err := upPurpose.ValidateFileSize(fileSize); err != nil {
		return uuid.Nil, "", err
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

func (s *Service) CompleteUpload(ctx context.Context, userID uuid.UUID, uploadID uuid.UUID) error {
	upload, err := s.repo.GetByID(ctx, uploadID)
	if err != nil {
		return err
	}

	if upload.UserID != userID {
		return ErrUploadNotOwned
	}

	if upload.Status != UploadStatusPENDING {
		return ErrUploadNotPending
	}

	quarantineKey := BucketPrefixQuarantine + uploadID.String()
	exists, err := s.provider.ObjectExists(ctx, quarantineKey)
	if err != nil {
		return fmt.Errorf("check_quarantine_object: %w", err)
	}
	if !exists {
		return ErrFileNotInQuarantine
	}

	if err := s.repo.SetStatusProcessing(ctx, uploadID); err != nil {
		return err
	}

	if err := s.processor.Enqueue(ctx, uploadID); err != nil {
		return fmt.Errorf("enqueue_upload: %w", err)
	}

	return nil
}

func (s *Service) GetUploadStatus(ctx context.Context, userID uuid.UUID, uploadID uuid.UUID) (Upload, error) {
	upload, err := s.repo.GetByID(ctx, uploadID)
	if err != nil {
		return Upload{}, err
	}

	if upload.UserID != userID {
		return Upload{}, ErrUploadNotOwned
	}

	return upload, nil
}
