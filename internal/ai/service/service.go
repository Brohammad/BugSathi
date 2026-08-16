package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Brohammad/BugSathi/internal/ai/domain"
	"github.com/Brohammad/BugSathi/internal/ai/keyframes"
	"github.com/Brohammad/BugSathi/internal/ai/port"
	"github.com/google/uuid"
)

type Service struct {
	store     port.Store
	analyzer  port.Analyzer
	maxFrames int
	now       func() time.Time
}

func New(store port.Store, analyzer port.Analyzer, maxFrames int) *Service {
	if maxFrames <= 0 {
		maxFrames = 5
	}
	return &Service{store: store, analyzer: analyzer, maxFrames: maxFrames, now: time.Now}
}

func (s *Service) HandleFramesExtracted(ctx context.Context, evt domain.FramesExtractedEvent) error {
	recordingID, err := uuid.Parse(evt.RecordingID)
	if err != nil {
		return fmt.Errorf("recording_id: %w", err)
	}
	projectID, err := uuid.Parse(evt.ProjectID)
	if err != nil {
		return fmt.Errorf("project_id: %w", err)
	}

	existing, err := s.store.GetAnalysis(ctx, recordingID, domain.PromptVersion)
	if err == nil && existing.Status == domain.AnalysisCompleted {
		return s.emitCompletedEvents(ctx, existing, evt.CorrelationID)
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	_, meta, err := s.store.GetRecordingMeta(ctx, recordingID)
	if err != nil {
		return err
	}

	if _, err := s.store.UpsertRunning(ctx, recordingID, projectID, domain.PromptVersion, s.now()); err != nil {
		return err
	}

	frameKeys := keyframes.Select(evt.FrameKeys, s.maxFrames)

	result, err := s.analyzer.Analyze(ctx, domain.AnalysisInput{
		RecordingID:   evt.RecordingID,
		ProjectID:     evt.ProjectID,
		FrameKeys:     frameKeys,
		MetadataJSON:  meta,
		PromptVersion: domain.PromptVersion,
	})
	if err != nil {
		_ = s.store.FailAnalysis(ctx, recordingID, domain.PromptVersion, err.Error(), s.now())
		return err
	}

	stepsJSON, err := json.Marshal(result.Steps)
	if err != nil {
		return err
	}
	now := s.now()
	analysis := domain.Analysis{
		ID: uuid.New(), RecordingID: recordingID, ProjectID: projectID,
		PromptVersion: domain.PromptVersion, Status: domain.AnalysisCompleted,
		Title: result.Title, Summary: result.Summary, Steps: stepsJSON,
		Provider: result.Provider, Model: result.Model,
		CreatedAt: now, UpdatedAt: now,
	}
	report := domain.Report{
		ID: uuid.New(), RecordingID: recordingID, ProjectID: projectID,
		Status: domain.ReportReady, Title: result.Title, Summary: result.Summary,
		Steps: stepsJSON, AIStatus: string(domain.AnalysisCompleted),
		PromptVersion: domain.PromptVersion, CreatedAt: now, UpdatedAt: now,
	}

	corr := evt.CorrelationID
	completedPayload, _ := json.Marshal(domain.AnalysisCompletedEvent{
		SchemaVersion: 1, RecordingID: evt.RecordingID, ReportID: report.ID.String(),
		PromptVersion: domain.PromptVersion, CorrelationID: corr, OccurredAt: now.UTC(),
	})
	reportPayload, _ := json.Marshal(domain.ReportGeneratedEvent{
		SchemaVersion: 1, ReportID: report.ID.String(), RecordingID: evt.RecordingID,
		ProjectID: evt.ProjectID, CorrelationID: corr, OccurredAt: now.UTC(),
	})

	return s.store.CompleteAnalysis(ctx, analysis, report, []port.OutboxEvent{
		{Topic: domain.TopicAnalysisCompleted, PartitionKey: evt.RecordingID, Payload: completedPayload, CorrelationID: corr},
		{Topic: domain.TopicReportGenerated, PartitionKey: evt.RecordingID, Payload: reportPayload, CorrelationID: corr},
	}, now)
}

func (s *Service) emitCompletedEvents(ctx context.Context, existing domain.Analysis, corr string) error {
	// Ensure downstream events exist again (at-least-once friendly).
	now := s.now()
	report := domain.Report{
		ID: uuid.New(), RecordingID: existing.RecordingID, ProjectID: existing.ProjectID,
		Status: domain.ReportReady, Title: existing.Title, Summary: existing.Summary,
		Steps: existing.Steps, AIStatus: string(domain.AnalysisCompleted),
		PromptVersion: existing.PromptVersion, CreatedAt: now, UpdatedAt: now,
	}
	completedPayload, _ := json.Marshal(domain.AnalysisCompletedEvent{
		SchemaVersion: 1, RecordingID: existing.RecordingID.String(), ReportID: report.ID.String(),
		PromptVersion: existing.PromptVersion, CorrelationID: corr, OccurredAt: now.UTC(),
	})
	reportPayload, _ := json.Marshal(domain.ReportGeneratedEvent{
		SchemaVersion: 1, ReportID: report.ID.String(), RecordingID: existing.RecordingID.String(),
		ProjectID: existing.ProjectID.String(), CorrelationID: corr, OccurredAt: now.UTC(),
	})
	return s.store.CompleteAnalysis(ctx, existing, report, []port.OutboxEvent{
		{Topic: domain.TopicAnalysisCompleted, PartitionKey: existing.RecordingID.String(), Payload: completedPayload, CorrelationID: corr},
		{Topic: domain.TopicReportGenerated, PartitionKey: existing.RecordingID.String(), Payload: reportPayload, CorrelationID: corr},
	}, now)
}
