package memory

import (
	"context"
	"sort"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/collab/domain"
	"github.com/google/uuid"
)

func (r *Repo) ListByReport(_ context.Context, projectID, reportID uuid.UUID, page pagination.Page) (pagination.Result[domain.Comment], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var matches []domain.Comment
	for _, c := range r.comments {
		if c.ProjectID == projectID && c.ReportID == reportID {
			matches = append(matches, c)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].CreatedAt.Equal(matches[j].CreatedAt) {
			return matches[i].ID.String() < matches[j].ID.String()
		}
		return matches[i].CreatedAt.Before(matches[j].CreatedAt)
	})
	if page.Cursor != "" {
		at, id, err := pagination.DecodeCursor(page.Cursor)
		if err != nil {
			return pagination.Result[domain.Comment]{}, err
		}
		filtered := matches[:0]
		for _, c := range matches {
			if c.CreatedAt.After(at) || (c.CreatedAt.Equal(at) && c.ID.String() > id.String()) {
				filtered = append(filtered, c)
			}
		}
		matches = filtered
	}
	return pagination.TrimPage(page, matches, func(c domain.Comment) (time.Time, uuid.UUID) {
		return c.CreatedAt, c.ID
	}), nil
}
