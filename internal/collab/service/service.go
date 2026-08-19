package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/collab/domain"
	"github.com/Brohammad/BugSathi/internal/collab/port"
	"github.com/google/uuid"
)

type Service struct {
	repo    port.Repository
	access  port.ProjectAccess
	reports port.ReportGuard
	authors port.AuthorLookup
	hub     port.Hub
	listCfg config.ListConfig
	now     func() time.Time
}

func New(repo port.Repository, access port.ProjectAccess, reports port.ReportGuard, authors port.AuthorLookup, hub port.Hub, listCfg config.ListConfig) *Service {
	return &Service{repo: repo, access: access, reports: reports, authors: authors, hub: hub, listCfg: listCfg, now: time.Now}
}

type CommentDTO struct {
	ID         uuid.UUID `json:"id"`
	ReportID   uuid.UUID `json:"report_id"`
	ProjectID  uuid.UUID `json:"project_id"`
	AuthorID   uuid.UUID `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *Service) CreateComment(ctx context.Context, userID, projectID, reportID uuid.UUID, body string) (CommentDTO, error) {
	if err := s.access.EnsureMember(ctx, userID, projectID); err != nil {
		return CommentDTO{}, domain.ErrForbidden
	}
	if err := s.reports.EnsureInProject(ctx, projectID, reportID); err != nil {
		return CommentDTO{}, err
	}
	norm, err := domain.NormalizeBody(body)
	if err != nil {
		return CommentDTO{}, err
	}
	name, err := s.authors.DisplayName(ctx, userID)
	if err != nil {
		return CommentDTO{}, err
	}
	now := s.now()
	c := domain.Comment{
		ID: uuid.New(), ReportID: reportID, ProjectID: projectID,
		AuthorID: userID, Body: norm, CreatedAt: now,
	}
	payload, _ := json.Marshal(domain.CommentCreatedEvent{
		SchemaVersion: 1,
		CommentID:     c.ID.String(),
		ReportID:      reportID.String(),
		ProjectID:     projectID.String(),
		AuthorID:      userID.String(),
		OccurredAt:    now.UTC(),
	})
	created, err := s.repo.Create(ctx, c, domain.TopicCommentCreated, reportID.String(), payload, "")
	if err != nil {
		return CommentDTO{}, err
	}
	dto := CommentDTO{
		ID: created.ID, ReportID: created.ReportID, ProjectID: created.ProjectID,
		AuthorID: created.AuthorID, AuthorName: name, Body: created.Body, CreatedAt: created.CreatedAt,
	}
	s.hub.Publish(reportID, port.StreamEvent{Type: domain.EventCommentCreated, Data: dto})
	return dto, nil
}

func (s *Service) ListComments(ctx context.Context, userID, projectID, reportID uuid.UUID, page pagination.Page) (pagination.Result[CommentDTO], error) {
	if err := s.access.EnsureMember(ctx, userID, projectID); err != nil {
		return pagination.Result[CommentDTO]{}, domain.ErrForbidden
	}
	if err := s.reports.EnsureInProject(ctx, projectID, reportID); err != nil {
		return pagination.Result[CommentDTO]{}, err
	}
	rows, err := s.repo.ListByReport(ctx, projectID, reportID, page)
	if err != nil {
		return pagination.Result[CommentDTO]{}, err
	}
	out := make([]CommentDTO, 0, len(rows.Items))
	for _, c := range rows.Items {
		name, _ := s.authors.DisplayName(ctx, c.AuthorID)
		out = append(out, CommentDTO{
			ID: c.ID, ReportID: c.ReportID, ProjectID: c.ProjectID,
			AuthorID: c.AuthorID, AuthorName: name, Body: c.Body, CreatedAt: c.CreatedAt,
		})
	}
	return pagination.Result[CommentDTO]{Items: out, NextCursor: rows.NextCursor}, nil
}

func (s *Service) ListLimits() pagination.Limits {
	return pagination.Limits{Default: s.listCfg.DefaultLimit, Max: s.listCfg.MaxLimit}
}

// Subscribe opens an SSE channel after authz. Caller must invoke unsubscribe.
func (s *Service) Subscribe(ctx context.Context, userID, projectID, reportID uuid.UUID) (<-chan port.StreamEvent, func(), error) {
	if err := s.access.EnsureMember(ctx, userID, projectID); err != nil {
		return nil, nil, domain.ErrForbidden
	}
	if err := s.reports.EnsureInProject(ctx, projectID, reportID); err != nil {
		return nil, nil, err
	}
	name, err := s.authors.DisplayName(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	ch, unsub := s.hub.Subscribe(reportID, userID, name)
	return ch, unsub, nil
}
