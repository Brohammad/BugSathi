package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Brohammad/BugSathi/internal/ai/adapter/memory"
	aimock "github.com/Brohammad/BugSathi/internal/ai/adapter/mock"
	"github.com/Brohammad/BugSathi/internal/ai/domain"
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
	if len(store.Outbox()) != 2 {
		t.Fatalf("outbox=%d", len(store.Outbox()))
	}

	// idempotent
	if err := svc.HandleFramesExtracted(context.Background(), domain.FramesExtractedEvent{
		RecordingID: recID.String(), ProjectID: projID.String(),
		FrameKeys: []string{"a.jpg"}, CorrelationID: "corr",
	}); err != nil {
		t.Fatal(err)
	}
	if len(store.Outbox()) < 4 {
		t.Fatalf("expected more outbox events on replay, got %d", len(store.Outbox()))
	}
}
