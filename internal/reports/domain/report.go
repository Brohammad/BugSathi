package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound  = errors.New("report not found")
	ErrForbidden = errors.New("forbidden")
)

type Status string

const (
	StatusPending    Status = "PENDING"
	StatusGenerating Status = "GENERATING"
	StatusReady      Status = "READY"
	StatusFailed     Status = "FAILED"
)

type Report struct {
	ID            uuid.UUID
	RecordingID   uuid.UUID
	ProjectID     uuid.UUID
	Status        Status
	Title         string
	Summary       string
	Steps         json.RawMessage
	AIStatus      string
	PromptVersion string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Frame struct {
	Ordinal     int
	StorageKey  string
	ContentType string
	ByteSize    *int64
	URL         string // presigned GET when available
}

type Detail struct {
	Report          Report
	RecordingStatus string
	Metadata        json.RawMessage
	Frames          []Frame
	ThumbURL        string
}
