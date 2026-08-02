package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/reports/adapter/memory"
	"github.com/Brohammad/BugSathi/internal/reports/domain"
	"github.com/Brohammad/BugSathi/internal/reports/service"
	"github.com/google/uuid"
)

func TestListAndGetReport(t *testing.T) {
	repo := memory.NewRepo()
	svc := service.New(repo, memory.AccessOK{}, memory.Signer{})

	projectID := uuid.New()
	userID := uuid.New()
	reportID := uuid.New()
	recordingID := uuid.New()
	repo.Seed(domain.Detail{
		Report: domain.Report{
			ID: reportID, ProjectID: projectID, RecordingID: recordingID,
			Status: domain.StatusReady, Title: "Bug", Summary: "Summary",
			Steps: json.RawMessage(`["one"]`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		RecordingStatus: "READY",
		Metadata:        json.RawMessage(`{"browser":"chrome"}`),
		Frames: []domain.Frame{{
			Ordinal: 0, StorageKey: "projects/p/recordings/r/frames/00000.jpg", ContentType: "image/jpeg",
		}},
		ThumbURL: "projects/p/recordings/r/thumb.jpg",
	})

	list, err := svc.List(context.Background(), userID, projectID)
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}

	detail, err := svc.Get(context.Background(), userID, projectID, reportID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Report.Title != "Bug" || len(detail.Frames) != 1 {
		t.Fatalf("%+v", detail)
	}
	if detail.Frames[0].URL == "" || detail.ThumbURL == "" {
		t.Fatalf("expected presigned urls: %+v", detail)
	}

	byRec, err := svc.GetByRecording(context.Background(), userID, projectID, recordingID)
	if err != nil || byRec.Report.ID != reportID {
		t.Fatalf("%+v %v", byRec, err)
	}

	deny := service.New(repo, memory.AccessDeny{}, memory.Signer{})
	if _, err := deny.List(context.Background(), userID, projectID); err != domain.ErrForbidden {
		t.Fatalf("got %v", err)
	}
}
