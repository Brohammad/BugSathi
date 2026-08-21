package memory

import (
	"context"
	"sync"
	"time"

	"github.com/Brohammad/BugSathi/internal/sharing/domain"
	"github.com/Brohammad/BugSathi/internal/sharing/port"
	"github.com/google/uuid"
)

type Repo struct {
	mu    sync.Mutex
	byID  map[uuid.UUID]domain.ShareLink
	token map[string]uuid.UUID
}

func NewRepo() *Repo {
	return &Repo{byID: map[uuid.UUID]domain.ShareLink{}, token: map[string]uuid.UUID{}}
}

func (r *Repo) Create(_ context.Context, link domain.ShareLink, _, _ string, _ []byte, _ string) (domain.ShareLink, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[link.ID] = link
	r.token[link.Token] = link.ID
	return link, nil
}

func (r *Repo) GetByID(_ context.Context, projectID, shareID uuid.UUID) (domain.ShareLink, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byID[shareID]
	if !ok || s.ProjectID != projectID {
		return domain.ShareLink{}, domain.ErrNotFound
	}
	return s, nil
}

func (r *Repo) Revoke(_ context.Context, projectID, shareID uuid.UUID, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byID[shareID]
	if !ok || s.ProjectID != projectID {
		return domain.ErrNotFound
	}
	s.RevokedAt = &at
	r.byID[shareID] = s
	return nil
}

func (r *Repo) GetByToken(_ context.Context, token string) (domain.ShareLink, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.token[token]
	if !ok {
		return domain.ShareLink{}, domain.ErrNotFound
	}
	return r.byID[id], nil
}

type AccessOK struct{}

func (AccessOK) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (AccessOK) EnsureOwner(context.Context, uuid.UUID, uuid.UUID) error  { return nil }

type AccessMemberOnly struct{}

func (AccessMemberOnly) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (AccessMemberOnly) EnsureOwner(context.Context, uuid.UUID, uuid.UUID) error {
	return domain.ErrForbidden
}

type Reports struct {
	payload port.PublicReport
	err     error
}

func NewReports(p port.PublicReport) *Reports { return &Reports{payload: p} }

func (r *Reports) GetPublicPayload(context.Context, uuid.UUID, uuid.UUID) (port.PublicReport, error) {
	if r.err != nil {
		return port.PublicReport{}, r.err
	}
	return r.payload, nil
}

type Signer struct{}

func (Signer) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://cdn.test/" + key, nil
}
