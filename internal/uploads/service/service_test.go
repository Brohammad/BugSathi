package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/uploads/adapter/memory"
	"github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/Brohammad/BugSathi/internal/uploads/service"
	"github.com/google/uuid"
)

func TestCreateCompleteIdempotent(t *testing.T) {
	outbox := memory.NewOutboxRepo()
	recs := memory.NewRecordingRepo(outbox)
	store := memory.NewStorage()
	svc := service.New(recs, store, memory.AccessOK{}, time.Minute)

	ctx := context.Background()
	user := uuid.New()
	project := uuid.New()

	created, err := svc.Create(ctx, service.CreateInput{
		ProjectID: project, UserID: user,
		ContentType: "video/webm", Filename: "bug.webm",
		Metadata: json.RawMessage(`{"browser":"chrome"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Recording.Status != domain.StatusUploading {
		t.Fatalf("status=%s", created.Recording.Status)
	}
	if created.UploadURL == "" {
		t.Fatal("expected upload url")
	}

	// complete without object → missing
	if _, err := svc.Complete(ctx, user, project, created.Recording.ID); err != domain.ErrObjectMissing {
		t.Fatalf("expected object missing, got %v", err)
	}

	store.Put(created.Recording.StorageKey, "video/webm", []byte("fake-video"))
	dto, err := svc.Complete(ctx, user, project, created.Recording.ID)
	if err != nil {
		t.Fatal(err)
	}
	if dto.Status != domain.StatusUploaded {
		t.Fatalf("status=%s", dto.Status)
	}
	if dto.ByteSize == nil || *dto.ByteSize != 10 {
		t.Fatalf("size=%v", dto.ByteSize)
	}
	if len(outbox.Messages()) != 1 {
		t.Fatalf("outbox=%d", len(outbox.Messages()))
	}

	// idempotent complete
	dto2, err := svc.Complete(ctx, user, project, created.Recording.ID)
	if err != nil {
		t.Fatal(err)
	}
	if dto2.Status != domain.StatusUploaded {
		t.Fatalf("status=%s", dto2.Status)
	}
	if len(outbox.Messages()) != 1 {
		t.Fatalf("outbox should not duplicate, got %d", len(outbox.Messages()))
	}
}

func TestCreateForbidden(t *testing.T) {
	outbox := memory.NewOutboxRepo()
	svc := service.New(memory.NewRecordingRepo(outbox), memory.NewStorage(), memory.AccessDeny{}, time.Minute)
	_, err := svc.Create(context.Background(), service.CreateInput{
		ProjectID: uuid.New(), UserID: uuid.New(), ContentType: "video/webm",
	})
	if err != domain.ErrForbidden {
		t.Fatalf("got %v", err)
	}
}
