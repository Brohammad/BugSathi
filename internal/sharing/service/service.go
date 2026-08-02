package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Brohammad/BugSathi/internal/sharing/domain"
	"github.com/Brohammad/BugSathi/internal/sharing/port"
	"github.com/google/uuid"
)

type Service struct {
	repo    port.Repository
	access  port.ProjectAccess
	reports port.ReportReader
	signer  port.URLSigner
	urlTTL  time.Duration
	now     func() time.Time
}

func New(repo port.Repository, access port.ProjectAccess, reports port.ReportReader, signer port.URLSigner) *Service {
	return &Service{
		repo: repo, access: access, reports: reports, signer: signer,
		urlTTL: 15 * time.Minute, now: time.Now,
	}
}

type ShareDTO struct {
	ID        uuid.UUID  `json:"id"`
	ReportID  uuid.UUID  `json:"report_id"`
	ProjectID uuid.UUID  `json:"project_id"`
	Token     string     `json:"token"`
	URLPath   string     `json:"url_path"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type PublicView struct {
	Title    string          `json:"title"`
	Summary  string          `json:"summary"`
	Steps    json.RawMessage `json:"steps"`
	Status   string          `json:"status"`
	Frames   []PublicFrame   `json:"frames"`
	ThumbURL string          `json:"thumb_url,omitempty"`
}

type PublicFrame struct {
	Ordinal     int    `json:"ordinal"`
	ContentType string `json:"content_type"`
	URL         string `json:"url,omitempty"`
}

func toDTO(s domain.ShareLink) ShareDTO {
	return ShareDTO{
		ID: s.ID, ReportID: s.ReportID, ProjectID: s.ProjectID, Token: s.Token,
		URLPath: "/s/" + s.Token, ExpiresAt: s.ExpiresAt, RevokedAt: s.RevokedAt, CreatedAt: s.CreatedAt,
	}
}

func (s *Service) Create(ctx context.Context, userID, projectID, reportID uuid.UUID, expiresIn *time.Duration) (ShareDTO, error) {
	if err := s.access.EnsureMember(ctx, userID, projectID); err != nil {
		return ShareDTO{}, domain.ErrForbidden
	}
	if _, err := s.reports.GetPublicPayload(ctx, projectID, reportID); err != nil {
		return ShareDTO{}, err
	}
	token, err := domain.NewToken()
	if err != nil {
		return ShareDTO{}, err
	}
	now := s.now()
	var exp *time.Time
	if expiresIn != nil && *expiresIn > 0 {
		t := now.Add(*expiresIn)
		exp = &t
	}
	link := domain.ShareLink{
		ID: uuid.New(), ReportID: reportID, ProjectID: projectID,
		Token: token, ExpiresAt: exp, CreatedBy: userID, CreatedAt: now,
	}
	payload, _ := json.Marshal(domain.ShareCreatedEvent{
		SchemaVersion: 1, ShareID: link.ID.String(), ReportID: reportID.String(),
		ExpiresAt: exp, OccurredAt: now.UTC(),
	})
	created, err := s.repo.Create(ctx, link, domain.TopicShareCreated, reportID.String(), payload, "")
	if err != nil {
		return ShareDTO{}, err
	}
	return toDTO(created), nil
}

func (s *Service) List(ctx context.Context, userID, projectID, reportID uuid.UUID) ([]ShareDTO, error) {
	if err := s.access.EnsureMember(ctx, userID, projectID); err != nil {
		return nil, domain.ErrForbidden
	}
	rows, err := s.repo.ListByReport(ctx, projectID, reportID)
	if err != nil {
		return nil, err
	}
	out := make([]ShareDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDTO(row))
	}
	return out, nil
}

func (s *Service) Revoke(ctx context.Context, userID, projectID, shareID uuid.UUID) error {
	if err := s.access.EnsureMember(ctx, userID, projectID); err != nil {
		return domain.ErrForbidden
	}
	return s.repo.Revoke(ctx, projectID, shareID, s.now())
}

func (s *Service) PublicGet(ctx context.Context, token string) (PublicView, error) {
	if token == "" {
		return PublicView{}, domain.ErrInvalidInput
	}
	link, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return PublicView{}, err
	}
	if !link.Active(s.now()) {
		return PublicView{}, domain.ErrShareInactive
	}
	payload, err := s.reports.GetPublicPayload(ctx, link.ProjectID, link.ReportID)
	if err != nil {
		return PublicView{}, err
	}
	view := PublicView{
		Title: payload.Title, Summary: payload.Summary, Steps: payload.Steps, Status: payload.Status,
	}
	for _, f := range payload.Frames {
		pf := PublicFrame{Ordinal: f.Ordinal, ContentType: f.ContentType}
		if s.signer != nil {
			if u, err := s.signer.PresignGet(ctx, f.StorageKey, s.urlTTL); err == nil {
				pf.URL = u
			}
		}
		view.Frames = append(view.Frames, pf)
	}
	if s.signer != nil && payload.ThumbKey != "" {
		if u, err := s.signer.PresignGet(ctx, payload.ThumbKey, s.urlTTL); err == nil {
			view.ThumbURL = u
		}
	}
	return view, nil
}
