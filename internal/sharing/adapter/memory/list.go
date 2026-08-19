package memory

import (
	"context"
	"sort"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/sharing/domain"
	"github.com/google/uuid"
)

func (r *Repo) ListByReport(_ context.Context, projectID, reportID uuid.UUID, page pagination.Page) (pagination.Result[domain.ShareLink], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var matches []domain.ShareLink
	for _, s := range r.byID {
		if s.ProjectID == projectID && s.ReportID == reportID {
			matches = append(matches, s)
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
			return pagination.Result[domain.ShareLink]{}, err
		}
		filtered := matches[:0]
		for _, s := range matches {
			if s.CreatedAt.Before(at) || (s.CreatedAt.Equal(at) && s.ID.String() < id.String()) {
				filtered = append(filtered, s)
			}
		}
		matches = filtered
	}
	return pagination.TrimPage(page, matches, func(s domain.ShareLink) (time.Time, uuid.UUID) {
		return s.CreatedAt, s.ID
	}), nil
}
