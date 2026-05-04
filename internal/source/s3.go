package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// S3ClientAPI defines the subset of s3.Client methods used by S3Source.
type S3ClientAPI interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// S3Source fetches the canonical config from an s3 bucket.
type S3Source struct {
	bucket string
	key    string
	client S3ClientAPI

	mu         sync.RWMutex
	cachedData []byte
	etag       string
}

// NewS3Source creates a new s3 source.
func NewS3Source(ctx context.Context, bucket, key, region string) (*S3Source, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	return &S3Source{
		bucket: bucket,
		key:    key,
		client: s3.NewFromConfig(cfg),
	}, nil
}

// Fetch reads the canonical config from an s3 bucket and returns the bytes.
// It uses ETag-based caching to avoid redundant data transfers.
func (s *S3Source) Fetch(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	etag := s.etag
	cached := s.cachedData
	s.mu.RUnlock()

	input := &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &s.key,
	}
	if etag != "" {
		input.IfNoneMatch = &etag
	}

	out, err := s.client.GetObject(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotModified" {
			// object hasn't changed, returning cached data
			res := make([]byte, len(cached))
			copy(res, cached)
			return res, nil
		}
		return nil, fmt.Errorf("s3 get object (bucket=%s key=%s): %w", s.bucket, s.key, err)
	}
	defer func() { _ = out.Body.Close() }()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("read s3 object body: %w", err)
	}

	s.mu.Lock()
	s.cachedData = data
	if out.ETag != nil {
		s.etag = *out.ETag
	}
	s.mu.Unlock()

	// avoid external mutation of cache
	res := make([]byte, len(data))
	copy(res, data)
	return res, nil
}

// String returns a human-readable identifier for the s3 source.
func (s *S3Source) String() string {
	return fmt.Sprintf("s3::%s/%s", s.bucket, s.key)
}
