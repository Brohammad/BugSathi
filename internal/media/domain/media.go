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
	// ErrClaimHeld means another worker holds a live processing lease, so this
	// delivery must not run ffmpeg again.
	ErrClaimHeld = errors.New("recording claimed by another worker")
	// ErrClaimLost means our lease expired or was taken over mid-job, so the
	// work we just did must not be committed.
	ErrClaimLost = errors.New("processing claim lost")
)

// IsClaimConflict reports whether the error means some other worker owns this
// recording. Such a delivery is finished from our side: nothing to retry.
func IsClaimConflict(err error) bool {
	return errors.Is(err, ErrClaimHeld) || errors.Is(err, ErrClaimLost)
}

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
