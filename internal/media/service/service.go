package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Brohammad/BugSathi/internal/media/domain"
	"github.com/Brohammad/BugSathi/internal/media/port"
	uploaddomain "github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/google/uuid"
)

// ClaimConfig describes the processing lease this worker takes before running
// ffmpeg. The lease is what stops a redelivered or reprocessed event from
// extracting the same recording twice, while still letting another worker pick
// the recording up if the holder dies.
type ClaimConfig struct {
	Owner         string        // identity stored on the recording
	Lease         time.Duration // how long a claim survives without renewal
	RenewInterval time.Duration // how often the holder extends its lease
}

const (
	defaultLease         = 2 * time.Minute
	defaultRenewInterval = 30 * time.Second
)

func (c ClaimConfig) withDefaults() ClaimConfig {
	if c.Owner == "" {
		host, err := os.Hostname()
		if err != nil {
			host = "worker"
		}
		c.Owner = fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	if c.Lease <= 0 {
		c.Lease = defaultLease
	}
	if c.RenewInterval <= 0 {
		c.RenewInterval = defaultRenewInterval
	}
	// Renewing at (or after) expiry would let the lease lapse mid-job.
	if c.RenewInterval >= c.Lease {
		c.RenewInterval = c.Lease / 2
	}
	return c
}

type Service struct {
	store     port.RecordingStore
	objects   port.ObjectStore
	extractor port.FrameExtractor
	now       func() time.Time
	workdir   string
	claim     ClaimConfig
}

func New(store port.RecordingStore, objects port.ObjectStore, extractor port.FrameExtractor, claim ClaimConfig) *Service {
	return &Service{
		store:     store,
		objects:   objects,
		extractor: extractor,
		now:       time.Now,
		workdir:   os.TempDir(),
		claim:     claim.withDefaults(),
	}
}

// Owner is the identity this worker writes into processing claims.
func (s *Service) Owner() string { return s.claim.Owner }

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
	if rec.Status == uploaddomain.StatusReady {
		if len(existing) > 0 {
			return s.publishFramesExtracted(ctx, rec, existing, 0, evt.CorrelationID)
		}
		return nil
	}

	if rec.Status != uploaddomain.StatusUploaded && rec.Status != uploaddomain.StatusProcessing && rec.Status != uploaddomain.StatusFailed {
		return fmt.Errorf("%w: status=%s", domain.ErrConflict, rec.Status)
	}

	now := s.now()
	if err := s.store.ClaimProcessing(ctx, id, s.claim.Owner, now, now.Add(s.claim.Lease)); err != nil {
		return err
	}

	// The lease can outlive us (crash) or we can outlive the lease (very long
	// ffmpeg run), so hold it with a heartbeat and abandon the job the moment
	// someone else takes over.
	jobCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	stopRenewal := s.renewClaimUntilDone(jobCtx, cancel, id)
	defer stopRenewal()

	if err := s.extract(jobCtx, id, rec, evt); err != nil {
		if cause := context.Cause(jobCtx); domain.IsClaimConflict(cause) {
			return cause
		}
		return err
	}
	return nil
}

func (s *Service) extract(ctx context.Context, id uuid.UUID, rec uploaddomain.Recording, evt domain.RecordingUploadedEvent) error {
	jobDir, err := os.MkdirTemp(s.workdir, "bugsathi-media-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(jobDir)

	srcPath := filepath.Join(jobDir, "source"+extFromKey(evt.ObjectKey))
	if err := s.downloadToFile(ctx, evt.ObjectKey, srcPath); err != nil {
		s.fail(ctx, id)
		return err
	}

	framesDir := filepath.Join(jobDir, "frames")
	if err := os.MkdirAll(framesDir, 0o755); err != nil {
		return err
	}
	result, err := s.extractor.Extract(ctx, srcPath, framesDir)
	if err != nil {
		s.fail(ctx, id)
		return err
	}
	if len(result.FramePaths) == 0 {
		s.fail(ctx, id)
		return fmt.Errorf("no frames extracted")
	}

	artifacts := make([]domain.Artifact, 0, len(result.FramePaths)+1)
	frameKeys := make([]string, 0, len(result.FramePaths))
	for i, fp := range result.FramePaths {
		key := fmt.Sprintf("projects/%s/recordings/%s/frames/%05d.jpg", evt.ProjectID, evt.RecordingID, i)
		size, err := s.uploadFile(ctx, key, "image/jpeg", fp)
		if err != nil {
			s.fail(ctx, id)
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
			s.fail(ctx, id)
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
		ctx, id, s.claim.Owner, s.now(), artifacts,
		domain.TopicFramesExtracted, evt.RecordingID, payload,
		firstNonEmpty(evt.CorrelationID, rec.CorrelationID),
	)
}

// renewClaimUntilDone extends our lease in the background and cancels the job
// once the recording belongs to someone else. Transient renewal errors are
// tolerated: the lease is still valid until it expires, and FinalizeReady is
// owner-gated, so nothing can be committed under a stolen claim.
func (s *Service) renewClaimUntilDone(ctx context.Context, cancel context.CancelCauseFunc, id uuid.UUID) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(s.claim.RenewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := s.store.RenewClaim(ctx, id, s.claim.Owner, s.now().Add(s.claim.Lease))
				if errors.Is(err, domain.ErrClaimLost) {
					cancel(err)
					return
				}
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

// fail releases the claim and marks the recording FAILED so a retry can pick it
// up immediately. It is a no-op when the claim already changed hands, and it
// deliberately outlives a canceled job context so shutdown still releases.
func (s *Service) fail(ctx context.Context, id uuid.UUID) {
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = s.store.MarkFailed(releaseCtx, id, s.claim.Owner, s.now())
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
	// The recording is already READY with artifacts on disk; only the event
	// needs to exist again, so no claim is involved.
	return s.store.InsertOutbox(ctx, domain.TopicFramesExtracted, rec.ID.String(), payload, firstNonEmpty(corr, rec.CorrelationID))
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
