package port

import (
	"context"
	"io"
	"time"

	"github.com/Brohammad/BugSathi/internal/media/domain"
	uploaddomain "github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/Brohammad/BugSathi/internal/uploads/port"
	"github.com/google/uuid"
)

type RecordingStore interface {
	Get(ctx context.Context, id uuid.UUID) (uploaddomain.Recording, error)
	// ClaimProcessing sets status PROCESSING and takes an expiring lease for
	// owner. It returns domain.ErrClaimHeld when a different owner holds a
	// lease that has not expired at `at`.
	ClaimProcessing(ctx context.Context, id uuid.UUID, owner string, at, expiresAt time.Time) error
	// RenewClaim extends the lease of a claim we still own, and returns
	// domain.ErrClaimLost otherwise.
	RenewClaim(ctx context.Context, id uuid.UUID, owner string, expiresAt time.Time) error
	// MarkFailed sets FAILED and releases the claim, but only if owner still
	// holds it; otherwise it returns domain.ErrClaimLost and changes nothing.
	MarkFailed(ctx context.Context, id uuid.UUID, owner string, at time.Time) error
	// FinalizeReady inserts artifacts, sets READY, releases the claim, and
	// writes outbox in one transaction. It returns domain.ErrClaimLost if the
	// lease changed hands while the frames were being extracted.
	FinalizeReady(ctx context.Context, id uuid.UUID, owner string, at time.Time, artifacts []domain.Artifact, topic, key string, payload []byte, correlationID string) error
	// InsertOutbox re-publishes an event for a recording that is already READY.
	InsertOutbox(ctx context.Context, topic, key string, payload []byte, correlationID string) error
	ListArtifacts(ctx context.Context, recordingID uuid.UUID) ([]domain.Artifact, error)
}

type ObjectStore interface {
	Download(ctx context.Context, key string, w io.Writer) error
	Upload(ctx context.Context, key, contentType string, r io.Reader, size int64) error
}

type FrameExtractor interface {
	Extract(ctx context.Context, inputPath, outputDir string) (Result, error)
}

type Result struct {
	FramePaths []string
	ThumbPath  string
	DurationMS int64
}

type OutboxRepository = port.OutboxRepository
type EventPublisher = port.EventPublisher
