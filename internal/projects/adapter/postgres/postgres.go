package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/projects/domain"
	"github.com/Brohammad/BugSathi/internal/projects/port"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) CreateWithOwner(ctx context.Context, project domain.Project, owner domain.Member) (domain.Project, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Project{}, err
	}
	defer tx.Rollback(ctx)

	const insertProject = `
		INSERT INTO projects (id, name, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, created_by, created_at, updated_at`
	var out domain.Project
	if err := tx.QueryRow(ctx, insertProject,
		project.ID, project.Name, project.CreatedBy, project.CreatedAt, project.UpdatedAt,
	).Scan(&out.ID, &out.Name, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return domain.Project{}, err
	}

	const insertMember = `
		INSERT INTO project_members (project_id, user_id, role, created_at)
		VALUES ($1, $2, $3, $4)`
	if _, err := tx.Exec(ctx, insertMember, owner.ProjectID, owner.UserID, string(owner.Role), owner.CreatedAt); err != nil {
		return domain.Project{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Project{}, err
	}
	return out, nil
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (domain.Project, error) {
	const q = `SELECT id, name, created_by, created_at, updated_at FROM projects WHERE id = $1`
	var out domain.Project
	err := r.pool.QueryRow(ctx, q, id).Scan(&out.ID, &out.Name, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, domain.ErrNotFound
	}
	return out, err
}

func (r *Repo) ListForUser(ctx context.Context, userID uuid.UUID, page pagination.Page) (pagination.Result[port.ProjectWithRole], error) {
	limit := page.Limit + 1
	var rows pgx.Rows
	var err error
	if page.Cursor == "" {
		const q = `
			SELECT p.id, p.name, p.created_by, p.created_at, p.updated_at, pm.role
			FROM projects p
			INNER JOIN project_members pm ON pm.project_id = p.id
			WHERE pm.user_id = $1
			ORDER BY p.created_at DESC, p.id DESC
			LIMIT $2`
		rows, err = r.pool.Query(ctx, q, userID, limit)
	} else {
		at, id, err := pagination.DecodeCursor(page.Cursor)
		if err != nil {
			return pagination.Result[port.ProjectWithRole]{}, err
		}
		const q = `
			SELECT p.id, p.name, p.created_by, p.created_at, p.updated_at, pm.role
			FROM projects p
			INNER JOIN project_members pm ON pm.project_id = p.id
			WHERE pm.user_id = $1
			  AND (p.created_at, p.id) < ($2, $3)
			ORDER BY p.created_at DESC, p.id DESC
			LIMIT $4`
		rows, err = r.pool.Query(ctx, q, userID, at, id, limit)
	}
	if err != nil {
		return pagination.Result[port.ProjectWithRole]{}, err
	}
	defer rows.Close()
	var out []port.ProjectWithRole
	for rows.Next() {
		var item port.ProjectWithRole
		var role string
		if err := rows.Scan(
			&item.Project.ID, &item.Project.Name, &item.Project.CreatedBy,
			&item.Project.CreatedAt, &item.Project.UpdatedAt, &role,
		); err != nil {
			return pagination.Result[port.ProjectWithRole]{}, err
		}
		item.Role = domain.Role(role)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return pagination.Result[port.ProjectWithRole]{}, err
	}
	return pagination.TrimPage(page, out, func(item port.ProjectWithRole) (time.Time, uuid.UUID) {
		return item.Project.CreatedAt, item.Project.ID
	}), nil
}

func (r *Repo) Update(ctx context.Context, project domain.Project) (domain.Project, error) {
	const q = `
		UPDATE projects SET name = $2, updated_at = $3
		WHERE id = $1
		RETURNING id, name, created_by, created_at, updated_at`
	var out domain.Project
	err := r.pool.QueryRow(ctx, q, project.ID, project.Name, project.UpdatedAt).
		Scan(&out.ID, &out.Name, &out.CreatedBy, &out.CreatedAt, &out.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, domain.ErrNotFound
	}
	return out, err
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) GetMembership(ctx context.Context, projectID, userID uuid.UUID) (domain.Member, error) {
	const q = `
		SELECT project_id, user_id, role, created_at
		FROM project_members WHERE project_id = $1 AND user_id = $2`
	var out domain.Member
	var role string
	err := r.pool.QueryRow(ctx, q, projectID, userID).Scan(&out.ProjectID, &out.UserID, &role, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Member{}, domain.ErrNotFound
	}
	out.Role = domain.Role(role)
	return out, err
}

func (r *Repo) AddMember(ctx context.Context, member domain.Member) error {
	const q = `
		INSERT INTO project_members (project_id, user_id, role, created_at)
		VALUES ($1, $2, $3, $4)`
	_, err := r.pool.Exec(ctx, q, member.ProjectID, member.UserID, string(member.Role), member.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrAlreadyMember
		}
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return domain.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *Repo) RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error {
	const q = `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`
	ct, err := r.pool.Exec(ctx, q, projectID, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repo) ListMembers(ctx context.Context, projectID uuid.UUID, page pagination.Page) (pagination.Result[domain.Member], error) {
	limit := page.Limit + 1
	var rows pgx.Rows
	var err error
	if page.Cursor == "" {
		const q = `
			SELECT project_id, user_id, role, created_at
			FROM project_members WHERE project_id = $1
			ORDER BY created_at ASC, user_id ASC
			LIMIT $2`
		rows, err = r.pool.Query(ctx, q, projectID, limit)
	} else {
		at, id, err := pagination.DecodeCursor(page.Cursor)
		if err != nil {
			return pagination.Result[domain.Member]{}, err
		}
		const q = `
			SELECT project_id, user_id, role, created_at
			FROM project_members WHERE project_id = $1
			  AND (created_at, user_id) > ($2, $3)
			ORDER BY created_at ASC, user_id ASC
			LIMIT $4`
		rows, err = r.pool.Query(ctx, q, projectID, at, id, limit)
	}
	if err != nil {
		return pagination.Result[domain.Member]{}, err
	}
	defer rows.Close()
	var out []domain.Member
	for rows.Next() {
		var m domain.Member
		var role string
		if err := rows.Scan(&m.ProjectID, &m.UserID, &role, &m.CreatedAt); err != nil {
			return pagination.Result[domain.Member]{}, err
		}
		m.Role = domain.Role(role)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return pagination.Result[domain.Member]{}, err
	}
	return pagination.TrimPage(page, out, func(m domain.Member) (time.Time, uuid.UUID) {
		return m.CreatedAt, m.UserID
	}), nil
}

func (r *Repo) CountOwners(ctx context.Context, projectID uuid.UUID) (int, error) {
	const q = `SELECT COUNT(*) FROM project_members WHERE project_id = $1 AND role = 'owner'`
	var n int
	err := r.pool.QueryRow(ctx, q, projectID).Scan(&n)
	return n, err
}
