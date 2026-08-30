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
	store      port.Store
	analyzer   port.Analyzer
	frames     port.FrameReader
	maxFrames  int
	frameBytes int64
	claimLease time.Duration
	claimRenew time.Duration
	cache      port.ReportCacheInvalidator
	now        func() time.Time
}

const defaultFrameMaxBytes int64 = 5 << 20

func New(store port.Store, analyzer port.Analyzer, maxFrames int, claimLease, claimRenew time.Duration) *Service {
	if maxFrames <= 0 {
		maxFrames = 5
	}
	if claimLease <= 0 {
		claimLease = 2 * time.Minute
	}
	if claimRenew <= 0 || claimRenew >= claimLease {
		claimRenew = claimLease / 4
		if claimRenew < time.Second {
			claimRenew = time.Second
		}
	}
	return &Service{
		store: store, analyzer: analyzer, maxFrames: maxFrames,
		frameBytes: defaultFrameMaxBytes,
		claimLease: claimLease, claimRenew: claimRenew, now: time.Now,
	}
}

// WithFrameReader enables visual analysis. The reader remains optional so the
// deterministic mock provider can run without object-storage reads.
func (s *Service) WithFrameReader(reader port.FrameReader, maxBytes int64) *Service {
	s.frames = reader
	if maxBytes > 0 {
		s.frameBytes = maxBytes
	}
	return s
}

// WithCacheInvalidator clears report detail caches after successful/failed writes.
func (s *Service) WithCacheInvalidator(cache port.ReportCacheInvalidator) *Service {
	s.cache = cache
	return s
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

	now := s.now()
	leaseCutoff := now.Add(-s.claimLease)
	if _, err := s.store.TryClaimRunning(ctx, recordingID, projectID, domain.PromptVersion, now, leaseCutoff); err != nil {
		if errors.Is(err, domain.ErrAnalysisInFlight) {
			return err
		}
		// Completed raced in between Get and claim.
		if errors.Is(err, domain.ErrNotFound) {
			existing, gerr := s.store.GetAnalysis(ctx, recordingID, domain.PromptVersion)
			if gerr == nil && existing.Status == domain.AnalysisCompleted {
				return s.emitCompletedEvents(ctx, existing, evt.CorrelationID)
			}
			if gerr == nil && existing.Status == domain.AnalysisRunning {
				return domain.ErrAnalysisInFlight
			}
		}
		return err
	}

	if _, err := s.store.MarkGenerating(ctx, recordingID, projectID, evt.CorrelationID, now); err != nil {
		return err
	}

	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go s.renewLease(jobCtx, recordingID, domain.PromptVersion)

	frameKeys := keyframes.Select(evt.FrameKeys, s.maxFrames)
	frames, err := s.loadFrames(jobCtx, frameKeys)
	if err != nil {
		_ = s.store.FailAnalysis(ctx, recordingID, domain.PromptVersion, err.Error(), s.now())
		s.invalidateReport(ctx, recordingID)
		return err
	}

	result, err := s.analyzer.Analyze(jobCtx, domain.AnalysisInput{
		RecordingID:   evt.RecordingID,
		ProjectID:     evt.ProjectID,
		FrameKeys:     frameKeys,
		Frames:        frames,
		MetadataJSON:  meta,
		PromptVersion: domain.PromptVersion,
	})
	if err != nil {
		_ = s.store.FailAnalysis(ctx, recordingID, domain.PromptVersion, err.Error(), s.now())
		s.invalidateReport(ctx, recordingID)
		return err
	}
	result = domain.NormalizeAnalysisResult(result)
	if err := domain.ValidateAnalysisResult(result); err != nil {
		_ = s.store.FailAnalysis(ctx, recordingID, domain.PromptVersion, err.Error(), s.now())
		s.invalidateReport(ctx, recordingID)
		return err
	}

	stepsJSON, err := json.Marshal(result.Steps)
	if err != nil {
		return err
	}
	now = s.now()
	reportID := uuid.New()
	if existing, err := s.store.GetReportByRecording(ctx, recordingID); err == nil {
		reportID = existing.ID
	}
	analysis := domain.Analysis{
		ID: uuid.New(), RecordingID: recordingID, ProjectID: projectID,
		PromptVersion: domain.PromptVersion, Status: domain.AnalysisCompleted,
		Title: result.Title, Summary: result.Summary, Steps: stepsJSON,
		Provider: result.Provider, Model: result.Model,
		CreatedAt: now, UpdatedAt: now,
	}
	report := domain.Report{
		ID: reportID, RecordingID: recordingID, ProjectID: projectID,
		Status: domain.ReportReady, Title: result.Title, Summary: result.Summary,
		Steps: stepsJSON, AIStatus: string(domain.AnalysisCompleted),
		PromptVersion: domain.PromptVersion, CreatedAt: now, UpdatedAt: now,
	}

	if err := s.store.CompleteAnalysis(ctx, analysis, report, s.buildOutboxEvents(report, evt.RecordingID, evt.ProjectID, evt.CorrelationID, now), now); err != nil {
		return err
	}
	if rep, rerr := s.store.GetReportByRecording(ctx, recordingID); rerr == nil {
		s.invalidateID(rep.ID)
	}
	return nil
}

