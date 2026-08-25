package image

import (
	"context"

	"io"

	"github.com/emanuelfelicio/artblogapi/internal/storage"
)

type ProcessedImage struct {
	Data        []byte
	ContentType storage.ImageContentType
}

type ImageProcessor interface {
	Process(ctx context.Context, r io.Reader, purpose storage.UploadPurpose) (ProcessedImage, error)
}

type DummyProcessor struct{}

func NewDummy() DummyProcessor {
	return DummyProcessor{}
}

func (d DummyProcessor) Process(ctx context.Context, r io.Reader, purpose storage.UploadPurpose) (ProcessedImage, error) {
	return ProcessedImage{}, nil
}
