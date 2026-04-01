package storage

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/qiwang/book-e-commerce-micro/common/config"
)

type MinIOClient struct {
	client   *minio.Client
	bucket   string
	endpoint string
	useSSL   bool
}

func NewMinIOClient(cfg config.MinIOConfig) (*MinIOClient, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket: %w", err)
		}
		log.Printf("[MinIO] created bucket %q", cfg.Bucket)
	}

	// Allow anonymous read so uploaded files are publicly accessible via URL.
	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"AWS": ["*"]},
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::%s/*"]
		}]
	}`, cfg.Bucket)
	if err := client.SetBucketPolicy(ctx, cfg.Bucket, policy); err != nil {
		log.Printf("[MinIO] warning: failed to set public-read policy: %v", err)
	}

	return &MinIOClient{
		client:   client,
		bucket:   cfg.Bucket,
		endpoint: cfg.Endpoint,
		useSSL:   cfg.UseSSL,
	}, nil
}

// Upload stores a file in MinIO and returns its publicly accessible URL.
func (m *MinIOClient) Upload(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) (string, error) {
	_, err := m.client.PutObject(ctx, m.bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}
	return m.ObjectURL(objectName), nil
}

// Delete removes a file from MinIO.
func (m *MinIOClient) Delete(ctx context.Context, objectName string) error {
	return m.client.RemoveObject(ctx, m.bucket, objectName, minio.RemoveObjectOptions{})
}

// ObjectURL returns the public URL for an object.
func (m *MinIOClient) ObjectURL(objectName string) string {
	scheme := "http"
	if m.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, m.endpoint, m.bucket, objectName)
}
