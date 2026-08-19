package memory

import (
	"context"
	"sync"

	"github.com/Brohammad/BugSathi/internal/projects/domain"
	"github.com/google/uuid"
)

type Repo struct {
	mu       sync.Mutex
	projects map[uuid.UUID]domain.Project
	members  map[uuid.UUID]map[uuid.UUID]domain.Member // project -> user -> member
}

func NewRepo() *Repo {
	return &Repo{
		projects: make(map[uuid.UUID]domain.Project),
		members:  make(map[uuid.UUID]map[uuid.UUID]domain.Member),
	}
}

func (r *Repo) CreateWithOwner(_ context.Context, project domain.Project, owner domain.Member) (domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.projects[project.ID] = project
	r.members[project.ID] = map[uuid.UUID]domain.Member{owner.UserID: owner}
	return project, nil
}

func (r *Repo) GetByID(_ context.Context, id uuid.UUID) (domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.projects[id]
	if !ok {
		return domain.Project{}, domain.ErrNotFound
	}
	return p, nil
}

func (r *Repo) Update(_ context.Context, project domain.Project) (domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.projects[project.ID]; !ok {
		return domain.Project{}, domain.ErrNotFound
	}
	r.projects[project.ID] = project
	return project, nil
}

func (r *Repo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.projects[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.projects, id)
	delete(r.members, id)
	return nil
}

func (r *Repo) GetMembership(_ context.Context, projectID, userID uuid.UUID) (domain.Member, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ms, ok := r.members[projectID]
	if !ok {
		return domain.Member{}, domain.ErrNotFound
	}
	m, ok := ms[userID]
	if !ok {
		return domain.Member{}, domain.ErrNotFound
	}
	return m, nil
}

func (r *Repo) AddMember(_ context.Context, member domain.Member) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ms, ok := r.members[member.ProjectID]
	if !ok {
		return domain.ErrNotFound
	}
	if _, exists := ms[member.UserID]; exists {
		return domain.ErrAlreadyMember
	}
	ms[member.UserID] = member
	return nil
}

func (r *Repo) CountOwners(_ context.Context, projectID uuid.UUID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ms, ok := r.members[projectID]
	if !ok {
		return 0, domain.ErrNotFound
	}
	n := 0
	for _, m := range ms {
		if m.Role == domain.RoleOwner {
			n++
		}
	}
	return n, nil
}
