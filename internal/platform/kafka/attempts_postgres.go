package kafka

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresAttemptStore persists Kafka retry counters so worker restarts do not
// reset the path to DLQ (ADR 0023 / 0034).
type PostgresAttemptStore struct {
	pool *pgxpool.Pool
	mem  *AttemptTracker // fallback if Postgres is briefly unavailable
	log  *slog.Logger
}

func NewPostgresAttemptStore(pool *pgxpool.Pool, log *slog.Logger) *PostgresAttemptStore {
	if log == nil {
		log = slog.Default()
	}
	return &PostgresAttemptStore{pool: pool, mem: NewAttemptTracker(), log: log}
}

func (s *PostgresAttemptStore) Inc(topic string, partition int, offset int64) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	const q = `
		INSERT INTO kafka_retry_attempts (topic, partition, "offset", attempts, updated_at)
		VALUES ($1, $2, $3, 1, $4)
		ON CONFLICT (topic, partition, "offset") DO UPDATE
		SET attempts = kafka_retry_attempts.attempts + 1,
		    updated_at = EXCLUDED.updated_at
		RETURNING attempts`
	var n int
	err := s.pool.QueryRow(ctx, q, topic, partition, offset, time.Now().UTC()).Scan(&n)
	if err != nil {
		s.log.Error("kafka attempt store incr failed; using memory fallback",
			"topic", topic, "partition", partition, "offset", offset, "error", err)
		return s.mem.Inc(topic, partition, offset)
	}
	return n
}

func (s *PostgresAttemptStore) Clear(topic string, partition int, offset int64) {
	s.mem.Clear(topic, partition, offset)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		DELETE FROM kafka_retry_attempts
		WHERE topic = $1 AND partition = $2 AND "offset" = $3`,
		topic, partition, offset)
	if err != nil {
		s.log.Error("kafka attempt store clear failed",
			"topic", topic, "partition", partition, "offset", offset, "error", err)
	}
}
