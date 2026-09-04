package frames_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Brohammad/BugSathi/internal/ai/adapter/frames"
	aiport "github.com/Brohammad/BugSathi/internal/ai/port"
	uploadport "github.com/Brohammad/BugSathi/internal/uploads/port"
)

type objectStore struct {
	meta      uploadport.ObjectMeta
	data      string
	statErr   error
	readErr   error
	downloads int
}

func (s *objectStore) Stat(context.Context, string) (uploadport.ObjectMeta, error) {
	return s.meta, s.statErr
}

func (s *objectStore) Download(_ context.Context, _ string, dst io.Writer) error {
	s.downloads++
	if s.readErr != nil {
		return s.readErr
	}
	_, err := io.WriteString(dst, s.data)
	return err
}

func TestReadFrameReturnsBoundedVisualInput(t *testing.T) {
	objects := &objectStore{
		meta: uploadport.ObjectMeta{Size: 4, ContentType: "image/JPEG; charset=binary"},
		data: "jpeg",
	}
	reader := frames.New(objects)

	frame, err := reader.ReadFrame(context.Background(), "frames/00000.jpg", 8)
	if err != nil {
		t.Fatal(err)
	}
	if frame.StorageKey != "frames/00000.jpg" || frame.MediaType != "image/jpeg" || string(frame.Data) != "jpeg" {
		t.Fatalf("frame=%+v", frame)
	}
	if objects.downloads != 1 {
		t.Fatalf("downloads=%d", objects.downloads)
	}
}

func TestReadFrameRejectsOversizedMetadataBeforeDownload(t *testing.T) {
	objects := &objectStore{
		meta: uploadport.ObjectMeta{Size: 9, ContentType: "image/jpeg"},
		data: "oversized",
	}
	_, err := frames.New(objects).ReadFrame(context.Background(), "frame.jpg", 8)
	if !errors.Is(err, frames.ErrFrameTooLarge) {
		t.Fatalf("err=%v", err)
	}
	if objects.downloads != 0 {
		t.Fatalf("downloads=%d want 0", objects.downloads)
	}
}

func TestReadFrameRejectsObjectGrowthDuringDownload(t *testing.T) {
	objects := &objectStore{
		meta: uploadport.ObjectMeta{Size: 4, ContentType: "image/jpeg"},
		data: strings.Repeat("x", 9),
	}
	_, err := frames.New(objects).ReadFrame(context.Background(), "frame.jpg", 8)
	if !errors.Is(err, frames.ErrFrameTooLarge) {
		t.Fatalf("err=%v", err)
	}
}

func TestReadFrameRejectsEmptyAndUnsupportedObjects(t *testing.T) {
	cases := []struct {
		name string
		meta uploadport.ObjectMeta
		want error
	}{
		{name: "empty", meta: uploadport.ObjectMeta{ContentType: "image/jpeg"}, want: frames.ErrEmptyFrame},
		{name: "unsupported", meta: uploadport.ObjectMeta{Size: 4, ContentType: "image/gif"}, want: frames.ErrUnsupportedMediaType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := frames.New(&objectStore{meta: tc.meta}).ReadFrame(context.Background(), "frame", 8)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want %v", err, tc.want)
			}
		})
	}
}

var _ aiport.FrameReader = (*frames.Reader)(nil)
var _ frames.ObjectStore = (*objectStore)(nil)
