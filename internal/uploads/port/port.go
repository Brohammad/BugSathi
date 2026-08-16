package port

import (
	"context"
	"time"

	"github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/google/uuid"
)

type RecordingRepository interface {
	Create(ctx context.Context, rec domain.Recording) (domain.Recording, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Recording, error)
	Update(ctx context.Context, rec domain.Recording) (domain.Recording, error)
	// CompleteInTx transitions to UPLOADED and inserts outbox in one transaction.
	CompleteWithOutbox(ctx context.Context, rec domain.Recording, eventTopic, partitionKey string, payload []byte, correlationID string) (domain.Recording, error)
	// InsertOutbox appends an outbox row without changing recording status (reprocess).
	InsertOutbox(ctx context.Context, eventTopic, partitionKey string, payload []byte, correlationID string) error
}

type OutboxRepository interface {
	// WithClaimed locks up to limit unpublished rows, runs fn, and on success
	// marks them published in the same transaction. Concurrent callers using
	// FOR UPDATE SKIP LOCKED (Postgres) or an in-process mutex (memory) never
	// claim the same row. If fn fails, the claim is released and rows stay unpublished.
	WithClaimed(ctx context.Context, limit int, fn func(ctx context.Context, msgs []OutboxMessage) error) error
}

type OutboxMessage struct {
	ID            int64
	Topic         string
	PartitionKey  string
	Payload       []byte
	CorrelationID string
	CreatedAt     time.Time
}

type ObjectStorage interface {
	PresignPut(ctx context.Context, key, contentType string, expiry time.Duration) (url string, err error)
	Stat(ctx context.Context, key string) (size int64, contentType string, err error)
}

type ProjectAccess interface {
	EnsureMember(ctx context.Context, userID, projectID uuid.UUID) error
	EnsureOwner(ctx context.Context, userID, projectID uuid.UUID) error
}

type EventPublisher interface {
	Publish(ctx context.Context, topic, key string, payload []byte, headers map[string]string) error
}
