package image

import (
	"context"
	"errors"
	"io"

	"github.com/emanuelfelicio/artblogapi/internal/storage"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported image format (supported: JPEG, PNG, WEBP)")
	ErrReadImage         = errors.New("failed to read image")
	ErrDecodeImage       = errors.New("failed to decode image")
	ErrEncodeImage       = errors.New("failed to encode image")
	ErrInvalidPurpose    = errors.New("invalid upload purpose for image processing")
	ErrImageTooLarge     = errors.New("image exceeds maximum allowed file size")
	ErrImageDimensions   = errors.New("image dimensions exceed maximum allowed limits")
)

type ProcessedImage struct {
	Data        []byte
	ContentType storage.ImageContentType
}

type ImageProcessor interface {
	Process(ctx context.Context, r io.Reader, purpose storage.UploadPurpose) (ProcessedImage, error)
}
