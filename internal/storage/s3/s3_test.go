package s3

import (
	"bytes"
	"context"
	"errors"
	"github.com/emanuelfelicio/artblogapi/internal/storage"
	"io"
	"testing"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// --- MOCKS ---

type mockS3Client struct {
	getObject    func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	putObject    func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	deleteObject func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	headObject   func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getObject != nil {
		return m.getObject(ctx, params, optFns...)
	}
	return &s3.GetObjectOutput{}, nil
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.putObject != nil {
		return m.putObject(ctx, params, optFns...)
	}
	return &s3.PutObjectOutput{}, nil
}

func (m *mockS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if m.deleteObject != nil {
		return m.deleteObject(ctx, params, optFns...)
	}
	return &s3.DeleteObjectOutput{}, nil
}

func (m *mockS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if m.headObject != nil {
		return m.headObject(ctx, params, optFns...)
	}
	return &s3.HeadObjectOutput{}, nil
}

type mockS3Presign struct {
	presignPutObject func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

func (m *mockS3Presign) PresignPutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	if m.presignPutObject != nil {
		return m.presignPutObject(ctx, params, optFns...)
	}
	return &v4.PresignedHTTPRequest{}, nil
}

// --- TESTS ---

func TestS3StorageProvider_GenerateUploadURL(t *testing.T) {
	bucket := "test-bucket"
	key := "quarantine/upload-id"
	expectedURL := "https://s3.example.com/presigned-url"

	presigner := &mockS3Presign{
		presignPutObject: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
			if *params.Bucket != bucket {
				t.Errorf("expected bucket %q, got %q", bucket, *params.Bucket)
			}
			if *params.Key != key {
				t.Errorf("expected key %q, got %q", key, *params.Key)
			}
			return &v4.PresignedHTTPRequest{
				URL: expectedURL,
			}, nil
		},
	}

	provider := NewS3StorageProvider(&mockS3Client{}, presigner, bucket)
	url, err := provider.GenerateUploadURL(context.Background(), key, storage.ContentTypePNG, 15*time.Minute)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != expectedURL {
		t.Errorf("expected URL %q, got %q", expectedURL, url)
	}
}

func TestS3StorageProvider_GenerateUploadURL_Error(t *testing.T) {
	presignErr := errors.New("presign failed")
	presigner := &mockS3Presign{
		presignPutObject: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
			return nil, presignErr
		},
	}

	provider := NewS3StorageProvider(&mockS3Client{}, presigner, "bucket")
	_, err := provider.GenerateUploadURL(context.Background(), "key", storage.ContentTypePNG, 15*time.Minute)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, presignErr) {
		t.Errorf("expected error containing %q, got %v", presignErr.Error(), err)
	}
}

func TestS3StorageProvider_GetObject(t *testing.T) {
	bucket := "test-bucket"
	key := "final/image.webp"
	content := []byte("fake-webp-data")

	client := &mockS3Client{
		getObject: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			if *params.Bucket != bucket {
				t.Errorf("expected bucket %q, got %q", bucket, *params.Bucket)
			}
			if *params.Key != key {
				t.Errorf("expected key %q, got %q", key, *params.Key)
			}
			return &s3.GetObjectOutput{
				Body: io.NopCloser(bytes.NewReader(content)),
			}, nil
		},
	}

	provider := NewS3StorageProvider(client, &mockS3Presign{}, bucket)
	body, err := provider.GetObject(context.Background(), key)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer body.Close()

	res, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if !bytes.Equal(res, content) {
		t.Errorf("expected content %q, got %q", content, res)
	}
}

func TestS3StorageProvider_GetObject_Error(t *testing.T) {
	s3Err := errors.New("s3 download failed")
	client := &mockS3Client{
		getObject: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
			return nil, s3Err
		},
	}

	provider := NewS3StorageProvider(client, &mockS3Presign{}, "bucket")
	_, err := provider.GetObject(context.Background(), "key")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, s3Err) {
		t.Errorf("expected error containing %q, got %v", s3Err.Error(), err)
	}
}

