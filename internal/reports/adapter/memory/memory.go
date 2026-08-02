package memory

import (
	"context"
	"sync"
	"time"

	"github.com/Brohammad/BugSathi/internal/reports/domain"
	"github.com/google/uuid"
)

type Repo struct {
	mu      sync.Mutex
	reports map[uuid.UUID]domain.Detail
}

func NewRepo() *Repo {
	return &Repo{reports: map[uuid.UUID]domain.Detail{}}
}

func (r *Repo) Seed(d domain.Detail) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reports[d.Report.ID] = d
}

func (r *Repo) ListByProject(_ context.Context, projectID uuid.UUID) ([]domain.Report, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Report
	for _, d := range r.reports {
		if d.Report.ProjectID == projectID {
			out = append(out, d.Report)
		}
	}
	return out, nil
}

func (r *Repo) GetByID(_ context.Context, projectID, reportID uuid.UUID) (domain.Detail, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.reports[reportID]
	if !ok || d.Report.ProjectID != projectID {
		return domain.Detail{}, domain.ErrNotFound
	}
	return d, nil
}

func (r *Repo) GetByRecordingID(_ context.Context, projectID, recordingID uuid.UUID) (domain.Detail, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range r.reports {
		if d.Report.ProjectID == projectID && d.Report.RecordingID == recordingID {
			return d, nil
		}
	}
	return domain.Detail{}, domain.ErrNotFound
}

type AccessOK struct{}

func (AccessOK) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type AccessDeny struct{}

func (AccessDeny) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error {
	return domain.ErrForbidden
}

type Signer struct{}

func (Signer) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://example.test/" + key, nil
}
