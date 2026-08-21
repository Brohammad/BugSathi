package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/sharing/adapter/memory"
	"github.com/Brohammad/BugSathi/internal/sharing/domain"
	"github.com/Brohammad/BugSathi/internal/sharing/port"
	"github.com/Brohammad/BugSathi/internal/sharing/service"
	"github.com/google/uuid"
)

func TestShareCreatePublicRevoke(t *testing.T) {
	repo := memory.NewRepo()
	projectID := uuid.New()
	reportID := uuid.New()
	userID := uuid.New()
	reports := memory.NewReports(port.PublicReport{
		ReportID: reportID, ProjectID: projectID, Status: "READY",
		Title: "Bug", Summary: "Sum", Steps: json.RawMessage(`["a"]`),
		Frames:   []port.PublicFrame{{Ordinal: 0, StorageKey: "f.jpg", ContentType: "image/jpeg"}},
		ThumbKey: "t.jpg",
	})
	svc := service.New(repo, memory.AccessOK{}, reports, memory.Signer{}, config.SharingConfig{HashTokens: true}, config.ListConfig{})

	exp := 3600 * time.Second
	share, err := svc.Create(context.Background(), userID, projectID, reportID, &exp)
	if err != nil {
		t.Fatal(err)
	}
	if share.Token == "" || share.URLPath != "/s/"+share.Token {
		t.Fatalf("%+v", share)
	}

	view, err := svc.PublicGet(context.Background(), share.Token)
	if err != nil || view.Title != "Bug" || len(view.Frames) != 1 || view.Frames[0].URL == "" {
		t.Fatalf("%+v %v", view, err)
	}

	if err := svc.Revoke(context.Background(), userID, projectID, share.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublicGet(context.Background(), share.Token); err != domain.ErrShareInactive {
		t.Fatalf("got %v", err)
	}
}

func TestShareCreateRevokeRequireOwner(t *testing.T) {
	repo := memory.NewRepo()
	projectID := uuid.New()
	reportID := uuid.New()
	userID := uuid.New()
	reports := memory.NewReports(port.PublicReport{
		ReportID: reportID, ProjectID: projectID, Status: "READY",
		Title: "Bug", Summary: "Sum", Steps: json.RawMessage(`[]`),
	})
	svc := service.New(repo, memory.AccessMemberOnly{}, reports, memory.Signer{}, config.SharingConfig{HashTokens: true}, config.ListConfig{})
	exp := time.Hour
	if _, err := svc.Create(context.Background(), userID, projectID, reportID, &exp); err != domain.ErrForbidden {
		t.Fatalf("create: got %v", err)
	}
	if err := svc.Revoke(context.Background(), userID, projectID, uuid.New()); err != domain.ErrForbidden {
		t.Fatalf("revoke: got %v", err)
	}
}

func TestShareDefaultTTLAndRejectNeverExpire(t *testing.T) {
	repo := memory.NewRepo()
	projectID := uuid.New()
	reportID := uuid.New()
	userID := uuid.New()
	reports := memory.NewReports(port.PublicReport{
		ReportID: reportID, ProjectID: projectID, Status: "READY",
		Title: "Bug", Summary: "Sum", Steps: json.RawMessage(`[]`),
	})
	svc := service.New(repo, memory.AccessOK{}, reports, memory.Signer{}, config.SharingConfig{
		DefaultTTL: 24 * time.Hour, MaxTTL: 48 * time.Hour, HashTokens: true,
	}, config.ListConfig{})

	share, err := svc.Create(context.Background(), userID, projectID, reportID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if share.ExpiresAt == nil {
		t.Fatal("expected default expiry")
	}

	zero := time.Duration(0)
	if _, err := svc.Create(context.Background(), userID, projectID, reportID, &zero); err != domain.ErrInvalidInput {
		t.Fatalf("zero expiry: got %v", err)
	}
	over := 72 * time.Hour
	if _, err := svc.Create(context.Background(), userID, projectID, reportID, &over); err != domain.ErrInvalidInput {
		t.Fatalf("over max ttl: got %v", err)
	}
}
