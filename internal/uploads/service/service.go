package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/logging"
	"github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/Brohammad/BugSathi/internal/uploads/port"
	"github.com/google/uuid"
)

type Service struct {
	recordings port.RecordingRepository
	storage    port.ObjectStorage
	access     port.ProjectAccess
	presignTTL time.Duration
	now        func() time.Time
}

func New(
	recordings port.RecordingRepository,
	storage port.ObjectStorage,
	access port.ProjectAccess,
	presignTTL time.Duration,
) *Service {
	if presignTTL <= 0 {
		presignTTL = 15 * time.Minute
	}
	return &Service{
		recordings: recordings,
		storage:    storage,
		access:     access,
		presignTTL: presignTTL,
		now:        time.Now,
	}
}

type CreateInput struct {
	ProjectID     uuid.UUID
	UserID        uuid.UUID
	ContentType   string
	Filename      string
	Metadata      json.RawMessage
	CorrelationID string
}

type CreateResult struct {
	Recording domain.Recording `json:"recording"`
	UploadURL string           `json:"upload_url"`
}

type RecordingDTO struct {
	ID            uuid.UUID       `json:"id"`
	ProjectID     uuid.UUID       `json:"project_id"`
	CreatedBy     uuid.UUID       `json:"created_by"`
	Status        domain.Status   `json:"status"`
	StorageKey    string          `json:"storage_key"`
	ContentType   string          `json:"content_type"`
	ByteSize      *int64          `json:"byte_size,omitempty"`
	Checksum      string          `json:"checksum,omitempty"`
	Metadata      json.RawMessage `json:"metadata"`
	CorrelationID string          `json:"correlation_id"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

func toDTO(r domain.Recording) RecordingDTO {
	return RecordingDTO{
		ID: r.ID, ProjectID: r.ProjectID, CreatedBy: r.CreatedBy,
		Status: r.Status, StorageKey: r.StorageKey, ContentType: r.ContentType,
		ByteSize: r.ByteSize, Checksum: r.Checksum, Metadata: r.Metadata,
		CorrelationID: r.CorrelationID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func RecordingDTOFrom(r domain.Recording) RecordingDTO { return toDTO(r) }

func (s *Service) Create(ctx context.Context, in CreateInput) (CreateResult, error) {
	if err := s.access.EnsureMember(ctx, in.UserID, in.ProjectID); err != nil {
		return CreateResult{}, err
	}
	ct := strings.TrimSpace(in.ContentType)
	if ct == "" {
		ct = "application/octet-stream"
	}
	ext := extensionFor(ct, in.Filename)
	id := uuid.New()
	key := fmt.Sprintf("projects/%s/recordings/%s/source%s", in.ProjectID, id, ext)
	corr := in.CorrelationID
	if corr == "" {
		corr = logging.CorrelationIDFromContext(ctx)
	}
	if corr == "" {
		corr = id.String()
	}
	meta := in.Metadata
	if len(meta) == 0 {
		meta = json.RawMessage(`{}`)
	}
	now := s.now()
	rec := domain.Recording{
		ID: id, ProjectID: in.ProjectID, CreatedBy: in.UserID,
		Status: domain.StatusUploading, StorageKey: key, ContentType: ct,
		Metadata: meta, CorrelationID: corr, CreatedAt: now, UpdatedAt: now,
	}
	created, err := s.recordings.Create(ctx, rec)
	if err != nil {
		return CreateResult{}, err
	}
	url, err := s.storage.PresignPut(ctx, key, ct, s.presignTTL)
	if err != nil {
		return CreateResult{}, err
	}
	return CreateResult{Recording: created, UploadURL: url}, nil
}

func (s *Service) Complete(ctx context.Context, userID, projectID, recordingID uuid.UUID) (RecordingDTO, error) {
	if err := s.access.EnsureMember(ctx, userID, projectID); err != nil {
		return RecordingDTO{}, err
	}
	rec, err := s.recordings.Get(ctx, recordingID)
	if err != nil {
		return RecordingDTO{}, err
	}
	if rec.ProjectID != projectID {
		return RecordingDTO{}, domain.ErrNotFound
	}
	if rec.IsUploadedOrBeyond() {
		return toDTO(rec), nil // idempotent
	}
	if rec.Status != domain.StatusUploading {
		return RecordingDTO{}, domain.ErrIllegalTransition
	}

	size, _, err := s.storage.Stat(ctx, rec.StorageKey)
	if err != nil {
		return RecordingDTO{}, domain.ErrObjectMissing
	}
	rec.ByteSize = &size
	if err := rec.Transition(domain.StatusUploaded, s.now()); err != nil {
		return RecordingDTO{}, err
	}

	payload, err := json.Marshal(domain.RecordingUploadedEvent{
		SchemaVersion: 1,
		RecordingID:   rec.ID.String(),
		ProjectID:     rec.ProjectID.String(),
		ObjectKey:     rec.StorageKey,
		ContentType:   rec.ContentType,
		ByteSize:      size,
		Checksum:      rec.Checksum,
		Metadata:      rec.Metadata,
		CorrelationID: rec.CorrelationID,
		OccurredAt:    s.now().UTC(),
	})
	if err != nil {
		return RecordingDTO{}, err
	}

	updated, err := s.recordings.CompleteWithOutbox(
		ctx, rec, domain.TopicRecordingUploaded, rec.ID.String(), payload, rec.CorrelationID,
	)
	if err != nil {
		return RecordingDTO{}, err
	}
	return toDTO(updated), nil
}

func (s *Service) Get(ctx context.Context, userID, projectID, recordingID uuid.UUID) (RecordingDTO, error) {
	if err := s.access.EnsureMember(ctx, userID, projectID); err != nil {
		return RecordingDTO{}, err
	}
	rec, err := s.recordings.Get(ctx, recordingID)
	if err != nil {
		return RecordingDTO{}, err
	}
	if rec.ProjectID != projectID {
		return RecordingDTO{}, domain.ErrNotFound
	}
	return toDTO(rec), nil
}

// Reprocess re-emits RecordingUploaded via the outbox so media/AI can run again.
// Owner-only. Allowed for FAILED, UPLOADED, PROCESSING, or READY.
func (s *Service) Reprocess(ctx context.Context, userID, projectID, recordingID uuid.UUID) (RecordingDTO, error) {
	if err := s.access.EnsureOwner(ctx, userID, projectID); err != nil {
		return RecordingDTO{}, err
	}
	rec, err := s.recordings.Get(ctx, recordingID)
	if err != nil {
		return RecordingDTO{}, err
	}
	if rec.ProjectID != projectID {
		return RecordingDTO{}, domain.ErrNotFound
	}
	switch rec.Status {
	case domain.StatusFailed, domain.StatusUploaded, domain.StatusProcessing, domain.StatusReady:
		// ok
	default:
		return RecordingDTO{}, domain.ErrIllegalTransition
	}
	if rec.ByteSize == nil {
		return RecordingDTO{}, domain.ErrObjectMissing
	}

	payload, err := json.Marshal(domain.RecordingUploadedEvent{
		SchemaVersion: 1,
		RecordingID:   rec.ID.String(),
		ProjectID:     rec.ProjectID.String(),
		ObjectKey:     rec.StorageKey,
		ContentType:   rec.ContentType,
		ByteSize:      *rec.ByteSize,
		Checksum:      rec.Checksum,
		Metadata:      rec.Metadata,
		CorrelationID: rec.CorrelationID,
		OccurredAt:    s.now().UTC(),
	})
	if err != nil {
		return RecordingDTO{}, err
	}
	if err := s.recordings.InsertOutbox(ctx, domain.TopicRecordingUploaded, rec.ID.String(), payload, rec.CorrelationID); err != nil {
		return RecordingDTO{}, err
	}
	return toDTO(rec), nil
}

func extensionFor(contentType, filename string) string {
	filename = strings.TrimSpace(filename)
	if filename != "" {
		ext := path.Ext(filename)
		if ext != "" {
			return strings.ToLower(ext)
		}
	}
	switch {
	case strings.Contains(contentType, "webm"):
		return ".webm"
	case strings.Contains(contentType, "mp4"):
		return ".mp4"
	case strings.Contains(contentType, "quicktime"):
		return ".mov"
	default:
		return ".bin"
	}
}
