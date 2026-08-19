package port

import (
	"context"

	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/projects/domain"
	"github.com/google/uuid"
)

type Repository interface {
	CreateWithOwner(ctx context.Context, project domain.Project, owner domain.Member) (domain.Project, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Project, error)
	ListForUser(ctx context.Context, userID uuid.UUID, page pagination.Page) (pagination.Result[ProjectWithRole], error)
	Update(ctx context.Context, project domain.Project) (domain.Project, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetMembership(ctx context.Context, projectID, userID uuid.UUID) (domain.Member, error)
	AddMember(ctx context.Context, member domain.Member) error
	ListMembers(ctx context.Context, projectID uuid.UUID, page pagination.Page) (pagination.Result[domain.Member], error)
	CountOwners(ctx context.Context, projectID uuid.UUID) (int, error)
}

// ObjectStore deletes media for a project after the DB row is gone.
// Keys follow ADR 0003: projects/{project_id}/...
type ObjectStore interface {
	DeletePrefix(ctx context.Context, prefix string) error
}

type ProjectWithRole struct {
	Project domain.Project
	Role    domain.Role
}
