package access

import (
	"context"
	"errors"

	projectdomain "github.com/Brohammad/BugSathi/internal/projects/domain"
	"github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/google/uuid"
)

type ProjectGuard interface {
	EnsureMember(ctx context.Context, userID, projectID uuid.UUID) error
	EnsureOwner(ctx context.Context, userID, projectID uuid.UUID) error
}

type Adapter struct {
	guard ProjectGuard
}

func New(guard ProjectGuard) *Adapter {
	return &Adapter{guard: guard}
}

func (a *Adapter) EnsureMember(ctx context.Context, userID, projectID uuid.UUID) error {
	err := a.guard.EnsureMember(ctx, userID, projectID)
	if err == nil {
		return nil
	}
	if errors.Is(err, projectdomain.ErrForbidden) || errors.Is(err, projectdomain.ErrNotFound) {
		return domain.ErrForbidden
	}
	return err
}

func (a *Adapter) EnsureOwner(ctx context.Context, userID, projectID uuid.UUID) error {
	err := a.guard.EnsureOwner(ctx, userID, projectID)
	if err == nil {
		return nil
	}
	if errors.Is(err, projectdomain.ErrForbidden) || errors.Is(err, projectdomain.ErrNotFound) {
		return domain.ErrForbidden
	}
	return err
}
