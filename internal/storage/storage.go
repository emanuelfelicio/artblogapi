package storage

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

const (
	bucketPrefixQuarantine = "quarantine/"
	bucketPrefixFinal      = "final/"
)

type StorageProvider interface {
	GenerateUploadURL(ctx context.Context, key string, expires time.Duration) (string, error)
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
	PutObject(ctx context.Context, key string, reader io.Reader, contentType string) error
	DeleteObject(ctx context.Context, key string) error
	ObjectExists(ctx context.Context, key string) (bool, error)
}

// UploadProcessor is an abstraction for enqueuing uploads for background processing.
type UploadProcessor interface {
	Enqueue(ctx context.Context, uploadID uuid.UUID) error
}

type ImageContentType string

const (
	ContentTypeJPEG ImageContentType = "image/jpeg"
	ContentTypePNG  ImageContentType = "image/png"
	ContentTypeWebP ImageContentType = "image/webp"
)

var allowedImageTypes = map[ImageContentType]struct{}{
	ContentTypeJPEG: {},
	ContentTypePNG:  {},
	ContentTypeWebP: {},
}

func (i ImageContentType) Valid() bool {
	_, ok := allowedImageTypes[i]
	return ok
}

type UploadStatus string

const (
	UploadStatusPENDING    UploadStatus = "PENDING"
	UploadStatusPROCESSING UploadStatus = "PROCESSING"
	UploadStatusCOMPLETED  UploadStatus = "COMPLETED"
	UploadStatusREJECTED   UploadStatus = "REJECTED"
	UploadStatusEXPIRED    UploadStatus = "EXPIRED"
	UploadStatusSUPERSEDED UploadStatus = "SUPERSEDED"
	UploadStatusDELETED    UploadStatus = "DELETED"
	UploadStatusBOUND      UploadStatus = "BOUND"
)

type UploadPurpose string

const (
	PurposeAVATAR    UploadPurpose = "AVATAR"
	PurposeBANNER    UploadPurpose = "BANNER"
	PurposePOSTIMAGE UploadPurpose = "POST_IMAGE"
)

var allowedPurpose = map[UploadPurpose]struct{}{
	PurposeAVATAR:    {},
	PurposeBANNER:    {},
	PurposePOSTIMAGE: {},
}

func (p UploadPurpose) Valid() bool {
	_, ok := allowedPurpose[p]
	return ok
}

var uploadMaxSizeByPurpose = map[UploadPurpose]int{
	PurposeAVATAR:    5 << 20,  // 5 MiB
	PurposeBANNER:    8 << 20,  // 8 MiB
	PurposePOSTIMAGE: 20 << 20, // 20 MiB
}

func (p UploadPurpose) MaxFileSize() int {
	return uploadMaxSizeByPurpose[p]
}

func (p UploadPurpose) ValidateFileSize(size int) error {
	if size <= 0 {
		return ErrInvalidFileSize
	}
	if max := p.MaxFileSize(); size > max {
		return ErrFileSizeExceeded
	}
	return nil
}

func BuildQuarantineKey(id uuid.UUID) string {
	return bucketPrefixQuarantine + id.String()
}

func BuildFinalKey(id uuid.UUID) string {
	return bucketPrefixFinal + id.String()
}

func NewUpload(userID uuid.UUID, purpose UploadPurpose, fileSize int, contentType ImageContentType) (Upload, error) {
	if !purpose.Valid() {
		return Upload{}, ErrInvalidPurpose
	}
	if err := purpose.ValidateFileSize(fileSize); err != nil {
		return Upload{}, err
	}
	if !contentType.Valid() {
		return Upload{}, ErrInvaliImageContentType
	}

	uploadID, err := uuid.NewV7()
	if err != nil {
		return Upload{}, err
	}

	return Upload{
		ID:          uploadID,
		UserID:      userID,
		ObjectKey:   BuildQuarantineKey(uploadID),
		Status:      UploadStatusPENDING,
		Purpose:     purpose,
		FileSize:    fileSize,
		ContentType: contentType,
	}, nil
}

// Upload is the domain representation of an upload record
type Upload struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	ObjectKey     string
	Status        UploadStatus
	Purpose       UploadPurpose
	FileSize      int
	ContentType   ImageContentType
	FailureReason *string
	RetryCount    int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
