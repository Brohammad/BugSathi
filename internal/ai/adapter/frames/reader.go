package frames

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Brohammad/BugSathi/internal/ai/domain"
	uploadport "github.com/Brohammad/BugSathi/internal/uploads/port"
)

var (
	ErrEmptyFrame           = errors.New("frame is empty")
	ErrFrameTooLarge        = errors.New("frame exceeds byte limit")
	ErrUnsupportedMediaType = errors.New("unsupported frame media type")
)

type ObjectStore interface {
	Stat(ctx context.Context, key string) (uploadport.ObjectMeta, error)
	Download(ctx context.Context, key string, dst io.Writer) error
}

type Reader struct {
	objects ObjectStore
}

func New(objects ObjectStore) *Reader {
	return &Reader{objects: objects}
}

func (r *Reader) ReadFrame(ctx context.Context, key string, maxBytes int64) (domain.FrameInput, error) {
	if r.objects == nil {
		return domain.FrameInput{}, fmt.Errorf("frame object store is required")
	}
	if maxBytes <= 0 {
		return domain.FrameInput{}, fmt.Errorf("frame byte limit must be positive")
	}

	meta, err := r.objects.Stat(ctx, key)
	if err != nil {
		return domain.FrameInput{}, fmt.Errorf("stat frame: %w", err)
	}
	if meta.Size <= 0 {
		return domain.FrameInput{}, ErrEmptyFrame
	}
	if meta.Size > maxBytes {
		return domain.FrameInput{}, fmt.Errorf("%w: size=%d limit=%d", ErrFrameTooLarge, meta.Size, maxBytes)
	}

	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(meta.ContentType, ";", 2)[0]))
	switch mediaType {
	case "image/jpeg", "image/png", "image/webp":
	default:
		return domain.FrameInput{}, fmt.Errorf("%w: %q", ErrUnsupportedMediaType, meta.ContentType)
	}

	dst := &boundedBuffer{max: maxBytes}
	if err := r.objects.Download(ctx, key, dst); err != nil {
		return domain.FrameInput{}, fmt.Errorf("download frame: %w", err)
	}
	if dst.buf.Len() == 0 {
		return domain.FrameInput{}, ErrEmptyFrame
	}

	return domain.FrameInput{
		StorageKey: key,
		MediaType:  mediaType,
		Data:       bytes.Clone(dst.buf.Bytes()),
	}, nil
}

type boundedBuffer struct {
	buf bytes.Buffer
	max int64
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if int64(b.buf.Len())+int64(len(p)) > b.max {
		return 0, ErrFrameTooLarge
	}
	return b.buf.Write(p)
}
