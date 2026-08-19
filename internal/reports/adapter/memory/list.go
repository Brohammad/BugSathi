package memory

import (
	"context"
	"sort"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/reports/domain"
	"github.com/google/uuid"
)

func (r *Repo) ListByProject(_ context.Context, projectID uuid.UUID, page pagination.Page) (pagination.Result[domain.Report], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var matches []domain.Report
	for _, d := range r.reports {
		if d.Report.ProjectID == projectID {
			matches = append(matches, d.Report)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].CreatedAt.Equal(matches[j].CreatedAt) {
			return matches[i].ID.String() > matches[j].ID.String()
		}
		return matches[i].CreatedAt.After(matches[j].CreatedAt)
	})
	if page.Cursor != "" {
		at, id, err := pagination.DecodeCursor(page.Cursor)
		if err != nil {
			return pagination.Result[domain.Report]{}, err
		}
		filtered := matches[:0]
		for _, rep := range matches {
			if rep.CreatedAt.Before(at) || (rep.CreatedAt.Equal(at) && rep.ID.String() < id.String()) {
				filtered = append(filtered, rep)
			}
		}
		matches = filtered
	}
	return pagination.TrimPage(page, matches, func(rep domain.Report) (time.Time, uuid.UUID) {
		return rep.CreatedAt, rep.ID
	}), nil
}
