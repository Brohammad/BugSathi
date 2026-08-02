package domain

import (
	"encoding/json"
	"errors"
	"time"

	mediadomain "github.com/Brohammad/BugSathi/internal/media/domain"
	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("not found")
)

const PromptVersion = "prompt_v1"

const (
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

type AnalysisInput struct {
	RecordingID   string
	ProjectID     string
	FrameKeys     []string
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
