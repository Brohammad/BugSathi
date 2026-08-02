package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Brohammad/BugSathi/internal/reports/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]domain.Report, error) {
	const q = `
		SELECT id, recording_id, project_id, status, title, summary, steps, ai_status, prompt_version, created_at, updated_at
		FROM reports WHERE project_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Report
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rep)
	}
	return out, rows.Err()
}

func (r *Repo) GetByID(ctx context.Context, projectID, reportID uuid.UUID) (domain.Detail, error) {
	const q = `
		SELECT rep.id, rep.recording_id, rep.project_id, rep.status, rep.title, rep.summary, rep.steps,
			rep.ai_status, rep.prompt_version, rep.created_at, rep.updated_at,
			rec.status, rec.metadata
		FROM reports rep
		INNER JOIN recordings rec ON rec.id = rep.recording_id
		WHERE rep.id = $1 AND rep.project_id = $2`
	return r.loadDetail(ctx, q, reportID, projectID)
}

func (r *Repo) GetByRecordingID(ctx context.Context, projectID, recordingID uuid.UUID) (domain.Detail, error) {
	const q = `
		SELECT rep.id, rep.recording_id, rep.project_id, rep.status, rep.title, rep.summary, rep.steps,
			rep.ai_status, rep.prompt_version, rep.created_at, rep.updated_at,
			rec.status, rec.metadata
		FROM reports rep
		INNER JOIN recordings rec ON rec.id = rep.recording_id
		WHERE rep.recording_id = $1 AND rep.project_id = $2`
	return r.loadDetail(ctx, q, recordingID, projectID)
}

func (r *Repo) loadDetail(ctx context.Context, q string, id, projectID uuid.UUID) (domain.Detail, error) {
	var d domain.Detail
	var status, recStatus string
	err := r.pool.QueryRow(ctx, q, id, projectID).Scan(
		&d.Report.ID, &d.Report.RecordingID, &d.Report.ProjectID, &status,
		&d.Report.Title, &d.Report.Summary, &d.Report.Steps, &d.Report.AIStatus, &d.Report.PromptVersion,
		&d.Report.CreatedAt, &d.Report.UpdatedAt, &recStatus, &d.Metadata,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Detail{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Detail{}, err
	}
	d.Report.Status = domain.Status(status)
	d.RecordingStatus = recStatus

	frames, thumb, err := r.loadFrames(ctx, d.Report.RecordingID)
	if err != nil {
		return domain.Detail{}, err
	}
	d.Frames = frames
	d.ThumbURL = thumb
	if d.Metadata == nil {
		d.Metadata = json.RawMessage(`{}`)
	}
	return d, nil
}

func (r *Repo) loadFrames(ctx context.Context, recordingID uuid.UUID) ([]domain.Frame, string, error) {
	const q = `
		SELECT kind, storage_key, ordinal, content_type, byte_size
		FROM media_artifacts WHERE recording_id = $1 ORDER BY kind, ordinal`
	rows, err := r.pool.Query(ctx, q, recordingID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var frames []domain.Frame
	thumbKey := ""
	for rows.Next() {
		var kind, key, ct string
		var ordinal int
		var size *int64
		if err := rows.Scan(&kind, &key, &ordinal, &ct, &size); err != nil {
			return nil, "", err
		}
		if kind == "thumb" {
			thumbKey = key
			continue
		}
		frames = append(frames, domain.Frame{
			Ordinal: ordinal, StorageKey: key, ContentType: ct, ByteSize: size,
		})
	}
	return frames, thumbKey, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanReport(row rowScanner) (domain.Report, error) {
	var out domain.Report
	var status string
	err := row.Scan(
		&out.ID, &out.RecordingID, &out.ProjectID, &status, &out.Title, &out.Summary, &out.Steps,
		&out.AIStatus, &out.PromptVersion, &out.CreatedAt, &out.UpdatedAt,
	)
	out.Status = domain.Status(status)
	return out, err
}
