package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidName   = errors.New("project name is required")
	ErrNotFound      = errors.New("project not found")
	ErrForbidden     = errors.New("forbidden")
	ErrAlreadyMember = errors.New("user is already a member")
	ErrInvalidRole   = errors.New("invalid role")
	ErrLastOwner     = errors.New("cannot remove the last owner")
)

type Role string

const (
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
)

func ParseRole(s string) (Role, error) {
	switch Role(strings.ToLower(strings.TrimSpace(s))) {
	case RoleOwner:
		return RoleOwner, nil
	case RoleMember:
		return RoleMember, nil
	default:
		return "", ErrInvalidRole
	}
}

func (r Role) IsOwner() bool { return r == RoleOwner }

type Project struct {
	ID        uuid.UUID
	Name      string
	CreatedBy uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewProject(name string, createdBy uuid.UUID, now time.Time) (Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Project{}, ErrInvalidName
	}
	return Project{
		ID:        uuid.New(),
		Name:      name,
		CreatedBy: createdBy,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

type Member struct {
	ProjectID uuid.UUID
	UserID    uuid.UUID
	Role      Role
	CreatedAt time.Time
}
