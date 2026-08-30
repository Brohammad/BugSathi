package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	mediadomain "github.com/Brohammad/BugSathi/internal/media/domain"
	"github.com/google/uuid"
)

var (
	ErrNotFound              = errors.New("not found")
	ErrInvalidAnalysisResult = errors.New("invalid analysis result")
	// ErrAnalysisInFlight means another worker holds a live analysis lease.
	// Kafka consumers should commit and skip rather than retry into the DLQ.
	ErrAnalysisInFlight = errors.New("analysis already in progress")
)

// IsInFlight reports whether the error means some other worker owns this analysis.
func IsInFlight(err error) bool {
	return errors.Is(err, ErrAnalysisInFlight)
}

const PromptVersion = "prompt_v1"

const (
	TopicAnalysisStarted   = "bugsathi.analysis.started"
	TopicAnalysisCompleted = "bugsathi.analysis.completed"
	TopicReportGenerated   = "bugsathi.report.generated"
)

const TopicFramesExtracted = mediadomain.TopicFramesExtracted

type FramesExtractedEvent = mediadomain.FramesExtractedEvent

type AnalysisStatus string

const (
	AnalysisPending   AnalysisStatus = "pending"
	AnalysisRunning   AnalysisStatus = "running"
	AnalysisCompleted AnalysisStatus = "completed"
	AnalysisFailed    AnalysisStatus = "failed"
)

type ReportStatus string

const (
	ReportPending    ReportStatus = "PENDING"
	ReportGenerating ReportStatus = "GENERATING"
	ReportReady      ReportStatus = "READY"
	ReportFailed     ReportStatus = "FAILED"
)

type Analysis struct {
	ID            uuid.UUID
	RecordingID   uuid.UUID
	ProjectID     uuid.UUID
	PromptVersion string
	Status        AnalysisStatus
	Title         string
	Summary       string
	Steps         json.RawMessage
	Provider      string
	Model         string
	ErrorMessage  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Report struct {
	ID            uuid.UUID
	RecordingID   uuid.UUID
	ProjectID     uuid.UUID
	Status        ReportStatus
	Title         string
	Summary       string
	Steps         json.RawMessage
	AIStatus      string
	PromptVersion string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// FrameInput is a bounded, provider-neutral visual input loaded from object
// storage before analysis. StorageKey is retained for ordering and diagnostics;
// analyzers must use Data rather than assuming they can access private storage.
type FrameInput struct {
	StorageKey string
	MediaType  string
	Data       []byte
}

type AnalysisInput struct {
	RecordingID   string
	ProjectID     string
	FrameKeys     []string
	Frames        []FrameInput
	MetadataJSON  json.RawMessage
	PromptVersion string
}

type AnalysisResult struct {
	Title    string
	Summary  string
	Steps    []string
	Provider string
	Model    string
}

// ValidateAnalysisResult rejects empty or malformed LLM output before persistence.
func ValidateAnalysisResult(r AnalysisResult) error {
	title := strings.TrimSpace(r.Title)
	summary := strings.TrimSpace(r.Summary)
	if title == "" || summary == "" {
		return ErrInvalidAnalysisResult
	}
	var steps []string
	for _, step := range r.Steps {
		if s := strings.TrimSpace(step); s != "" {
			steps = append(steps, s)
		}
	}
	if len(steps) == 0 {
		return ErrInvalidAnalysisResult
	}
	return nil
}

// NormalizeAnalysisResult trims fields and drops blank steps for storage.
func NormalizeAnalysisResult(r AnalysisResult) AnalysisResult {
	out := r
	out.Title = strings.TrimSpace(r.Title)
	out.Summary = strings.TrimSpace(r.Summary)
	out.Steps = out.Steps[:0]
	for _, step := range r.Steps {
		if s := strings.TrimSpace(step); s != "" {
			out.Steps = append(out.Steps, s)
		}
	}
	return out
}

type AnalysisStartedEvent struct {
	SchemaVersion int       `json:"schema_version"`
	RecordingID   string    `json:"recording_id"`
	ProjectID     string    `json:"project_id"`
	ReportID      string    `json:"report_id"`
	PromptVersion string    `json:"prompt_version"`
	CorrelationID string    `json:"correlation_id"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type AnalysisCompletedEvent struct {
	SchemaVersion int       `json:"schema_version"`
	RecordingID   string    `json:"recording_id"`
	ReportID      string    `json:"report_id"`
	PromptVersion string    `json:"prompt_version"`
	CorrelationID string    `json:"correlation_id"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type ReportGeneratedEvent struct {
	SchemaVersion int       `json:"schema_version"`
	ReportID      string    `json:"report_id"`
	RecordingID   string    `json:"recording_id"`
	ProjectID     string    `json:"project_id"`
	CorrelationID string    `json:"correlation_id"`
	OccurredAt    time.Time `json:"occurred_at"`
}
