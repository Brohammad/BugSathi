package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/Brohammad/BugSathi/internal/uploads/port"
	miniosdk "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	client *miniosdk.Client
	signer *miniosdk.Client
	bucket string
}

func New(cfg config.MinIOConfig) (*Storage, error) {
	creds := credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")
	client, err := miniosdk.New(cfg.Endpoint, &miniosdk.Options{
		Creds:  creds,
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	signer := client
	if ep, useSSL := cfg.PresignEndpoint(); ep != cfg.Endpoint || useSSL != cfg.UseSSL {
		signer, err = miniosdk.New(ep, &miniosdk.Options{
			Creds:  creds,
			Secure: useSSL,
		})
		if err != nil {
			return nil, fmt.Errorf("minio presign client: %w", err)
		}
	}
	return &Storage{client: client, signer: signer, bucket: cfg.Bucket}, nil
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
	headers := http.Header{}
	if ct := strings.TrimSpace(contentType); ct != "" {
		headers.Set("Content-Type", ct)
	}
	u, err := s.signer.PresignHeader(ctx, http.MethodPut, s.bucket, key, expiry, nil, headers)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *Storage) Stat(ctx context.Context, key string) (port.ObjectMeta, error) {
	info, err := s.client.StatObject(ctx, s.bucket, key, miniosdk.StatObjectOptions{})
	if err != nil {
		errResp := miniosdk.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" || errResp.StatusCode == 404 {
			return port.ObjectMeta{}, domain.ErrObjectMissing
		}
		return port.ObjectMeta{}, err
	}
	return port.ObjectMeta{
		Size:        info.Size,
		ContentType: info.ContentType,
		ETag:        info.ETag,
	}, nil
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
	u, err := s.signer.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
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
