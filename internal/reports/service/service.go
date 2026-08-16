package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Brohammad/BugSathi/internal/reports/domain"
	"github.com/Brohammad/BugSathi/internal/reports/port"
	"github.com/google/uuid"
)

type Service struct {
	repo   port.Repository
	access port.ProjectAccess
	signer port.URLSigner
	urlTTL time.Duration
	cache  DetailCache
}

// DetailCache stores report aggregates without presigned URLs.
type DetailCache interface {
	Enabled() bool
	Get(id uuid.UUID) (domain.Detail, bool)
	Set(id uuid.UUID, d domain.Detail)
}

func New(repo port.Repository, access port.ProjectAccess, signer port.URLSigner, cache DetailCache) *Service {
	return &Service{repo: repo, access: access, signer: signer, urlTTL: 15 * time.Minute, cache: cache}
}

type ReportDTO struct {
	ID            uuid.UUID       `json:"id"`
	RecordingID   uuid.UUID       `json:"recording_id"`
	ProjectID     uuid.UUID       `json:"project_id"`
	Status        domain.Status   `json:"status"`
	Title         string          `json:"title"`
	Summary       string          `json:"summary"`
	Steps         json.RawMessage `json:"steps"`
	AIStatus      string          `json:"ai_status"`
	PromptVersion string          `json:"prompt_version"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type FrameDTO struct {
	Ordinal     int    `json:"ordinal"`
	StorageKey  string `json:"storage_key"`
	ContentType string `json:"content_type"`
	ByteSize    *int64 `json:"byte_size,omitempty"`
	URL         string `json:"url,omitempty"`
}

type DetailDTO struct {
	Report          ReportDTO       `json:"report"`
	RecordingStatus string          `json:"recording_status"`
	Metadata        json.RawMessage `json:"metadata"`
	Frames          []FrameDTO      `json:"frames"`
	ThumbURL        string          `json:"thumb_url,omitempty"`
}

func toReportDTO(r domain.Report) ReportDTO {
	return ReportDTO{
		ID: r.ID, RecordingID: r.RecordingID, ProjectID: r.ProjectID, Status: r.Status,
		Title: r.Title, Summary: r.Summary, Steps: r.Steps, AIStatus: r.AIStatus,
		PromptVersion: r.PromptVersion, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (s *Service) List(ctx context.Context, userID, projectID uuid.UUID) ([]ReportDTO, error) {
	if err := s.access.EnsureMember(ctx, userID, projectID); err != nil {
		return nil, mapAccess(err)
	}
	rows, err := s.repo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]ReportDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, toReportDTO(r))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, userID, projectID, reportID uuid.UUID) (DetailDTO, error) {
	if err := s.access.EnsureMember(ctx, userID, projectID); err != nil {
		return DetailDTO{}, mapAccess(err)
	}
	if s.cache != nil && s.cache.Enabled() {
		if d, ok := s.cache.Get(reportID); ok && d.Report.ProjectID == projectID {
			return s.withURLs(ctx, d)
		}
	}
	d, err := s.repo.GetByID(ctx, projectID, reportID)
	if err != nil {
		return DetailDTO{}, err
	}
	if s.cache != nil {
		s.cache.Set(reportID, d)
	}
	return s.withURLs(ctx, d)
}

func (s *Service) GetByRecording(ctx context.Context, userID, projectID, recordingID uuid.UUID) (DetailDTO, error) {
	if err := s.access.EnsureMember(ctx, userID, projectID); err != nil {
		return DetailDTO{}, mapAccess(err)
	}
	d, err := s.repo.GetByRecordingID(ctx, projectID, recordingID)
	if err != nil {
		return DetailDTO{}, err
	}
	if s.cache != nil {
		s.cache.Set(d.Report.ID, d)
	}
	return s.withURLs(ctx, d)
}

func (s *Service) withURLs(ctx context.Context, d domain.Detail) (DetailDTO, error) {
	frames := make([]FrameDTO, 0, len(d.Frames))
	for _, f := range d.Frames {
		dto := FrameDTO{
			Ordinal: f.Ordinal, StorageKey: f.StorageKey,
			ContentType: f.ContentType, ByteSize: f.ByteSize,
		}
		if s.signer != nil && f.StorageKey != "" {
			if u, err := s.signer.PresignGet(ctx, f.StorageKey, s.urlTTL); err == nil {
				dto.URL = u
			}
		}
		frames = append(frames, dto)
	}
	thumb := ""
	if s.signer != nil && d.ThumbURL != "" {
		if u, err := s.signer.PresignGet(ctx, d.ThumbURL, s.urlTTL); err == nil {
			thumb = u
		}
	}
	return DetailDTO{
		Report: toReportDTO(d.Report), RecordingStatus: d.RecordingStatus,
		Metadata: d.Metadata, Frames: frames, ThumbURL: thumb,
	}, nil
}

func mapAccess(err error) error {
	if err == nil {
		return nil
	}
	// uploads access adapter maps to uploads/domain; projects maps to projects/domain.
	// HTTP layer treats any EnsureMember failure as forbidden unless already typed.
	return domain.ErrForbidden
}
