package observability

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboxLagPoller updates bugsathi_outbox_pending from Postgres.
type OutboxLagPoller struct {
	pool     *pgxpool.Pool
	metrics  *Metrics
	interval time.Duration
}

func NewOutboxLagPoller(pool *pgxpool.Pool, metrics *Metrics) *OutboxLagPoller {
	return &OutboxLagPoller{pool: pool, metrics: metrics, interval: 15 * time.Second}
}

func (p *OutboxLagPoller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	p.sample(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.sample(ctx)
		}
	}
}

func (p *OutboxLagPoller) sample(ctx context.Context) {
	var n float64
	err := p.pool.QueryRow(ctx, `SELECT count(*)::float8 FROM outbox WHERE published_at IS NULL`).Scan(&n)
	if err != nil {
		return
	}
	p.metrics.OutboxPending.Set(n)
}
