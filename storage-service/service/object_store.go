// Package service holds storage-service's clients of other systems: the S3
// compatible object store and the Users service.
package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ObjectStore keeps the CONTENT of uploaded files in an S3 compatible object
// store (the docker-compose service "minio"); only metadata goes to MongoDB.
type ObjectStore struct {
	client *minio.Client
	bucket string
}

// NewObjectStore connects to the store and makes sure the bucket exists. The
// store may still be starting when the service boots, so it retries for ~20s.
func NewObjectStore(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*ObjectStore, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create object store client: %w", err)
	}

	ctx := context.Background()
	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		exists, err := client.BucketExists(ctx, bucket)
		if err == nil && !exists {
			err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		}
		if err == nil {
			return &ObjectStore{client: client, bucket: bucket}, nil
		}

		lastErr = fmt.Errorf("waiting for object store (attempt %d/10): %w", attempt, err)
		log.Println(lastErr)
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

// Put stores exactly size bytes read from r under key.
func (s *ObjectStore) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// Delete removes an object; removing a key that does not exist is not an error.
func (s *ObjectStore) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
