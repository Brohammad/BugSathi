package outbox

import (
	"context"
	"log/slog"
	"time"

	pkafka "github.com/Brohammad/BugSathi/internal/platform/kafka"
	"github.com/Brohammad/BugSathi/internal/uploads/port"
)

type Publisher interface {
	Publish(ctx context.Context, topic, key string, payload []byte, headers map[string]string) error
}

type Relay struct {
	repo     port.OutboxRepository
	pub      Publisher
	log      *slog.Logger
	interval time.Duration
	batch    int
}

func NewRelay(repo port.OutboxRepository, pub Publisher, log *slog.Logger) *Relay {
	return &Relay{
		repo:     repo,
		pub:      pub,
		log:      log,
		interval: 500 * time.Millisecond,
		batch:    50,
	}
}

func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		if err := r.Flush(ctx); err != nil {
			r.log.Error("outbox flush failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Flush claims a batch of unpublished rows (SKIP LOCKED / mutex), publishes
// each to Kafka, then marks them published in the same claim transaction.
func (r *Relay) Flush(ctx context.Context) error {
	return r.repo.WithClaimed(ctx, r.batch, func(ctx context.Context, msgs []port.OutboxMessage) error {
		for _, m := range msgs {
			headers := map[string]string{}
			if m.CorrelationID != "" {
				headers[pkafka.HeaderCorrelationID] = m.CorrelationID
			}
			// Pipeline events use recording_id as the partition key; expose it
			// as a header so consumers can restore context without parsing JSON.
			if m.PartitionKey != "" {
				headers[pkafka.HeaderRecordingID] = m.PartitionKey
			}
			if err := r.pub.Publish(ctx, m.Topic, m.PartitionKey, m.Payload, headers); err != nil {
				return err
			}
			r.log.Info("outbox published",
				"outbox_id", m.ID,
				"topic", m.Topic,
				"key", m.PartitionKey,
				"correlation_id", m.CorrelationID,
			)
		}
		return nil
	})
}
