package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Brohammad/BugSathi/internal/ai/adapter/memory"
	aimock "github.com/Brohammad/BugSathi/internal/ai/adapter/mock"
	"github.com/Brohammad/BugSathi/internal/ai/domain"
	"github.com/Brohammad/BugSathi/internal/ai/port"
	"github.com/Brohammad/BugSathi/internal/ai/service"
	"github.com/google/uuid"
)

func TestHandleFramesExtractedMock(t *testing.T) {
	store := memory.NewStore()
	svc := service.New(store, aimock.New(), 5)

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
	svc := service.New(store, badAnalyzer{}, 5)
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
	if _, ok := store.Report(recID); ok {
		t.Fatal("expected no ready report")
	}
}

var _ port.Analyzer = badAnalyzer{}
