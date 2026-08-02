package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Brohammad/BugSathi/internal/projects/domain"
	"github.com/Brohammad/BugSathi/internal/projects/port"
	"github.com/google/uuid"
)

type Service struct {
	repo port.Repository
	now  func() time.Time
}

func New(repo port.Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

type ProjectDTO struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	CreatedBy uuid.UUID   `json:"created_by"`
	Role      domain.Role `json:"role,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type MemberDTO struct {
	UserID    uuid.UUID   `json:"user_id"`
	Role      domain.Role `json:"role"`
	CreatedAt time.Time   `json:"created_at"`
}

func toDTO(p domain.Project, role domain.Role) ProjectDTO {
	return ProjectDTO{
		ID: p.ID, Name: p.Name, CreatedBy: p.CreatedBy,
		Role: role, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, name string) (ProjectDTO, error) {
	p, err := domain.NewProject(name, userID, s.now())
	if err != nil {
		return ProjectDTO{}, err
	}
	owner := domain.Member{
		ProjectID: p.ID,
		UserID:    userID,
		Role:      domain.RoleOwner,
		CreatedAt: s.now(),
	}
	created, err := s.repo.CreateWithOwner(ctx, p, owner)
	if err != nil {
		return ProjectDTO{}, err
	}
	return toDTO(created, domain.RoleOwner), nil
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]ProjectDTO, error) {
	rows, err := s.repo.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDTO(r.Project, r.Role))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, userID, projectID uuid.UUID) (ProjectDTO, error) {
	m, err := s.requireMember(ctx, projectID, userID)
	if err != nil {
		return ProjectDTO{}, err
	}
	p, err := s.repo.GetByID(ctx, projectID)
	if err != nil {
		return ProjectDTO{}, err
	}
	return toDTO(p, m.Role), nil
}

func (s *Service) Update(ctx context.Context, userID, projectID uuid.UUID, name string) (ProjectDTO, error) {
	if _, err := s.requireOwner(ctx, projectID, userID); err != nil {
		return ProjectDTO{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ProjectDTO{}, domain.ErrInvalidName
	}
	p, err := s.repo.GetByID(ctx, projectID)
	if err != nil {
		return ProjectDTO{}, err
	}
	p.Name = name
	p.UpdatedAt = s.now()
	updated, err := s.repo.Update(ctx, p)
	if err != nil {
		return ProjectDTO{}, err
	}
	return toDTO(updated, domain.RoleOwner), nil
}

func (s *Service) Delete(ctx context.Context, userID, projectID uuid.UUID) error {
	if _, err := s.requireOwner(ctx, projectID, userID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, projectID)
}

func (s *Service) AddMember(ctx context.Context, actorID, projectID, memberUserID uuid.UUID, roleStr string) (MemberDTO, error) {
	if _, err := s.requireOwner(ctx, projectID, actorID); err != nil {
		return MemberDTO{}, err
	}
	role, err := domain.ParseRole(roleStr)
	if err != nil {
		return MemberDTO{}, err
	}
	if role == domain.RoleOwner {
		// MVP: only one path to owner is create; adding owners allowed but keep simple
	}
	m := domain.Member{
		ProjectID: projectID,
		UserID:    memberUserID,
		Role:      role,
		CreatedAt: s.now(),
	}
	if err := s.repo.AddMember(ctx, m); err != nil {
		return MemberDTO{}, err
	}
	return MemberDTO{UserID: m.UserID, Role: m.Role, CreatedAt: m.CreatedAt}, nil
}

func (s *Service) ListMembers(ctx context.Context, userID, projectID uuid.UUID) ([]MemberDTO, error) {
	if _, err := s.requireMember(ctx, projectID, userID); err != nil {
		return nil, err
	}
	members, err := s.repo.ListMembers(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberDTO, 0, len(members))
	for _, m := range members {
		out = append(out, MemberDTO{UserID: m.UserID, Role: m.Role, CreatedAt: m.CreatedAt})
	}
	return out, nil
}

func (s *Service) requireMember(ctx context.Context, projectID, userID uuid.UUID) (domain.Member, error) {
	m, err := s.repo.GetMembership(ctx, projectID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Member{}, domain.ErrForbidden
		}
		return domain.Member{}, err
	}
	return m, nil
}

func (s *Service) requireOwner(ctx context.Context, projectID, userID uuid.UUID) (domain.Member, error) {
	m, err := s.requireMember(ctx, projectID, userID)
	if err != nil {
		return domain.Member{}, err
	}
	if !m.Role.IsOwner() {
		return domain.Member{}, domain.ErrForbidden
	}
	return m, nil
}
