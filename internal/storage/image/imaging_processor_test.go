package image_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/emanuelfelicio/artblogapi/internal/storage"
	imgproc "github.com/emanuelfelicio/artblogapi/internal/storage/image"
	"golang.org/x/image/draw"
)

func createOpaqueJPEG(width, height int, t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 255, G: 0, B: 0, A: 255}}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return buf.Bytes()
}

func createTransparentPNG(width, height int, t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.NRGBA{R: 255, G: 0, B: 0, A: 128})
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func createOpaquePNG(width, height int, t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 0, G: 255, B: 0, A: 255}}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestImagingProcessor_Process(t *testing.T) {
	proc := imgproc.NewImagingProcessor()
	ctx := context.Background()

	t.Run("Avatar JPEG output and dimensions", func(t *testing.T) {
		inputBytes := createOpaqueJPEG(1000, 800, t)
		res, err := proc.Process(ctx, bytes.NewReader(inputBytes), storage.PurposeAVATAR)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ContentType != storage.ContentTypeJPEG {
			t.Errorf("expected ContentType %s, got %s", storage.ContentTypeJPEG, res.ContentType)
		}

		decoded, err := imaging.Decode(bytes.NewReader(res.Data))
		if err != nil {
			t.Fatalf("failed to decode result image: %v", err)
		}
		bounds := decoded.Bounds()
		if bounds.Dx() != imgproc.AvatarSize || bounds.Dy() != imgproc.AvatarSize {
			t.Errorf("expected dimensions %dx%d, got %dx%d", imgproc.AvatarSize, imgproc.AvatarSize, bounds.Dx(), bounds.Dy())
		}
	})

	t.Run("Banner JPEG output and dimensions", func(t *testing.T) {
		inputBytes := createOpaqueJPEG(1600, 1000, t)
		res, err := proc.Process(ctx, bytes.NewReader(inputBytes), storage.PurposeBANNER)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ContentType != storage.ContentTypeJPEG {
			t.Errorf("expected ContentType %s, got %s", storage.ContentTypeJPEG, res.ContentType)
		}

		decoded, err := imaging.Decode(bytes.NewReader(res.Data))
		if err != nil {
			t.Fatalf("failed to decode result image: %v", err)
		}
		bounds := decoded.Bounds()
		if bounds.Dx() != imgproc.BannerWidth || bounds.Dy() != imgproc.BannerHeight {
			t.Errorf("expected dimensions %dx%d, got %dx%d", imgproc.BannerWidth, imgproc.BannerHeight, bounds.Dx(), bounds.Dy())
		}
	})

	t.Run("PostImage transparent PNG preservation", func(t *testing.T) {
		inputBytes := createTransparentPNG(3000, 1500, t)
		res, err := proc.Process(ctx, bytes.NewReader(inputBytes), storage.PurposePOSTIMAGE)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ContentType != storage.ContentTypePNG {
			t.Errorf("expected ContentType %s, got %s", storage.ContentTypePNG, res.ContentType)
		}

		decoded, err := imaging.Decode(bytes.NewReader(res.Data))
		if err != nil {
			t.Fatalf("failed to decode result image: %v", err)
		}
		bounds := decoded.Bounds()
		if bounds.Dx() > imgproc.PostMaxDimension || bounds.Dy() > imgproc.PostMaxDimension {
			t.Errorf("expected max dimension %d, got %dx%d", imgproc.PostMaxDimension, bounds.Dx(), bounds.Dy())
		}
		// 3000x1500 (2:1 aspect ratio) fit into 2048x2048 should result in 2048x1024
		if bounds.Dx() != 2048 || bounds.Dy() != 1024 {
			t.Errorf("expected fit dimensions 2048x1024, got %dx%d", bounds.Dx(), bounds.Dy())
		}
	})

	t.Run("Opaque PNG converted to JPEG", func(t *testing.T) {
		inputBytes := createOpaquePNG(400, 400, t)
		res, err := proc.Process(ctx, bytes.NewReader(inputBytes), storage.PurposeAVATAR)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ContentType != storage.ContentTypeJPEG {
			t.Errorf("expected opaque PNG to be converted to %s, got %s", storage.ContentTypeJPEG, res.ContentType)
		}
	})

	t.Run("Unsupported format error", func(t *testing.T) {
		invalidBytes := []byte("this is not an image file format")
		_, err := proc.Process(ctx, bytes.NewReader(invalidBytes), storage.PurposeAVATAR)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, imgproc.ErrUnsupportedFormat) {
			t.Errorf("expected ErrUnsupportedFormat, got %v", err)
		}
	})

	t.Run("Invalid upload purpose error", func(t *testing.T) {
		inputBytes := createOpaqueJPEG(100, 100, t)
		_, err := proc.Process(ctx, bytes.NewReader(inputBytes), storage.UploadPurpose("UNKNOWN_PURPOSE"))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, imgproc.ErrInvalidPurpose) {
			t.Errorf("expected ErrInvalidPurpose, got %v", err)
		}
	})
}
