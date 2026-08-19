package memory

import (
	"context"
	"sync"

	"github.com/Brohammad/BugSathi/internal/collab/domain"
	"github.com/Brohammad/BugSathi/internal/collab/port"
	"github.com/google/uuid"
)

type Repo struct {
	mu       sync.Mutex
	comments []domain.Comment
}

func NewRepo() *Repo { return &Repo{} }

func (r *Repo) Create(_ context.Context, c domain.Comment, _, _ string, _ []byte, _ string) (domain.Comment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.comments = append(r.comments, c)
	return c, nil
}

type AccessOK struct{}

func (AccessOK) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type AccessDeny struct{}

func (AccessDeny) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error {
	return domain.ErrForbidden
}

type ReportOK struct{}

func (ReportOK) EnsureInProject(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type Authors map[uuid.UUID]string

func (a Authors) DisplayName(_ context.Context, userID uuid.UUID) (string, error) {
	if n, ok := a[userID]; ok {
		return n, nil
	}
	return "unknown", nil
}

var (
	_ port.Repository    = (*Repo)(nil)
	_ port.ProjectAccess = AccessOK{}
	_ port.ReportGuard   = ReportOK{}
	_ port.AuthorLookup  = Authors{}
)
