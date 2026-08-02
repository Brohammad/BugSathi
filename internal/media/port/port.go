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
	MarkProcessing(ctx context.Context, id uuid.UUID, at time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, at time.Time) error
	// FinalizeReady inserts artifacts, sets READY, writes outbox in one transaction.
	FinalizeReady(ctx context.Context, id uuid.UUID, at time.Time, artifacts []domain.Artifact, topic, key string, payload []byte, correlationID string) error
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
