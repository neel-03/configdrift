package source

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type mockS3Client struct {
	GetObjectFunc func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return m.GetObjectFunc(ctx, params, optFns...)
}

func TestNewS3Source(t *testing.T) {
	src, err := NewS3Source(context.Background(), "bucket", "key", "us-east-1")
	if err != nil {
		t.Fatalf("NewS3Source failed: %v", err)
	}
	if src == nil {
		t.Fatal("NewS3Source returned nil")
	}
}

func TestS3Source_String(t *testing.T) {
	src := &S3Source{bucket: "b", key: "k"}
	if src.String() != "s3::b/k" {
		t.Errorf("expected s3::b/k, got %s", src.String())
	}
}

func TestS3Source_Fetch(t *testing.T) {
	bucket := "test-bucket"
	key := "config.yaml"
	content := "key: value"
	etag := "v1"

	t.Run("Successful Fetch", func(t *testing.T) {
		mock := &mockS3Client{
			GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte(content))),
					ETag: &etag,
				}, nil
			},
		}

		s := &S3Source{
			bucket: bucket,
			key:    key,
			client: mock,
		}

		data, err := s.Fetch(context.Background())
		if err != nil {
			t.Fatalf("Fetch failed: %v", err)
		}

		if string(data) != content {
			t.Errorf("Expected %q, got %q", content, string(data))
		}

		if s.etag != etag {
			t.Errorf("Expected etag %q, got %q", etag, s.etag)
		}
	})

	t.Run("Cached Fetch (Not Modified)", func(t *testing.T) {
		callCount := 0
		mock := &mockS3Client{
			GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				callCount++
				if params.IfNoneMatch != nil && *params.IfNoneMatch == etag {
					return nil, &smithy.GenericAPIError{
						Code:    "NotModified",
						Message: "Object not modified",
					}
				}
				return &s3.GetObjectOutput{
					Body: io.NopCloser(bytes.NewReader([]byte(content))),
					ETag: &etag,
				}, nil
			},
		}

		s := &S3Source{
			bucket: bucket,
			key:    key,
			client: mock,
		}

		// First call - fills cache
		_, _ = s.Fetch(context.Background())

		// Second call - should return cached
		data, err := s.Fetch(context.Background())
		if err != nil {
			t.Fatalf("Second fetch failed: %v", err)
		}

		if string(data) != content {
			t.Errorf("Expected %q, got %q", content, string(data))
		}

		if callCount != 2 {
			t.Errorf("Expected 2 calls to S3, got %d", callCount)
		}
	})

	t.Run("S3 Error", func(t *testing.T) {
		mock := &mockS3Client{
			GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return nil, fmt.Errorf("s3 error")
			},
		}

		s := &S3Source{
			bucket: bucket,
			key:    key,
			client: mock,
		}

		_, err := s.Fetch(context.Background())
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})
	t.Run("Body Read Error", func(t *testing.T) {
		mock := &mockS3Client{
			GetObjectFunc: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(&errorReader{err: fmt.Errorf("read error")}),
				}, nil
			},
		}

		s := &S3Source{
			bucket: bucket,
			key:    key,
			client: mock,
		}

		_, err := s.Fetch(context.Background())
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