func (s *Service) loadFrames(ctx context.Context, keys []string) ([]domain.FrameInput, error) {
	if s.frames == nil {
		return nil, nil
	}
	frames := make([]domain.FrameInput, 0, len(keys))
	for _, key := range keys {
		frame, err := s.frames.ReadFrame(ctx, key, s.frameBytes)
		if err != nil {
			return nil, fmt.Errorf("load frame %q: %w", key, err)
		}
		frames = append(frames, frame)
	}
	return frames, nil
}

func (s *Service) renewLease(ctx context.Context, recordingID uuid.UUID, promptVersion string) {
	ticker := time.NewTicker(s.claimRenew)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.store.TouchRunning(context.Background(), recordingID, promptVersion, s.now())
		}
	}
}

func (s *Service) invalidateReport(ctx context.Context, recordingID uuid.UUID) {
	if s.cache == nil {
		return
	}
	if rep, err := s.store.GetReportByRecording(ctx, recordingID); err == nil {
		s.invalidateID(rep.ID)
	}
}

func (s *Service) invalidateID(reportID uuid.UUID) {
	if s.cache != nil {
		s.cache.Invalidate(reportID)
	}
}

func (s *Service) emitCompletedEvents(ctx context.Context, existing domain.Analysis, corr string) error {
	report, err := s.store.GetReportByRecording(ctx, existing.RecordingID)
	if err != nil {
		return err
	}
	now := s.now()
	return s.store.CompleteAnalysis(ctx, existing, report, s.buildOutboxEvents(report, existing.RecordingID.String(), existing.ProjectID.String(), corr, now), now)
}

func (s *Service) buildOutboxEvents(report domain.Report, recordingID, projectID, corr string, at time.Time) []port.OutboxEvent {
	completedPayload, _ := json.Marshal(domain.AnalysisCompletedEvent{
		SchemaVersion: 1, RecordingID: recordingID, ReportID: report.ID.String(),
		PromptVersion: domain.PromptVersion, CorrelationID: corr, OccurredAt: at.UTC(),
	})
	reportPayload, _ := json.Marshal(domain.ReportGeneratedEvent{
		SchemaVersion: 1, ReportID: report.ID.String(), RecordingID: recordingID,
		ProjectID: projectID, CorrelationID: corr, OccurredAt: at.UTC(),
	})
	return []port.OutboxEvent{
		{Topic: domain.TopicAnalysisCompleted, PartitionKey: recordingID, Payload: completedPayload, CorrelationID: corr},
		{Topic: domain.TopicReportGenerated, PartitionKey: recordingID, Payload: reportPayload, CorrelationID: corr},
	}
}
