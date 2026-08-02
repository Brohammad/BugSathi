package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/media/adapter/memory"
	"github.com/Brohammad/BugSathi/internal/media/domain"
	"github.com/Brohammad/BugSathi/internal/media/service"
	uploaddomain "github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/google/uuid"
)

func TestHandleUploadedExtractsFrames(t *testing.T) {
	store := memory.NewStore()
	objs := memory.NewObjects()
	svc := service.New(store, objs, memory.FakeExtractor{})

	id := uuid.New()
	project := uuid.New()
	key := "projects/" + project.String() + "/recordings/" + id.String() + "/source.webm"
	objs.Put(key, []byte("not-a-real-video-but-fake-extractor-ignores-it"))
	store.Seed(uploaddomain.Recording{
		ID: id, ProjectID: project, Status: uploaddomain.StatusUploaded,
		StorageKey: key, ContentType: "video/webm", CorrelationID: "c1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	err := svc.HandleUploaded(context.Background(), domain.RecordingUploadedEvent{
		RecordingID: id.String(), ProjectID: project.String(),
		ObjectKey: key, CorrelationID: "c1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.Status(id) != uploaddomain.StatusReady {
		t.Fatalf("status=%s", store.Status(id))
	}
	arts, _ := store.ListArtifacts(context.Background(), id)
	if len(arts) < 2 {
		t.Fatalf("artifacts=%d", len(arts))
	}
	if len(store.Outbox()) != 1 {
		t.Fatalf("outbox=%d", len(store.Outbox()))
	}

	// idempotent second pass
	if err := svc.HandleUploaded(context.Background(), domain.RecordingUploadedEvent{
		RecordingID: id.String(), ProjectID: project.String(),
		ObjectKey: key, CorrelationID: "c1",
	}); err != nil {
		t.Fatal(err)
	}
	if len(store.Outbox()) < 2 {
		t.Fatalf("expected republish outbox, got %d", len(store.Outbox()))
	}
}
