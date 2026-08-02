package port

import (
	"context"

	"github.com/Brohammad/BugSathi/internal/collab/domain"
	"github.com/google/uuid"
)

type ProjectAccess interface {
	EnsureMember(ctx context.Context, userID, projectID uuid.UUID) error
}

type ReportGuard interface {
	EnsureInProject(ctx context.Context, projectID, reportID uuid.UUID) error
}

type AuthorLookup interface {
	DisplayName(ctx context.Context, userID uuid.UUID) (string, error)
}

type Repository interface {
	Create(ctx context.Context, c domain.Comment, outboxTopic, partitionKey string, payload []byte, corr string) (domain.Comment, error)
	ListByReport(ctx context.Context, projectID, reportID uuid.UUID) ([]domain.Comment, error)
}

type PresenceUser struct {
	UserID uuid.UUID `json:"user_id"`
	Name   string    `json:"name"`
}

type StreamEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Hub fans out realtime events for a report channel (single-process MVP).
type Hub interface {
	Subscribe(reportID, userID uuid.UUID, name string) (<-chan StreamEvent, func())
	Publish(reportID uuid.UUID, ev StreamEvent)
	Presence(reportID uuid.UUID) []PresenceUser
}
