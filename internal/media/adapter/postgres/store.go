package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Brohammad/BugSathi/internal/media/domain"
	uploaddomain "github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (uploaddomain.Recording, error) {
	const q = `
		SELECT id, project_id, created_by, status, storage_key, content_type,
			byte_size, checksum, metadata, correlation_id, created_at, updated_at
		FROM recordings WHERE id = $1`
	var out uploaddomain.Recording
	var status string
	var checksum *string
	err := s.pool.QueryRow(ctx, q, id).Scan(
		&out.ID, &out.ProjectID, &out.CreatedBy, &status, &out.StorageKey, &out.ContentType,
		&out.ByteSize, &checksum, &out.Metadata, &out.CorrelationID, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return uploaddomain.Recording{}, domain.ErrNotFound
	}
	if err != nil {
		return uploaddomain.Recording{}, err
	}
	out.Status = uploaddomain.Status(status)
	if checksum != nil {
		out.Checksum = *checksum
	}
	return out, nil
}

func (s *Store) ClaimProcessing(ctx context.Context, id uuid.UUID, owner string, at, expiresAt time.Time) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE recordings
		SET status = 'PROCESSING',
		    processing_owner = $2,
		    processing_expires_at = $4,
		    updated_at = $3
		WHERE id = $1
		  AND status IN ('UPLOADED', 'FAILED', 'PROCESSING')
		  AND (
		        status <> 'PROCESSING'
		     OR processing_owner IS NULL
		     OR processing_owner = $2
		     OR processing_expires_at <= $3
		  )`, id, owner, at, expiresAt)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return s.claimRejection(ctx, id, at)
	}
	return nil
}

// claimRejection turns a no-op claim UPDATE into the reason it did not apply.
func (s *Store) claimRejection(ctx context.Context, id uuid.UUID, at time.Time) error {
	var status string
	var expiresAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT status, processing_expires_at FROM recordings WHERE id = $1`, id).Scan(&status, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status == string(uploaddomain.StatusProcessing) && expiresAt != nil && expiresAt.After(at) {
		return domain.ErrClaimHeld
	}
	return domain.ErrConflict
}

func (s *Store) RenewClaim(ctx context.Context, id uuid.UUID, owner string, expiresAt time.Time) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE recordings SET processing_expires_at = $3
		WHERE id = $1 AND status = 'PROCESSING' AND processing_owner = $2`, id, owner, expiresAt)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrClaimLost
	}
	return nil
}

func (s *Store) MarkFailed(ctx context.Context, id uuid.UUID, owner string, at time.Time) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE recordings
		SET status = 'FAILED',
		    processing_owner = NULL,
		    processing_expires_at = NULL,
		    updated_at = $3
		WHERE id = $1 AND processing_owner = $2`, id, owner, at)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrClaimLost
	}
	return nil
}

func (s *Store) FinalizeReady(
	ctx context.Context,
	id uuid.UUID,
	owner string,
	at time.Time,
	artifacts []domain.Artifact,
	topic, key string,
	payload []byte,
	correlationID string,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	ct, err := tx.Exec(ctx, `
		UPDATE recordings
		SET status = 'READY',
		    processing_owner = NULL,
		    processing_expires_at = NULL,
		    updated_at = $3
		WHERE id = $1 AND processing_owner = $2`, id, owner, at)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrClaimLost
	}

	for _, a := range artifacts {
		if _, err := tx.Exec(ctx, `
			INSERT INTO media_artifacts (id, recording_id, kind, storage_key, ordinal, content_type, byte_size, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (recording_id, kind, ordinal) DO UPDATE
			SET storage_key = EXCLUDED.storage_key,
			    content_type = EXCLUDED.content_type,
			    byte_size = EXCLUDED.byte_size`,
			a.ID, a.RecordingID, string(a.Kind), a.StorageKey, a.Ordinal, a.ContentType, a.ByteSize, a.CreatedAt,
		); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox (topic, partition_key, payload, correlation_id)
		VALUES ($1,$2,$3,$4)`, topic, key, payload, correlationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) InsertOutbox(ctx context.Context, topic, key string, payload []byte, correlationID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO outbox (topic, partition_key, payload, correlation_id)
		VALUES ($1,$2,$3,$4)`, topic, key, payload, correlationID)
	return err
}

func (s *Store) ListArtifacts(ctx context.Context, recordingID uuid.UUID) ([]domain.Artifact, error) {
	const q = `
		SELECT id, recording_id, kind, storage_key, ordinal, content_type, byte_size, created_at
		FROM media_artifacts WHERE recording_id = $1 ORDER BY kind, ordinal`
	rows, err := s.pool.Query(ctx, q, recordingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Artifact
	for rows.Next() {
		var a domain.Artifact
		var kind string
		if err := rows.Scan(&a.ID, &a.RecordingID, &kind, &a.StorageKey, &a.Ordinal, &a.ContentType, &a.ByteSize, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Kind = domain.ArtifactKind(kind)
		out = append(out, a)
	}
	return out, rows.Err()
}
