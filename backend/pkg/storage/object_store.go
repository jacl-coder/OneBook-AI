package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ObjectStore provides access to object storage.
type ObjectStore interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	PresignGet(ctx context.Context, key string, expiry time.Duration, filename string) (string, error)
	Delete(ctx context.Context, key string) error
}

// MinioStore implements ObjectStore for MinIO/S3 compatible storage.
type MinioStore struct {
	client *minio.Client
	bucket string
}

// NewMinioStore connects to MinIO and ensures the bucket exists.
func NewMinioStore(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinioStore, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("init minio client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket: %w", err)
		}
	}
	return &MinioStore{client: client, bucket: bucket}, nil
}

// Put uploads an object.
func (m *MinioStore) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := m.client.PutObject(ctx, m.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

// PresignGet generates a pre-signed GET URL.
func (m *MinioStore) PresignGet(ctx context.Context, key string, expiry time.Duration, filename string) (string, error) {
	reqParams := url.Values{}
	if strings.TrimSpace(filename) != "" {
		reqParams.Set("response-content-disposition", contentDisposition(filename))
	}
	url, err := m.client.PresignedGetObject(ctx, m.bucket, key, expiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}
	return url.String(), nil
}

// Delete removes an object.
func (m *MinioStore) Delete(ctx context.Context, key string) error {
	if err := m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func contentDisposition(filename string) string {
	fallback := sanitizeASCII(filename)
	if fallback == "" {
		fallback = "download"
	}
	encoded := encodeRFC5987(filename)
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", fallback, encoded)
}

func sanitizeASCII(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(name))
	lastUnderscore := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_' {
			b.WriteByte(c)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func encodeRFC5987(value string) string {
	if value == "" {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if isAttrChar(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteString(fmt.Sprintf("%%%02X", c))
	}
	return b.String()
}

func isAttrChar(c byte) bool {
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
		return true
	}
	switch c {
	case '!', '#', '$', '&', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	default:
		return false
	}
}
