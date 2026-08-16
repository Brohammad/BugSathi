package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/Brohammad/BugSathi/internal/uploads/port"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RecordingRepo struct {
	pool *pgxpool.Pool
}

func NewRecordingRepo(pool *pgxpool.Pool) *RecordingRepo {
	return &RecordingRepo{pool: pool}
}

func (r *RecordingRepo) Create(ctx context.Context, rec domain.Recording) (domain.Recording, error) {
	const q = `
		INSERT INTO recordings (
			id, project_id, created_by, status, storage_key, content_type,
			byte_size, checksum, metadata, correlation_id, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, project_id, created_by, status, storage_key, content_type,
			byte_size, checksum, metadata, correlation_id, created_at, updated_at`
	return scanRecording(r.pool.QueryRow(ctx, q,
		rec.ID, rec.ProjectID, rec.CreatedBy, string(rec.Status), rec.StorageKey, rec.ContentType,
		rec.ByteSize, nullStr(rec.Checksum), rec.Metadata, rec.CorrelationID, rec.CreatedAt, rec.UpdatedAt,
	))
}

func (r *RecordingRepo) Get(ctx context.Context, id uuid.UUID) (domain.Recording, error) {
	const q = `
		SELECT id, project_id, created_by, status, storage_key, content_type,
			byte_size, checksum, metadata, correlation_id, created_at, updated_at
		FROM recordings WHERE id = $1`
	return scanRecording(r.pool.QueryRow(ctx, q, id))
}

func (r *RecordingRepo) Update(ctx context.Context, rec domain.Recording) (domain.Recording, error) {
	const q = `
		UPDATE recordings SET
			status = $2, byte_size = $3, checksum = $4, metadata = $5, updated_at = $6
		WHERE id = $1
		RETURNING id, project_id, created_by, status, storage_key, content_type,
			byte_size, checksum, metadata, correlation_id, created_at, updated_at`
	return scanRecording(r.pool.QueryRow(ctx, q,
		rec.ID, string(rec.Status), rec.ByteSize, nullStr(rec.Checksum), rec.Metadata, rec.UpdatedAt,
	))
}

func (r *RecordingRepo) CompleteWithOutbox(
	ctx context.Context,
	rec domain.Recording,
	eventTopic, partitionKey string,
	payload []byte,
	correlationID string,
) (domain.Recording, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Recording{}, err
	}
	defer tx.Rollback(ctx)

	const upd = `
		UPDATE recordings SET
			status = $2, byte_size = $3, checksum = $4, updated_at = $5
		WHERE id = $1 AND status = 'UPLOADING'
		RETURNING id, project_id, created_by, status, storage_key, content_type,
			byte_size, checksum, metadata, correlation_id, created_at, updated_at`
	out, err := scanRecording(tx.QueryRow(ctx, upd,
		rec.ID, string(rec.Status), rec.ByteSize, nullStr(rec.Checksum), rec.UpdatedAt,
	))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// raced / already completed — re-read
			return r.Get(ctx, rec.ID)
		}
		return domain.Recording{}, err
	}

	const ins = `
		INSERT INTO outbox (topic, partition_key, payload, correlation_id)
		VALUES ($1, $2, $3, $4)`
	if _, err := tx.Exec(ctx, ins, eventTopic, partitionKey, payload, correlationID); err != nil {
		return domain.Recording{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Recording{}, err
	}
	return out, nil
}

func (r *RecordingRepo) InsertOutbox(
	ctx context.Context,
	eventTopic, partitionKey string,
	payload []byte,
	correlationID string,
) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO outbox (topic, partition_key, payload, correlation_id)
		VALUES ($1, $2, $3, $4)`, eventTopic, partitionKey, payload, correlationID)
	return err
}

type OutboxRepo struct {
	pool *pgxpool.Pool
}

func NewOutboxRepo(pool *pgxpool.Pool) *OutboxRepo {
	return &OutboxRepo{pool: pool}
}

func (r *OutboxRepo) WithClaimed(ctx context.Context, limit int, fn func(context.Context, []port.OutboxMessage) error) error {
	if limit <= 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // no-op after Commit

	const q = `
		SELECT id, topic, partition_key, payload, correlation_id, created_at
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY id
		LIMIT $1
		FOR UPDATE SKIP LOCKED`
	rows, err := tx.Query(ctx, q, limit)
	if err != nil {
		return err
	}
	var msgs []port.OutboxMessage
	for rows.Next() {
		var m port.OutboxMessage
		if err := rows.Scan(&m.ID, &m.Topic, &m.PartitionKey, &m.Payload, &m.CorrelationID, &m.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		msgs = append(msgs, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(msgs) == 0 {
		return nil
	}

	if err := fn(ctx, msgs); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, m := range msgs {
		tag, err := tx.Exec(ctx, `UPDATE outbox SET published_at = $2 WHERE id = $1 AND published_at IS NULL`, m.ID, now)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("outbox mark published: id=%d rows=%d", m.ID, tag.RowsAffected())
		}
	}
	return tx.Commit(ctx)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanRecording(row scannable) (domain.Recording, error) {
	var out domain.Recording
	var status string
	var checksum *string
	err := row.Scan(
		&out.ID, &out.ProjectID, &out.CreatedBy, &status, &out.StorageKey, &out.ContentType,
		&out.ByteSize, &checksum, &out.Metadata, &out.CorrelationID, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Recording{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Recording{}, err
	}
	out.Status = domain.Status(status)
	if checksum != nil {
		out.Checksum = *checksum
	}
	return out, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
