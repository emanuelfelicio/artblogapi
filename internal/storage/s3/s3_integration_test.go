package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/emanuelfelicio/artblogapi/internal/storage"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	integrationBucket = "storage-integration"
	minioAccessKey    = "minioadmin"
	minioSecretKey    = "minioadmin"
)

func TestS3StorageProvider_PresignedUploadWithMinIO(t *testing.T) {

	ctx := context.Background()
	container, endpoint := startMinIO(t, ctx)
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Errorf("terminate MinIO container: %v", err)
		}
	}()

	client, err := InitS3Client(ctx, endpoint, "us-east-1", minioAccessKey, minioSecretKey, true)
	if err != nil {
		t.Fatalf("init S3 client: %v", err)
	}
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(integrationBucket),
	}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	provider := NewS3StorageProvider(client, s3.NewPresignClient(client), integrationBucket)
	httpClient := &http.Client{Timeout: 10 * time.Second}

	tests := []struct {
		name         string
		expectedSize int
		payloadSize  int
		contentType  string
		wantStatusOK bool
	}{
		{
			name:         "accepts exact size and content type",
			expectedSize: 1024,
			payloadSize:  1024,
			contentType:  string(storage.ContentTypePNG),
			wantStatusOK: true,
		},
		{
			name:         "rejects smaller payload",
			expectedSize: 1024,
			payloadSize:  1023,
			contentType:  string(storage.ContentTypePNG),
		},
		{
			name:         "rejects larger payload",
			expectedSize: 1024,
			payloadSize:  1025,
			contentType:  string(storage.ContentTypePNG),
		},
		{
			name:         "rejects different content type",
			expectedSize: 1024,
			payloadSize:  1024,
			contentType:  "image/jpeg",
		},
		{
			name:         "rejects payload larger than signed size",
			expectedSize: 1024,
			payloadSize:  2048,
			contentType:  string(storage.ContentTypePNG),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := fmt.Sprintf("quarantine/%s", tc.name)
			presignedURL, err := provider.GenerateUploadURL(
				ctx,
				key,
				storage.ContentTypePNG,
				5*time.Minute,
				tc.expectedSize,
			)
			if err != nil {
				t.Fatalf("generate presigned URL: %v", err)
			}

			payload := bytes.Repeat([]byte("x"), tc.payloadSize)
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignedURL, bytes.NewReader(payload))
			if err != nil {
				t.Fatalf("create upload request: %v", err)
			}
			req.Header.Set("Content-Type", tc.contentType)
			resp, err := httpClient.Do(req)
			if err != nil {
				t.Fatalf("execute upload request: %v", err)
			}
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)

			gotSuccess := resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
			if gotSuccess != tc.wantStatusOK {
				t.Fatalf("upload status = %d, want success = %t", resp.StatusCode, tc.wantStatusOK)
			}

			if !tc.wantStatusOK {
				return
			}

			head, err := client.HeadObject(ctx, &s3.HeadObjectInput{
				Bucket: aws.String(integrationBucket),
				Key:    aws.String(key),
			})
			if err != nil {
				t.Fatalf("head uploaded object: %v", err)
			}
			if head.ContentLength == nil || *head.ContentLength != int64(tc.expectedSize) {
				t.Fatalf("stored content length = %v, want %d", head.ContentLength, tc.expectedSize)
			}
			if head.ContentType == nil || *head.ContentType != tc.contentType {
				t.Fatalf("stored content type = %v, want %q", head.ContentType, tc.contentType)
			}
		})
	}
}

func startMinIO(t *testing.T, ctx context.Context) (testcontainers.Container, string) {
	t.Helper()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "minio/minio:latest",
			Env:          map[string]string{"MINIO_ROOT_USER": minioAccessKey, "MINIO_ROOT_PASSWORD": minioSecretKey},
			Cmd:          []string{"server", "/data"},
			ExposedPorts: []string{"9000/tcp"},
			WaitingFor: wait.ForHTTP("/minio/health/ready").
				WithPort("9000/tcp").
				WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start MinIO container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("get MinIO host: %v", err)
	}
	port, err := container.MappedPort(ctx, "9000/tcp")
	if err != nil {
		t.Fatalf("get MinIO port: %v", err)
	}
	return container, fmt.Sprintf("http://%s:%s", host, port.Port())
}
