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

type ObjectStore struct {
	client *minio.Client
	bucket string
}

func NewObjectStore(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*ObjectStore, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	ctx := context.Background()
	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		exists, err := client.BucketExists(ctx, bucket)
		if err == nil {
			if !exists {
				if mkErr := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); mkErr != nil {
					lastErr = fmt.Errorf("failed to create minio bucket: %w", mkErr)
					time.Sleep(2 * time.Second)
					continue
				}
			}
			return &ObjectStore{
				client: client,
				bucket: bucket,
			}, nil
		}

		lastErr = fmt.Errorf("waiting for object store (attempt %d/10): %w", attempt, err)
		log.Println(lastErr)
		time.Sleep(2 * time.Second)
	}

	return nil, lastErr
}

func (s *ObjectStore) UploadObject(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *ObjectStore) DeleteObject(ctx context.Context, objectKey string) error {
	return s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
}