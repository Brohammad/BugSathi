package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Brohammad/BugSathi/internal/media/domain"
	"github.com/Brohammad/BugSathi/internal/media/port"
	uploaddomain "github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/google/uuid"
)

type Service struct {
	store     port.RecordingStore
	objects   port.ObjectStore
	extractor port.FrameExtractor
	now       func() time.Time
	workdir   string
}

func New(store port.RecordingStore, objects port.ObjectStore, extractor port.FrameExtractor) *Service {
	return &Service{
		store:     store,
		objects:   objects,
		extractor: extractor,
		now:       time.Now,
		workdir:   os.TempDir(),
	}
}

func (s *Service) HandleUploaded(ctx context.Context, evt domain.RecordingUploadedEvent) error {
	id, err := uuid.Parse(evt.RecordingID)
	if err != nil {
		return fmt.Errorf("recording_id: %w", err)
	}
	rec, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}

	existing, err := s.store.ListArtifacts(ctx, id)
	if err != nil {
		return err
	}
	if rec.Status == uploaddomain.StatusReady && len(existing) > 0 {
		return s.publishFramesExtracted(ctx, rec, existing, 0, evt.CorrelationID)
	}

	if rec.Status != uploaddomain.StatusUploaded && rec.Status != uploaddomain.StatusProcessing && rec.Status != uploaddomain.StatusFailed {
		if rec.Status == uploaddomain.StatusReady {
			return nil
		}
		return fmt.Errorf("%w: status=%s", domain.ErrConflict, rec.Status)
	}

	now := s.now()
	if rec.Status == uploaddomain.StatusUploaded || rec.Status == uploaddomain.StatusFailed {
		if err := s.store.MarkProcessing(ctx, id, now); err != nil {
			return err
		}
	}

	jobDir, err := os.MkdirTemp(s.workdir, "bugsathi-media-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(jobDir)

	srcPath := filepath.Join(jobDir, "source"+extFromKey(evt.ObjectKey))
	if err := s.downloadToFile(ctx, evt.ObjectKey, srcPath); err != nil {
		_ = s.store.MarkFailed(ctx, id, s.now())
		return err
	}

	framesDir := filepath.Join(jobDir, "frames")
	if err := os.MkdirAll(framesDir, 0o755); err != nil {
		return err
	}
	result, err := s.extractor.Extract(ctx, srcPath, framesDir)
	if err != nil {
		_ = s.store.MarkFailed(ctx, id, s.now())
		return err
	}
	if len(result.FramePaths) == 0 {
		_ = s.store.MarkFailed(ctx, id, s.now())
		return fmt.Errorf("no frames extracted")
	}

	artifacts := make([]domain.Artifact, 0, len(result.FramePaths)+1)
	frameKeys := make([]string, 0, len(result.FramePaths))
	for i, fp := range result.FramePaths {
		key := fmt.Sprintf("projects/%s/recordings/%s/frames/%05d.jpg", evt.ProjectID, evt.RecordingID, i)
		size, err := s.uploadFile(ctx, key, "image/jpeg", fp)
		if err != nil {
			_ = s.store.MarkFailed(ctx, id, s.now())
			return err
		}
		sz := size
		artifacts = append(artifacts, domain.Artifact{
			ID: uuid.New(), RecordingID: id, Kind: domain.KindFrame,
			StorageKey: key, Ordinal: i, ContentType: "image/jpeg", ByteSize: &sz, CreatedAt: s.now(),
		})
		frameKeys = append(frameKeys, key)
	}

	thumbKey := ""
	if result.ThumbPath != "" {
		thumbKey = fmt.Sprintf("projects/%s/recordings/%s/thumb.jpg", evt.ProjectID, evt.RecordingID)
		size, err := s.uploadFile(ctx, thumbKey, "image/jpeg", result.ThumbPath)
		if err != nil {
			_ = s.store.MarkFailed(ctx, id, s.now())
			return err
		}
		sz := size
		artifacts = append(artifacts, domain.Artifact{
			ID: uuid.New(), RecordingID: id, Kind: domain.KindThumb,
			StorageKey: thumbKey, Ordinal: 0, ContentType: "image/jpeg", ByteSize: &sz, CreatedAt: s.now(),
		})
	} else if len(frameKeys) > 0 {
		thumbKey = frameKeys[0]
	}

	payload, err := json.Marshal(domain.FramesExtractedEvent{
		SchemaVersion: 1,
		RecordingID:   evt.RecordingID,
		ProjectID:     evt.ProjectID,
		FrameKeys:     frameKeys,
		ThumbKey:      thumbKey,
		DurationMS:    result.DurationMS,
		CorrelationID: firstNonEmpty(evt.CorrelationID, rec.CorrelationID),
		OccurredAt:    s.now().UTC(),
	})
	if err != nil {
		return err
	}

	return s.store.FinalizeReady(
		ctx, id, s.now(), artifacts,
		domain.TopicFramesExtracted, evt.RecordingID, payload,
		firstNonEmpty(evt.CorrelationID, rec.CorrelationID),
	)
}

func (s *Service) publishFramesExtracted(ctx context.Context, rec uploaddomain.Recording, artifacts []domain.Artifact, durationMS int64, corr string) error {
	frameKeys := make([]string, 0)
	thumbKey := ""
	for _, a := range artifacts {
		if a.Kind == domain.KindFrame {
			frameKeys = append(frameKeys, a.StorageKey)
		}
		if a.Kind == domain.KindThumb {
			thumbKey = a.StorageKey
		}
	}
	if thumbKey == "" && len(frameKeys) > 0 {
		thumbKey = frameKeys[0]
	}
	payload, err := json.Marshal(domain.FramesExtractedEvent{
		SchemaVersion: 1,
		RecordingID:   rec.ID.String(),
		ProjectID:     rec.ProjectID.String(),
		FrameKeys:     frameKeys,
		ThumbKey:      thumbKey,
		DurationMS:    durationMS,
		CorrelationID: firstNonEmpty(corr, rec.CorrelationID),
		OccurredAt:    s.now().UTC(),
	})
	if err != nil {
		return err
	}
	return s.store.FinalizeReady(ctx, rec.ID, s.now(), nil, domain.TopicFramesExtracted, rec.ID.String(), payload, firstNonEmpty(corr, rec.CorrelationID))
}

func (s *Service) downloadToFile(ctx context.Context, key, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return s.objects.Download(ctx, key, f)
}

func (s *Service) uploadFile(ctx context.Context, key, contentType, path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if err := s.objects.Upload(ctx, key, contentType, f, st.Size()); err != nil {
		return 0, err
	}
	return st.Size(), nil
}

func extFromKey(key string) string {
	ext := filepath.Ext(key)
	if ext == "" {
		return ".bin"
	}
	return ext
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
