package image

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/disintegration/imaging"
	"github.com/emanuelfelicio/artblogapi/internal/storage"
	_ "golang.org/x/image/webp"
)

const (
	AvatarSize         = 512
	BannerWidth        = 1200
	BannerHeight       = 400
	PostMaxDimension   = 2048
	DefaultJPEGQuality = 85
)

type ImagingProcessor struct{}

func NewImagingProcessor() *ImagingProcessor {
	return &ImagingProcessor{}
}

func (p *ImagingProcessor) Process(ctx context.Context, r io.Reader, purpose storage.UploadPurpose) (ProcessedImage, error) {
	img, err := imaging.Decode(r, imaging.AutoOrientation(true))
	if err != nil {
		if errors.Is(err, image.ErrFormat) || errors.Is(err, imaging.ErrUnsupportedFormat) {
			return ProcessedImage{}, ErrUnsupportedFormat
		}
		return ProcessedImage{}, fmt.Errorf("%w: %v", ErrDecodeImage, err)
	}

	var processedImg image.Image
	switch purpose {
	case storage.PurposeAVATAR:
		processedImg = imaging.Fill(img, AvatarSize, AvatarSize, imaging.Center, imaging.Lanczos)
	case storage.PurposeBANNER:
		processedImg = imaging.Fill(img, BannerWidth, BannerHeight, imaging.Center, imaging.Lanczos)
	case storage.PurposePOSTIMAGE:
		bounds := img.Bounds()
		if bounds.Dx() > PostMaxDimension || bounds.Dy() > PostMaxDimension {
			processedImg = imaging.Fit(img, PostMaxDimension, PostMaxDimension, imaging.Lanczos)
		} else {
			processedImg = img
		}
	default:
		return ProcessedImage{}, ErrInvalidPurpose
	}

	var buf bytes.Buffer
	var contentType storage.ImageContentType

	if isOpaque(processedImg) {
		contentType = storage.ContentTypeJPEG
		err = jpeg.Encode(&buf, processedImg, &jpeg.Options{Quality: DefaultJPEGQuality})
	} else {
		contentType = storage.ContentTypePNG
		err = png.Encode(&buf, processedImg)
	}

	if err != nil {
		return ProcessedImage{}, fmt.Errorf("%w: %v", ErrEncodeImage, err)
	}

	return ProcessedImage{
		Data:        buf.Bytes(),
		ContentType: contentType,
	}, nil
}

func isOpaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return false
}
