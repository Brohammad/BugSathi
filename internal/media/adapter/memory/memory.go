package memory

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Brohammad/BugSathi/internal/media/domain"
	"github.com/Brohammad/BugSathi/internal/media/port"
	uploaddomain "github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/google/uuid"
)

type Store struct {
	mu        sync.Mutex
	recs      map[uuid.UUID]uploaddomain.Recording
	claims    map[uuid.UUID]Claim
	artifacts map[uuid.UUID][]domain.Artifact
	outbox    []OutboxRow
}

// Claim mirrors the recordings.processing_owner / processing_expires_at pair.
type Claim struct {
	Owner     string
	ExpiresAt time.Time
}

type OutboxRow struct {
	Topic, Key, Corr string
	Payload          []byte
}

func NewStore() *Store {
	return &Store{
		recs:      map[uuid.UUID]uploaddomain.Recording{},
		claims:    map[uuid.UUID]Claim{},
		artifacts: map[uuid.UUID][]domain.Artifact{},
	}
}

func (s *Store) Seed(rec uploaddomain.Recording) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs[rec.ID] = rec
}

// SeedClaim installs a lease so tests can simulate another worker.
func (s *Store) SeedClaim(id uuid.UUID, owner string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims[id] = Claim{Owner: owner, ExpiresAt: expiresAt}
}

func (s *Store) Claim(id uuid.UUID) (Claim, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.claims[id]
	return c, ok
}

func (s *Store) Outbox() []OutboxRow {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]OutboxRow(nil), s.outbox...)
}

func (s *Store) Status(id uuid.UUID) uploaddomain.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recs[id].Status
}

func (s *Store) Get(_ context.Context, id uuid.UUID) (uploaddomain.Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.recs[id]
	if !ok {
		return uploaddomain.Recording{}, domain.ErrNotFound
	}
	return r, nil
}

func (s *Store) ClaimProcessing(_ context.Context, id uuid.UUID, owner string, at, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.recs[id]
	if !ok {
		return domain.ErrNotFound
	}
	switch r.Status {
	case uploaddomain.StatusUploaded, uploaddomain.StatusFailed:
	case uploaddomain.StatusProcessing:
		if c, held := s.claims[id]; held && c.Owner != owner && c.ExpiresAt.After(at) {
			return domain.ErrClaimHeld
		}
	default:
		return domain.ErrConflict
	}
	r.Status = uploaddomain.StatusProcessing
	r.UpdatedAt = at
	s.recs[id] = r
	s.claims[id] = Claim{Owner: owner, ExpiresAt: expiresAt}
	return nil
}

func (s *Store) RenewClaim(_ context.Context, id uuid.UUID, owner string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.claims[id]
	if !ok || c.Owner != owner {
		return domain.ErrClaimLost
	}
	s.claims[id] = Claim{Owner: owner, ExpiresAt: expiresAt}
	return nil
}

func (s *Store) MarkFailed(_ context.Context, id uuid.UUID, owner string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.recs[id]
	if !ok {
		return domain.ErrNotFound
	}
	if c, held := s.claims[id]; !held || c.Owner != owner {
		return domain.ErrClaimLost
	}
	r.Status = uploaddomain.StatusFailed
	r.UpdatedAt = at
	s.recs[id] = r
	delete(s.claims, id)
	return nil
}

func (s *Store) FinalizeReady(_ context.Context, id uuid.UUID, owner string, at time.Time, artifacts []domain.Artifact, topic, key string, payload []byte, corr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.recs[id]
	if !ok {
		return domain.ErrNotFound
	}
	if c, held := s.claims[id]; !held || c.Owner != owner {
		return domain.ErrClaimLost
	}
	r.Status = uploaddomain.StatusReady
	r.UpdatedAt = at
	s.recs[id] = r
	delete(s.claims, id)
	if len(artifacts) > 0 {
		s.artifacts[id] = append([]domain.Artifact(nil), artifacts...)
	}
	s.outbox = append(s.outbox, OutboxRow{Topic: topic, Key: key, Payload: payload, Corr: corr})
	return nil
}

func (s *Store) InsertOutbox(_ context.Context, topic, key string, payload []byte, corr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outbox = append(s.outbox, OutboxRow{Topic: topic, Key: key, Payload: payload, Corr: corr})
	return nil
}

func (s *Store) ListArtifacts(_ context.Context, recordingID uuid.UUID) ([]domain.Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.Artifact(nil), s.artifacts[recordingID]...), nil
}

type Objects struct {
	mu   sync.Mutex
	data map[string][]byte
}

func NewObjects() *Objects { return &Objects{data: map[string][]byte{}} }

func (o *Objects) Put(key string, b []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.data[key] = append([]byte(nil), b...)
}

func (o *Objects) Download(_ context.Context, key string, w io.Writer) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	b, ok := o.data[key]
	if !ok {
		return domain.ErrNotFound
	}
	_, err := w.Write(b)
	return err
}

func (o *Objects) Upload(_ context.Context, key, _ string, r io.Reader, _ int64) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.data[key] = b
	return nil
}

type FakeExtractor struct{}

func (FakeExtractor) Extract(_ context.Context, _, outputDir string) (port.Result, error) {
	f1 := filepath.Join(outputDir, "00001.jpg")
	f2 := filepath.Join(outputDir, "00002.jpg")
	thumb := filepath.Join(outputDir, "thumb.jpg")
	_ = os.WriteFile(f1, []byte("jpg1"), 0o644)
	_ = os.WriteFile(f2, []byte("jpg2"), 0o644)
	_ = os.WriteFile(thumb, []byte("thumb"), 0o644)
	return port.Result{FramePaths: []string{f1, f2}, ThumbPath: thumb, DurationMS: 2000}, nil
}
