package memory

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Brohammad/BugSathi/internal/ai/domain"
	"github.com/Brohammad/BugSathi/internal/ai/port"
	"github.com/google/uuid"
)

type Store struct {
	mu       sync.Mutex
	analyses map[string]domain.Analysis
	reports  map[uuid.UUID]domain.Report
	meta     map[uuid.UUID]meta
	outbox   []port.OutboxEvent
}

type meta struct {
	ProjectID uuid.UUID
	Metadata  json.RawMessage
}

func NewStore() *Store {
	return &Store{
		analyses: map[string]domain.Analysis{},
		reports:  map[uuid.UUID]domain.Report{},
		meta:     map[uuid.UUID]meta{},
	}
}

func key(recordingID uuid.UUID, prompt string) string {
	return recordingID.String() + "|" + prompt
}

func (s *Store) SeedMeta(recordingID, projectID uuid.UUID, metadata json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meta[recordingID] = meta{ProjectID: projectID, Metadata: metadata}
}

func (s *Store) Outbox() []port.OutboxEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]port.OutboxEvent(nil), s.outbox...)
}

func (s *Store) Report(recordingID uuid.UUID) (domain.Report, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.reports[recordingID]
	return r, ok
}

func (s *Store) GetAnalysis(_ context.Context, recordingID uuid.UUID, promptVersion string) (domain.Analysis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.analyses[key(recordingID, promptVersion)]
	if !ok {
		return domain.Analysis{}, domain.ErrNotFound
	}
	return a, nil
}

func (s *Store) UpsertRunning(_ context.Context, recordingID, projectID uuid.UUID, promptVersion string, at time.Time) (domain.Analysis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := domain.Analysis{
		ID: uuid.New(), RecordingID: recordingID, ProjectID: projectID,
		PromptVersion: promptVersion, Status: domain.AnalysisRunning,
		CreatedAt: at, UpdatedAt: at,
	}
	s.analyses[key(recordingID, promptVersion)] = a
	return a, nil
}

func (s *Store) CompleteAnalysis(_ context.Context, analysis domain.Analysis, report domain.Report, events []port.OutboxEvent, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	analysis.Status = domain.AnalysisCompleted
	analysis.UpdatedAt = at
	s.analyses[key(analysis.RecordingID, analysis.PromptVersion)] = analysis
	report.UpdatedAt = at
	s.reports[report.RecordingID] = report
	s.outbox = append(s.outbox, events...)
	return nil
}

func (s *Store) FailAnalysis(_ context.Context, recordingID uuid.UUID, promptVersion, msg string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.analyses[key(recordingID, promptVersion)]
	a.Status = domain.AnalysisFailed
	a.ErrorMessage = msg
	a.UpdatedAt = at
	s.analyses[key(recordingID, promptVersion)] = a
	return nil
}

func (s *Store) GetRecordingMeta(_ context.Context, recordingID uuid.UUID) (uuid.UUID, json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.meta[recordingID]
	if !ok {
		return uuid.Nil, nil, domain.ErrNotFound
	}
	return m.ProjectID, m.Metadata, nil
}
