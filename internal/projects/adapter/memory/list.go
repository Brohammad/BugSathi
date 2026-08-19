package memory

import (
	"context"
	"sort"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/projects/domain"
	"github.com/Brohammad/BugSathi/internal/projects/port"
	"github.com/google/uuid"
)

func (r *Repo) ListForUser(_ context.Context, userID uuid.UUID, page pagination.Page) (pagination.Result[port.ProjectWithRole], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var matches []port.ProjectWithRole
	for pid, ms := range r.members {
		if m, ok := ms[userID]; ok {
			matches = append(matches, port.ProjectWithRole{Project: r.projects[pid], Role: m.Role})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Project.CreatedAt.Equal(matches[j].Project.CreatedAt) {
			return matches[i].Project.ID.String() > matches[j].Project.ID.String()
		}
		return matches[i].Project.CreatedAt.After(matches[j].Project.CreatedAt)
	})
	if page.Cursor != "" {
		at, id, err := pagination.DecodeCursor(page.Cursor)
		if err != nil {
			return pagination.Result[port.ProjectWithRole]{}, err
		}
		filtered := matches[:0]
		for _, item := range matches {
			p := item.Project
			if p.CreatedAt.Before(at) || (p.CreatedAt.Equal(at) && p.ID.String() < id.String()) {
				filtered = append(filtered, item)
			}
		}
		matches = filtered
	}
	return pagination.TrimPage(page, matches, func(item port.ProjectWithRole) (time.Time, uuid.UUID) {
		return item.Project.CreatedAt, item.Project.ID
	}), nil
}

func (r *Repo) ListMembers(_ context.Context, projectID uuid.UUID, page pagination.Page) (pagination.Result[domain.Member], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ms, ok := r.members[projectID]
	if !ok {
		return pagination.Result[domain.Member]{}, domain.ErrNotFound
	}
	matches := make([]domain.Member, 0, len(ms))
	for _, m := range ms {
		matches = append(matches, m)
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].CreatedAt.Equal(matches[j].CreatedAt) {
			return matches[i].UserID.String() < matches[j].UserID.String()
		}
		return matches[i].CreatedAt.Before(matches[j].CreatedAt)
	})
	if page.Cursor != "" {
		at, id, err := pagination.DecodeCursor(page.Cursor)
		if err != nil {
			return pagination.Result[domain.Member]{}, err
		}
		filtered := matches[:0]
		for _, m := range matches {
			if m.CreatedAt.After(at) || (m.CreatedAt.Equal(at) && m.UserID.String() > id.String()) {
				filtered = append(filtered, m)
			}
		}
		matches = filtered
	}
	return pagination.TrimPage(page, matches, func(m domain.Member) (time.Time, uuid.UUID) {
		return m.CreatedAt, m.UserID
	}), nil
}
