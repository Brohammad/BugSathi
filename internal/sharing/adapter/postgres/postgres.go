package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/sharing/domain"
	"github.com/Brohammad/BugSathi/internal/sharing/port"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) Create(ctx context.Context, link domain.ShareLink, outboxTopic, partitionKey string, payload []byte, corr string) (domain.ShareLink, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.ShareLink{}, err
	}
	defer tx.Rollback(ctx)

	const q = `
		INSERT INTO share_links (id, report_id, project_id, token, expires_at, revoked_at, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, report_id, project_id, token, expires_at, revoked_at, created_by, created_at`
	out, err := scanShare(tx.QueryRow(ctx, q,
		link.ID, link.ReportID, link.ProjectID, link.Token, link.ExpiresAt, link.RevokedAt, link.CreatedBy, link.CreatedAt,
	))
	if err != nil {
		return domain.ShareLink{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox (topic, partition_key, payload, correlation_id)
		VALUES ($1,$2,$3,$4)`, outboxTopic, partitionKey, payload, corr); err != nil {
		return domain.ShareLink{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ShareLink{}, err
	}
	return out, nil
}

func (r *Repo) ListByReport(ctx context.Context, projectID, reportID uuid.UUID, page pagination.Page) (pagination.Result[domain.ShareLink], error) {
	limit := page.Limit + 1
	var (
		rows pgx.Rows
		err  error
	)
	if page.Cursor == "" {
		const q = `
			SELECT id, report_id, project_id, token, expires_at, revoked_at, created_by, created_at
			FROM share_links
			WHERE project_id = $1 AND report_id = $2
			ORDER BY created_at DESC, id DESC
			LIMIT $3`
		rows, err = r.pool.Query(ctx, q, projectID, reportID, limit)
	} else {
		at, id, err := pagination.DecodeCursor(page.Cursor)
		if err != nil {
			return pagination.Result[domain.ShareLink]{}, err
		}
		const q = `
			SELECT id, report_id, project_id, token, expires_at, revoked_at, created_by, created_at
			FROM share_links
			WHERE project_id = $1 AND report_id = $2
			  AND (created_at, id) < ($3, $4)
			ORDER BY created_at DESC, id DESC
			LIMIT $5`
		rows, err = r.pool.Query(ctx, q, projectID, reportID, at, id, limit)
	}
	if err != nil {
		return pagination.Result[domain.ShareLink]{}, err
	}
	defer rows.Close()
	var out []domain.ShareLink
	for rows.Next() {
		s, err := scanShare(rows)
		if err != nil {
			return pagination.Result[domain.ShareLink]{}, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return pagination.Result[domain.ShareLink]{}, err
	}
	return pagination.TrimPage(page, out, func(s domain.ShareLink) (time.Time, uuid.UUID) {
		return s.CreatedAt, s.ID
	}), nil
}

func (r *Repo) GetByID(ctx context.Context, projectID, shareID uuid.UUID) (domain.ShareLink, error) {
	const q = `
		SELECT id, report_id, project_id, token, expires_at, revoked_at, created_by, created_at
		FROM share_links WHERE id = $1 AND project_id = $2`
	return scanShare(r.pool.QueryRow(ctx, q, shareID, projectID))
}

func (r *Repo) Revoke(ctx context.Context, projectID, shareID uuid.UUID, at time.Time) error {
	ct, err := r.pool.Exec(ctx, `
		UPDATE share_links SET revoked_at = $3
		WHERE id = $1 AND project_id = $2 AND revoked_at IS NULL`, shareID, projectID, at)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) GetByToken(ctx context.Context, token string) (domain.ShareLink, error) {
	const q = `
		SELECT id, report_id, project_id, token, expires_at, revoked_at, created_by, created_at
		FROM share_links WHERE token = $1`
	return scanShare(r.pool.QueryRow(ctx, q, token))
}

type ReportReader struct {
	pool *pgxpool.Pool
}

func NewReportReader(pool *pgxpool.Pool) *ReportReader {
	return &ReportReader{pool: pool}
}

func (r *ReportReader) GetPublicPayload(ctx context.Context, projectID, reportID uuid.UUID) (port.PublicReport, error) {
	const q = `
		SELECT id, project_id, status, title, summary, steps
		FROM reports WHERE id = $1 AND project_id = $2`
	var out port.PublicReport
	var status string
	err := r.pool.QueryRow(ctx, q, reportID, projectID).Scan(
		&out.ReportID, &out.ProjectID, &status, &out.Title, &out.Summary, &out.Steps,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return port.PublicReport{}, domain.ErrNotFound
	}
	if err != nil {
		return port.PublicReport{}, err
	}
	out.Status = status
	if status != "READY" {
		return port.PublicReport{}, domain.ErrReportNotReady
	}

	rows, err := r.pool.Query(ctx, `
		SELECT kind, storage_key, ordinal, content_type
		FROM media_artifacts
		WHERE recording_id = (SELECT recording_id FROM reports WHERE id = $1)
		ORDER BY kind, ordinal`, reportID)
	if err != nil {
		return port.PublicReport{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, key, ct string
		var ordinal int
		if err := rows.Scan(&kind, &key, &ordinal, &ct); err != nil {
			return port.PublicReport{}, err
		}
		if kind == "thumb" {
			out.ThumbKey = key
			continue
		}
		out.Frames = append(out.Frames, port.PublicFrame{
			Ordinal: ordinal, StorageKey: key, ContentType: ct,
		})
	}
	return out, rows.Err()
}

type scannable interface{ Scan(dest ...any) error }

func scanShare(row scannable) (domain.ShareLink, error) {
	var out domain.ShareLink
	err := row.Scan(
		&out.ID, &out.ReportID, &out.ProjectID, &out.Token, &out.ExpiresAt, &out.RevokedAt, &out.CreatedBy, &out.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ShareLink{}, domain.ErrNotFound
	}
	return out, err
}
