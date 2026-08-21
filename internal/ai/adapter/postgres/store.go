package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Brohammad/BugSathi/internal/ai/domain"
	"github.com/Brohammad/BugSathi/internal/ai/port"
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

func (s *Store) GetAnalysis(ctx context.Context, recordingID uuid.UUID, promptVersion string) (domain.Analysis, error) {
	const q = `
		SELECT id, recording_id, project_id, prompt_version, status, title, summary, steps,
			provider, model, error_message, created_at, updated_at
		FROM analyses WHERE recording_id = $1 AND prompt_version = $2`
	return scanAnalysis(s.pool.QueryRow(ctx, q, recordingID, promptVersion))
}

func (s *Store) TryClaimRunning(ctx context.Context, recordingID, projectID uuid.UUID, promptVersion string, at, leaseCutoff time.Time) (domain.Analysis, error) {
	const q = `
		INSERT INTO analyses (id, recording_id, project_id, prompt_version, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,'running',$5,$5)
		ON CONFLICT (recording_id, prompt_version) DO UPDATE
		SET status = 'running', updated_at = EXCLUDED.updated_at, error_message = ''
		WHERE analyses.status IN ('pending', 'failed')
		   OR (analyses.status = 'running' AND analyses.updated_at < $6)
		RETURNING id, recording_id, project_id, prompt_version, status, title, summary, steps,
			provider, model, error_message, created_at, updated_at`
	out, err := scanAnalysis(s.pool.QueryRow(ctx, q, uuid.New(), recordingID, projectID, promptVersion, at, leaseCutoff))
	if errors.Is(err, domain.ErrNotFound) {
		existing, gerr := s.GetAnalysis(ctx, recordingID, promptVersion)
		if gerr != nil {
			return domain.Analysis{}, gerr
		}
		switch existing.Status {
		case domain.AnalysisCompleted:
			return domain.Analysis{}, domain.ErrNotFound
		case domain.AnalysisRunning:
			return domain.Analysis{}, domain.ErrAnalysisInFlight
		default:
			return domain.Analysis{}, domain.ErrAnalysisInFlight
		}
	}
	return out, err
}

func (s *Store) TouchRunning(ctx context.Context, recordingID uuid.UUID, promptVersion string, at time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE analyses SET updated_at = $3
		WHERE recording_id = $1 AND prompt_version = $2 AND status = 'running'`,
		recordingID, promptVersion, at)
	return err
}

func (s *Store) MarkGenerating(ctx context.Context, recordingID, projectID uuid.UUID, corr string, at time.Time) (domain.Report, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Report{}, err
	}
	defer tx.Rollback(ctx)

	reportID := uuid.New()
	const upsert = `
		INSERT INTO reports (id, recording_id, project_id, status, ai_status, prompt_version, created_at, updated_at)
		VALUES ($1,$2,$3,'GENERATING','running',$4,$5,$5)
		ON CONFLICT (recording_id) DO UPDATE SET
			status = 'GENERATING',
			ai_status = 'running',
			prompt_version = EXCLUDED.prompt_version,
			updated_at = EXCLUDED.updated_at
		RETURNING id, recording_id, project_id, status, title, summary, steps, ai_status, prompt_version, created_at, updated_at`
	var out domain.Report
	var status string
	err = tx.QueryRow(ctx, upsert, reportID, recordingID, projectID, domain.PromptVersion, at).Scan(
		&out.ID, &out.RecordingID, &out.ProjectID, &status,
		&out.Title, &out.Summary, &out.Steps, &out.AIStatus, &out.PromptVersion,
		&out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return domain.Report{}, err
	}
	out.Status = domain.ReportStatus(status)

	payload, err := json.Marshal(domain.AnalysisStartedEvent{
		SchemaVersion: 1, RecordingID: recordingID.String(), ProjectID: projectID.String(),
		ReportID: out.ID.String(), PromptVersion: domain.PromptVersion,
		CorrelationID: corr, OccurredAt: at.UTC(),
	})
	if err != nil {
		return domain.Report{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox (topic, partition_key, payload, correlation_id)
		VALUES ($1,$2,$3,$4)`, domain.TopicAnalysisStarted, recordingID.String(), payload, corr); err != nil {
		return domain.Report{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Report{}, err
	}
	return out, nil
}

