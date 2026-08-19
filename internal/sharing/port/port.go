package port

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/sharing/domain"
	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, link domain.ShareLink, outboxTopic, partitionKey string, payload []byte, corr string) (domain.ShareLink, error)
	ListByReport(ctx context.Context, projectID, reportID uuid.UUID, page pagination.Page) (pagination.Result[domain.ShareLink], error)
	GetByID(ctx context.Context, projectID, shareID uuid.UUID) (domain.ShareLink, error)
	Revoke(ctx context.Context, projectID, shareID uuid.UUID, at time.Time) error
	GetByToken(ctx context.Context, token string) (domain.ShareLink, error)
}

type ProjectAccess interface {
	EnsureMember(ctx context.Context, userID, projectID uuid.UUID) error
}

type ReportReader interface {
	// GetPublicPayload returns limited report fields if report belongs to project and is READY.
	GetPublicPayload(ctx context.Context, projectID, reportID uuid.UUID) (PublicReport, error)
}

type PublicReport struct {
	ReportID  uuid.UUID
	ProjectID uuid.UUID
	Status    string
	Title     string
	Summary   string
	Steps     json.RawMessage
	Frames    []PublicFrame
	ThumbKey  string
}

type PublicFrame struct {
	Ordinal     int
	StorageKey  string
	ContentType string
}

type URLSigner interface {
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}
