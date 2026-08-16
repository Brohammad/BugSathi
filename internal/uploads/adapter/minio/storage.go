package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/uploads/domain"
	miniosdk "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	client *miniosdk.Client
	bucket string
}

func New(cfg config.MinIOConfig) (*Storage, error) {
	client, err := miniosdk.New(cfg.Endpoint, &miniosdk.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	return &Storage{client: client, bucket: cfg.Bucket}, nil
}

func (s *Storage) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, miniosdk.MakeBucketOptions{})
}

func (s *Storage) PresignPut(ctx context.Context, key, contentType string, expiry time.Duration) (string, error) {
	_ = contentType // clients should send Content-Type; URL is method+key scoped
	u, err := s.client.PresignedPutObject(ctx, s.bucket, key, expiry)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *Storage) Stat(ctx context.Context, key string) (int64, string, error) {
	info, err := s.client.StatObject(ctx, s.bucket, key, miniosdk.StatObjectOptions{})
	if err != nil {
		errResp := miniosdk.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" || errResp.StatusCode == 404 {
			return 0, "", domain.ErrObjectMissing
		}
		return 0, "", err
	}
	return info.Size, info.ContentType, nil
}

// Put is used by tests / tooling (API path uses presign).
func (s *Storage) Put(ctx context.Context, key, contentType string, body []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(body), int64(len(body)), miniosdk.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *Storage) Download(ctx context.Context, key string, w io.Writer) error {
	obj, err := s.client.GetObject(ctx, s.bucket, key, miniosdk.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer obj.Close()
	_, err = io.Copy(w, obj)
	return err
}

func (s *Storage) Upload(ctx context.Context, key, contentType string, r io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, miniosdk.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (s *Storage) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// Delete removes one object. Missing keys are treated as success so cleanup is
// safe to retry after a partial failure.
func (s *Storage) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, miniosdk.RemoveObjectOptions{})
	if err == nil {
		return nil
	}
	errResp := miniosdk.ToErrorResponse(err)
	if errResp.Code == "NoSuchKey" || errResp.StatusCode == 404 {
		return nil
	}
	return err
}

// DeletePrefix removes every object under prefix (recursive). Used after a
// project is deleted so source/frames/thumb all go with it.
func (s *Storage) DeletePrefix(ctx context.Context, prefix string) error {
	if prefix == "" {
		return fmt.Errorf("refusing to delete with empty prefix")
	}
	opts := miniosdk.ListObjectsOptions{Prefix: prefix, Recursive: true}
	var firstErr error
	for obj := range s.client.ListObjects(ctx, s.bucket, opts) {
		if obj.Err != nil {
			return obj.Err
		}
		if err := s.Delete(ctx, obj.Key); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