func (s *Store) CompleteAnalysis(ctx context.Context, analysis domain.Analysis, report domain.Report, events []port.OutboxEvent, at time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO analyses (id, recording_id, project_id, prompt_version, status, title, summary, steps, provider, model, error_message, created_at, updated_at)
		VALUES ($1,$2,$3,$4,'completed',$5,$6,$7,$8,$9,'',$10,$10)
		ON CONFLICT (recording_id, prompt_version) DO UPDATE SET
			status = 'completed', title = EXCLUDED.title, summary = EXCLUDED.summary, steps = EXCLUDED.steps,
			provider = EXCLUDED.provider, model = EXCLUDED.model, error_message = '', updated_at = EXCLUDED.updated_at`,
		analysis.ID, analysis.RecordingID, analysis.ProjectID, analysis.PromptVersion,
		analysis.Title, analysis.Summary, analysis.Steps, analysis.Provider, analysis.Model, at,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO reports (id, recording_id, project_id, status, title, summary, steps, ai_status, prompt_version, created_at, updated_at)
		VALUES ($1,$2,$3,'READY',$4,$5,$6,$7,$8,$9,$9)
		ON CONFLICT (recording_id) DO UPDATE SET
			status = 'READY', title = EXCLUDED.title, summary = EXCLUDED.summary, steps = EXCLUDED.steps,
			ai_status = EXCLUDED.ai_status, prompt_version = EXCLUDED.prompt_version, updated_at = EXCLUDED.updated_at`,
		report.ID, report.RecordingID, report.ProjectID, report.Title, report.Summary, report.Steps,
		report.AIStatus, report.PromptVersion, at,
	); err != nil {
		return err
	}

	for _, e := range events {
		if _, err := tx.Exec(ctx, `
			INSERT INTO outbox (topic, partition_key, payload, correlation_id)
			VALUES ($1,$2,$3,$4)`, e.Topic, e.PartitionKey, e.Payload, e.CorrelationID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) FailAnalysis(ctx context.Context, recordingID uuid.UUID, promptVersion, msg string, at time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE analyses SET status = 'failed', error_message = $3, updated_at = $4
		WHERE recording_id = $1 AND prompt_version = $2`, recordingID, promptVersion, truncate(msg, 1000), at)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO reports (id, recording_id, project_id, status, ai_status, prompt_version, created_at, updated_at)
		SELECT gen_random_uuid(), r.id, r.project_id, 'FAILED', 'failed', $2, $3, $3
		FROM recordings r WHERE r.id = $1
		ON CONFLICT (recording_id) DO UPDATE SET status = 'FAILED', ai_status = 'failed', updated_at = EXCLUDED.updated_at`,
		recordingID, promptVersion, at)
	return err
}

func (s *Store) GetRecordingMeta(ctx context.Context, recordingID uuid.UUID) (uuid.UUID, json.RawMessage, error) {
	var projectID uuid.UUID
	var meta json.RawMessage
	err := s.pool.QueryRow(ctx, `SELECT project_id, metadata FROM recordings WHERE id = $1`, recordingID).
		Scan(&projectID, &meta)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil, domain.ErrNotFound
	}
	return projectID, meta, err
}

func (s *Store) GetReportByRecording(ctx context.Context, recordingID uuid.UUID) (domain.Report, error) {
	const q = `
		SELECT id, recording_id, project_id, status, title, summary, steps, ai_status, prompt_version, created_at, updated_at
		FROM reports WHERE recording_id = $1`
	var out domain.Report
	var status string
	err := s.pool.QueryRow(ctx, q, recordingID).Scan(
		&out.ID, &out.RecordingID, &out.ProjectID, &status,
		&out.Title, &out.Summary, &out.Steps, &out.AIStatus, &out.PromptVersion,
		&out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Report{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Report{}, err
	}
	out.Status = domain.ReportStatus(status)
	return out, nil
}

type scannable interface{ Scan(dest ...any) error }

func scanAnalysis(row scannable) (domain.Analysis, error) {
	var out domain.Analysis
	var status string
	err := row.Scan(
		&out.ID, &out.RecordingID, &out.ProjectID, &out.PromptVersion, &status,
		&out.Title, &out.Summary, &out.Steps, &out.Provider, &out.Model, &out.ErrorMessage,
		&out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Analysis{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Analysis{}, err
	}
	out.Status = domain.AnalysisStatus(status)
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
