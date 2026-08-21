package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/ai/adapter/memory"
	aimock "github.com/Brohammad/BugSathi/internal/ai/adapter/mock"
	"github.com/Brohammad/BugSathi/internal/ai/domain"
	"github.com/Brohammad/BugSathi/internal/ai/port"
	"github.com/Brohammad/BugSathi/internal/ai/service"
	"github.com/google/uuid"
)

func newSvc(store *memory.Store, analyzer port.Analyzer) *service.Service {
	return service.New(store, analyzer, 5, 2*time.Minute, 30*time.Second)
}

func TestHandleFramesExtractedMock(t *testing.T) {
	store := memory.NewStore()
	svc := newSvc(store, aimock.New())

	recID := uuid.New()
	projID := uuid.New()
	store.SeedMeta(recID, projID, json.RawMessage(`{"browser":"chrome"}`))

	err := svc.HandleFramesExtracted(context.Background(), domain.FramesExtractedEvent{
		RecordingID: recID.String(), ProjectID: projID.String(),
		FrameKeys: []string{"a.jpg", "b.jpg", "c.jpg"}, CorrelationID: "corr",
	})
	if err != nil {
		t.Fatal(err)
	}
	rep, ok := store.Report(recID)
	if !ok || rep.Status != domain.ReportReady || rep.Title == "" {
		t.Fatalf("report=%+v ok=%v", rep, ok)
	}
	firstReportID := rep.ID
	if len(store.Outbox()) != 2 {
		t.Fatalf("outbox=%d", len(store.Outbox()))
	}

	// idempotent replay must reuse the same report_id in outbox payloads
	if err := svc.HandleFramesExtracted(context.Background(), domain.FramesExtractedEvent{
		RecordingID: recID.String(), ProjectID: projID.String(),
		FrameKeys: []string{"a.jpg"}, CorrelationID: "corr",
	}); err != nil {
		t.Fatal(err)
	}
	repAfter, ok := store.Report(recID)
	if !ok || repAfter.ID != firstReportID {
		t.Fatalf("report id changed: before=%s after=%+v", firstReportID, repAfter)
	}
	if len(store.Outbox()) < 4 {
		t.Fatalf("expected more outbox events on replay, got %d", len(store.Outbox()))
	}
	for i, ev := range store.Outbox()[2:] {
		var payload struct {
			ReportID string `json:"report_id"`
		}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.ReportID != firstReportID.String() {
			t.Fatalf("outbox[%d] report_id=%q want %q", i+2, payload.ReportID, firstReportID)
		}
	}
}

type badAnalyzer struct{}

func (badAnalyzer) Analyze(context.Context, domain.AnalysisInput) (domain.AnalysisResult, error) {
	return domain.AnalysisResult{Title: " ", Summary: "x", Steps: []string{"  "}}, nil
}

func TestHandleFramesExtractedRejectsInvalidResult(t *testing.T) {
	store := memory.NewStore()
	svc := newSvc(store, badAnalyzer{})
	recID := uuid.New()
	projID := uuid.New()
	store.SeedMeta(recID, projID, json.RawMessage(`{}`))

	err := svc.HandleFramesExtracted(context.Background(), domain.FramesExtractedEvent{
		RecordingID: recID.String(), ProjectID: projID.String(),
		FrameKeys: []string{"a.jpg"}, CorrelationID: "corr",
	})
	if err != domain.ErrInvalidAnalysisResult {
		t.Fatalf("got %v", err)
	}
	if rep, ok := store.Report(recID); ok && rep.Status == domain.ReportReady {
		t.Fatalf("expected no ready report, got %+v", rep)
	}
}

type countingAnalyzer struct {
	n atomic.Int32
}

func (c *countingAnalyzer) Analyze(context.Context, domain.AnalysisInput) (domain.AnalysisResult, error) {
	c.n.Add(1)
	time.Sleep(50 * time.Millisecond)
	return domain.AnalysisResult{
		Title: "Bug", Summary: "Something broke", Steps: []string{"1. Open app"},
		Provider: "count", Model: "test",
	}, nil
}

func TestHandleFramesExtractedSkipsInFlight(t *testing.T) {
	store := memory.NewStore()
	analyzer := &countingAnalyzer{}
	svc := newSvc(store, analyzer)
	recID := uuid.New()
	projID := uuid.New()
	store.SeedMeta(recID, projID, json.RawMessage(`{}`))
	store.SeedRunning(recID, projID, domain.PromptVersion, time.Now())

	err := svc.HandleFramesExtracted(context.Background(), domain.FramesExtractedEvent{
		RecordingID: recID.String(), ProjectID: projID.String(),
		FrameKeys: []string{"a.jpg"}, CorrelationID: "corr",
	})
	if !errors.Is(err, domain.ErrAnalysisInFlight) {
		t.Fatalf("got %v", err)
	}
	if analyzer.n.Load() != 0 {
		t.Fatalf("analyzer called %d times", analyzer.n.Load())
	}
}

func TestHandleFramesExtractedReclaimsStaleLease(t *testing.T) {
	store := memory.NewStore()
	svc := service.New(store, aimock.New(), 5, 50*time.Millisecond, 10*time.Millisecond)
	recID := uuid.New()
	projID := uuid.New()
	store.SeedMeta(recID, projID, json.RawMessage(`{}`))
	store.SeedRunning(recID, projID, domain.PromptVersion, time.Now().Add(-time.Minute))

	if err := svc.HandleFramesExtracted(context.Background(), domain.FramesExtractedEvent{
		RecordingID: recID.String(), ProjectID: projID.String(),
		FrameKeys: []string{"a.jpg"}, CorrelationID: "corr",
	}); err != nil {
		t.Fatal(err)
	}
	rep, ok := store.Report(recID)
	if !ok || rep.Status != domain.ReportReady {
		t.Fatalf("report=%+v ok=%v", rep, ok)
	}
}

type invalidateCache struct {
	ids []uuid.UUID
}

func (c *invalidateCache) Invalidate(id uuid.UUID) {
	c.ids = append(c.ids, id)
}

func TestHandleFramesExtractedInvalidatesCache(t *testing.T) {
	store := memory.NewStore()
	cache := &invalidateCache{}
	svc := newSvc(store, aimock.New()).WithCacheInvalidator(cache)
	recID := uuid.New()
	projID := uuid.New()
	store.SeedMeta(recID, projID, json.RawMessage(`{}`))

	if err := svc.HandleFramesExtracted(context.Background(), domain.FramesExtractedEvent{
		RecordingID: recID.String(), ProjectID: projID.String(),
		FrameKeys: []string{"a.jpg"}, CorrelationID: "corr",
	}); err != nil {
		t.Fatal(err)
	}
	rep, _ := store.Report(recID)
	if len(cache.ids) != 1 || cache.ids[0] != rep.ID {
		t.Fatalf("invalidate=%v want %s", cache.ids, rep.ID)
	}
}

var _ port.Analyzer = badAnalyzer{}
var _ port.Analyzer = (*countingAnalyzer)(nil)
