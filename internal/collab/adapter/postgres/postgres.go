package postgres

import (
	"context"
	"errors"

	"github.com/Brohammad/BugSathi/internal/collab/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

func (r *Repo) Create(ctx context.Context, c domain.Comment, outboxTopic, partitionKey string, payload []byte, corr string) (domain.Comment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Comment{}, err
	}
	defer tx.Rollback(ctx)

	const q = `
		INSERT INTO report_comments (id, report_id, project_id, author_id, body, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, report_id, project_id, author_id, body, created_at`
	out, err := scanComment(tx.QueryRow(ctx, q,
		c.ID, c.ReportID, c.ProjectID, c.AuthorID, c.Body, c.CreatedAt,
	))
	if err != nil {
		return domain.Comment{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox (topic, partition_key, payload, correlation_id)
		VALUES ($1,$2,$3,$4)`, outboxTopic, partitionKey, payload, corr); err != nil {
		return domain.Comment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Comment{}, err
	}
	return out, nil
}

func (r *Repo) ListByReport(ctx context.Context, projectID, reportID uuid.UUID) ([]domain.Comment, error) {
	const q = `
		SELECT id, report_id, project_id, author_id, body, created_at
		FROM report_comments
		WHERE project_id = $1 AND report_id = $2
		ORDER BY created_at ASC`
	rows, err := r.pool.Query(ctx, q, projectID, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

type ReportGuard struct {
	pool *pgxpool.Pool
}

func NewReportGuard(pool *pgxpool.Pool) *ReportGuard {
	return &ReportGuard{pool: pool}
}

func (g *ReportGuard) EnsureInProject(ctx context.Context, projectID, reportID uuid.UUID) error {
	var ok bool
	err := g.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM reports WHERE id = $1 AND project_id = $2)`, reportID, projectID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrNotFound
	}
	return nil
}

type AuthorLookup struct {
	pool *pgxpool.Pool
}

func NewAuthorLookup(pool *pgxpool.Pool) *AuthorLookup {
	return &AuthorLookup{pool: pool}
}

func (a *AuthorLookup) DisplayName(ctx context.Context, userID uuid.UUID) (string, error) {
	var name string
	err := a.pool.QueryRow(ctx, `SELECT name FROM users WHERE id = $1`, userID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return name, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanComment(row rowScanner) (domain.Comment, error) {
	var c domain.Comment
	err := row.Scan(&c.ID, &c.ReportID, &c.ProjectID, &c.AuthorID, &c.Body, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Comment{}, domain.ErrNotFound
	}
	return c, err
}