func TestS3StorageProvider_PutObject(t *testing.T) {
	bucket := "test-bucket"
	key := "final/image.webp"
	content := []byte("processed-webp-data")
	contentType := "image/webp"

	client := &mockS3Client{
		putObject: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			if *params.Bucket != bucket {
				t.Errorf("expected bucket %q, got %q", bucket, *params.Bucket)
			}
			if *params.Key != key {
				t.Errorf("expected key %q, got %q", key, *params.Key)
			}
			if *params.ContentType != contentType {
				t.Errorf("expected content type %q, got %q", contentType, *params.ContentType)
			}
			bodyBytes, _ := io.ReadAll(params.Body)
			if !bytes.Equal(bodyBytes, content) {
				t.Errorf("expected body %q, got %q", content, bodyBytes)
			}
			return &s3.PutObjectOutput{}, nil
		},
	}

	provider := NewS3StorageProvider(client, &mockS3Presign{}, bucket)
	err := provider.PutObject(context.Background(), key, bytes.NewReader(content), contentType)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestS3StorageProvider_PutObject_Error(t *testing.T) {
	s3Err := errors.New("s3 upload failed")
	client := &mockS3Client{
		putObject: func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
			return nil, s3Err
		},
	}

	provider := NewS3StorageProvider(client, &mockS3Presign{}, "bucket")
	err := provider.PutObject(context.Background(), "key", bytes.NewReader([]byte("data")), "image/png")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, s3Err) {
		t.Errorf("expected error containing %q, got %v", s3Err.Error(), err)
	}
}

func TestS3StorageProvider_DeleteObject(t *testing.T) {
	bucket := "test-bucket"
	key := "quarantine/upload-id"

	client := &mockS3Client{
		deleteObject: func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
			if *params.Bucket != bucket {
				t.Errorf("expected bucket %q, got %q", bucket, *params.Bucket)
			}
			if *params.Key != key {
				t.Errorf("expected key %q, got %q", key, *params.Key)
			}
			return &s3.DeleteObjectOutput{}, nil
		},
	}

	provider := NewS3StorageProvider(client, &mockS3Presign{}, bucket)
	err := provider.DeleteObject(context.Background(), key)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestS3StorageProvider_DeleteObject_Error(t *testing.T) {
	s3Err := errors.New("s3 delete failed")
	client := &mockS3Client{
		deleteObject: func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
			return nil, s3Err
		},
	}

	provider := NewS3StorageProvider(client, &mockS3Presign{}, "bucket")
	err := provider.DeleteObject(context.Background(), "key")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, s3Err) {
		t.Errorf("expected error containing %q, got %v", s3Err.Error(), err)
	}
}

func TestS3StorageProvider_ObjectExists_True(t *testing.T) {
	bucket := "test-bucket"
	key := "quarantine/upload-id"

	client := &mockS3Client{
		headObject: func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
			if *params.Bucket != bucket {
				t.Errorf("expected bucket %q, got %q", bucket, *params.Bucket)
			}
			if *params.Key != key {
				t.Errorf("expected key %q, got %q", key, *params.Key)
			}
			return &s3.HeadObjectOutput{}, nil
		},
	}

	provider := NewS3StorageProvider(client, &mockS3Presign{}, bucket)
	exists, err := provider.ObjectExists(context.Background(), key)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected exists to be true, got false")
	}
}

func TestS3StorageProvider_ObjectExists_False(t *testing.T) {
	client := &mockS3Client{
		headObject: func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
			// S3 retorna 404 (NoSuchKey ou NotFound) para chaves inexistentes
			return nil, errors.New("NotFound: Not Found")
		},
	}

	provider := NewS3StorageProvider(client, &mockS3Presign{}, "bucket")
	exists, err := provider.ObjectExists(context.Background(), "missing-key")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected exists to be false, got true")
	}
}
