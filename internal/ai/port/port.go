package port

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Brohammad/BugSathi/internal/ai/domain"
	"github.com/google/uuid"
)

type Analyzer interface {
	Analyze(ctx context.Context, in domain.AnalysisInput) (domain.AnalysisResult, error)
}

type Store interface {
	GetAnalysis(ctx context.Context, recordingID uuid.UUID, promptVersion string) (domain.Analysis, error)
	// TryClaimRunning atomically marks the analysis running when it is new,
	// failed, or a stale in-flight lease (updated_at before leaseCutoff).
	// Returns domain.ErrAnalysisInFlight when another worker holds a fresh lease,
	// and domain.ErrNotFound only when the row is already completed (caller should
	// treat completed via GetAnalysis).
	TryClaimRunning(ctx context.Context, recordingID, projectID uuid.UUID, promptVersion string, at, leaseCutoff time.Time) (domain.Analysis, error)
	TouchRunning(ctx context.Context, recordingID uuid.UUID, promptVersion string, at time.Time) error
	CompleteAnalysis(ctx context.Context, analysis domain.Analysis, report domain.Report, events []OutboxEvent, at time.Time) error
	FailAnalysis(ctx context.Context, recordingID uuid.UUID, promptVersion, msg string, at time.Time) error
	GetReportByRecording(ctx context.Context, recordingID uuid.UUID) (domain.Report, error)
	GetRecordingMeta(ctx context.Context, recordingID uuid.UUID) (projectID uuid.UUID, metadata json.RawMessage, err error)
}

// ReportCacheInvalidator drops cached report aggregates after AI writes.
type ReportCacheInvalidator interface {
	Invalidate(reportID uuid.UUID)
}

type OutboxEvent struct {
	Topic         string
	PartitionKey  string
	Payload       []byte
	CorrelationID string
}

type ObjectStore interface {
	// Optional future: download frame bytes for multimodal. Mock ignores.
}
