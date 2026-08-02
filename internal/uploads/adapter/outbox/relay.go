package outbox

import (
	"context"
	"log/slog"
	"time"

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

func (r *Relay) Flush(ctx context.Context) error {
	msgs, err := r.repo.ListUnpublished(ctx, r.batch)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		headers := map[string]string{}
		if m.CorrelationID != "" {
			headers["correlation_id"] = m.CorrelationID
		}
		if err := r.pub.Publish(ctx, m.Topic, m.PartitionKey, m.Payload, headers); err != nil {
			return err
		}
		if err := r.repo.MarkPublished(ctx, m.ID, time.Now().UTC()); err != nil {
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
}
