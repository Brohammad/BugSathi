package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound       = errors.New("share not found")
	ErrForbidden      = errors.New("forbidden")
	ErrInvalidInput   = errors.New("invalid input")
	ErrReportNotReady = errors.New("report is not ready to share")
	ErrShareInactive  = errors.New("share link is expired or revoked")
)

const TopicShareCreated = "bugsathi.share.created"

type ShareLink struct {
	ID        uuid.UUID
	ReportID  uuid.UUID
	ProjectID uuid.UUID
	Token     string
	ExpiresAt *time.Time
	RevokedAt *time.Time
	CreatedBy uuid.UUID
	CreatedAt time.Time
}

func (s ShareLink) Active(now time.Time) bool {
	if s.RevokedAt != nil {
		return false
	}
	if s.ExpiresAt != nil && !now.Before(*s.ExpiresAt) {
		return false
	}
	return true
}

func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken fingerprints a raw share token for storage (SHA-256 hex).
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

type ShareCreatedEvent struct {
	SchemaVersion int        `json:"schema_version"`
	ShareID       string     `json:"share_id"`
	ReportID      string     `json:"report_id"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	CorrelationID string     `json:"correlation_id"`
	OccurredAt    time.Time  `json:"occurred_at"`
}
