package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound             = errors.New("recording not found")
	ErrForbidden            = errors.New("forbidden")
	ErrInvalidInput         = errors.New("invalid input")
	ErrIllegalTransition    = errors.New("illegal status transition")
	ErrObjectMissing        = errors.New("uploaded object not found in storage")
	ErrObjectTooLarge       = errors.New("uploaded object exceeds size limit")
	ErrContentTypeMismatch  = errors.New("uploaded object content type mismatch")
)

type Status string

const (
	StatusUploading  Status = "UPLOADING"
	StatusUploaded   Status = "UPLOADED"
	StatusProcessing Status = "PROCESSING"
	StatusReady      Status = "READY"
	StatusFailed     Status = "FAILED"
)

var allowed = map[Status]map[Status]bool{
	StatusUploading:  {StatusUploaded: true, StatusFailed: true},
	StatusUploaded:   {StatusProcessing: true, StatusFailed: true},
	StatusProcessing: {StatusReady: true, StatusFailed: true},
	StatusFailed:     {StatusProcessing: true},
}

type Recording struct {
	ID            uuid.UUID
	ProjectID     uuid.UUID
	CreatedBy     uuid.UUID
	Status        Status
	StorageKey    string
	ContentType   string
	ByteSize      *int64
	Checksum      string
	Metadata      json.RawMessage
	CorrelationID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (r *Recording) Transition(to Status, now time.Time) error {
	if r.Status == to {
		return nil
	}
	next, ok := allowed[r.Status]
	if !ok || !next[to] {
		return ErrIllegalTransition
	}
	r.Status = to
	r.UpdatedAt = now
	return nil
}

func (r Recording) IsUploadedOrBeyond() bool {
	switch r.Status {
	case StatusUploaded, StatusProcessing, StatusReady:
		return true
	default:
		return false
	}
}

const TopicRecordingUploaded = "bugsathi.recording.uploaded"

type RecordingUploadedEvent struct {
	SchemaVersion int             `json:"schema_version"`
	RecordingID   string          `json:"recording_id"`
	ProjectID     string          `json:"project_id"`
	ObjectKey     string          `json:"object_key"`
	ContentType   string          `json:"content_type"`
	ByteSize      int64           `json:"byte_size"`
	Checksum      string          `json:"checksum,omitempty"`
	Metadata      json.RawMessage `json:"metadata"`
	CorrelationID string          `json:"correlation_id"`
	OccurredAt    time.Time       `json:"occurred_at"`
}
