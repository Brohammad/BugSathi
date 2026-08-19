package port

import (
	"context"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/reports/domain"
	"github.com/google/uuid"
)

type Repository interface {
	ListByProject(ctx context.Context, projectID uuid.UUID, page pagination.Page) (pagination.Result[domain.Report], error)
	GetByID(ctx context.Context, projectID, reportID uuid.UUID) (domain.Detail, error)
	GetByRecordingID(ctx context.Context, projectID, recordingID uuid.UUID) (domain.Detail, error)
}

type ProjectAccess interface {
	EnsureMember(ctx context.Context, userID, projectID uuid.UUID) error
}

type URLSigner interface {
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}
