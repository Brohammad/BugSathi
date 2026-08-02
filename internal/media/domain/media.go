package domain

import (
	"errors"
	"time"

	uploaddomain "github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("recording not found")
	ErrConflict = errors.New("recording not in expected state")
)

type ArtifactKind string

const (
	KindFrame ArtifactKind = "frame"
	KindThumb ArtifactKind = "thumb"
)

type Artifact struct {
	ID          uuid.UUID
	RecordingID uuid.UUID
	Kind        ArtifactKind
	StorageKey  string
	Ordinal     int
	ContentType string
	ByteSize    *int64
	CreatedAt   time.Time
}

const TopicFramesExtracted = "bugsathi.recording.frames-extracted"

type FramesExtractedEvent struct {
	SchemaVersion int       `json:"schema_version"`
	RecordingID   string    `json:"recording_id"`
	ProjectID     string    `json:"project_id"`
	FrameKeys     []string  `json:"frame_keys"`
	ThumbKey      string    `json:"thumb_key"`
	DurationMS    int64     `json:"duration_ms"`
	CorrelationID string    `json:"correlation_id"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// Re-export upload event topic for consumer wiring clarity.
const TopicRecordingUploaded = uploaddomain.TopicRecordingUploaded

type RecordingUploadedEvent = uploaddomain.RecordingUploadedEvent
