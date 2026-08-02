package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrForbidden    = errors.New("forbidden")
)

const MaxBodyLen = 4000

type Comment struct {
	ID        uuid.UUID
	ReportID  uuid.UUID
	ProjectID uuid.UUID
	AuthorID  uuid.UUID
	Body      string
	CreatedAt time.Time
}

func NormalizeBody(body string) (string, error) {
	b := strings.TrimSpace(body)
	if b == "" {
		return "", ErrInvalidInput
	}
	if utf8.RuneCountInString(b) > MaxBodyLen {
		return "", ErrInvalidInput
	}
	return b, nil
}

const (
	EventCommentCreated  = "comment.created"
	EventPresenceUpdated = "presence.updated"
	EventHeartbeat       = "heartbeat"
)

const TopicCommentCreated = "bugsathi.comment.created"

type CommentCreatedEvent struct {
	SchemaVersion int       `json:"schema_version"`
	CommentID     string    `json:"comment_id"`
	ReportID      string    `json:"report_id"`
	ProjectID     string    `json:"project_id"`
	AuthorID      string    `json:"author_id"`
	OccurredAt    time.Time `json:"occurred_at"`
}
