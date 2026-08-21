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
	if dto.Checksum == "" {
		t.Fatal("expected checksum from object etag")
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

func TestCompleteRejectsTooLarge(t *testing.T) {
	outbox := memory.NewOutboxRepo()
	recs := memory.NewRecordingRepo(outbox)
	store := memory.NewStorage()
	svc := service.New(recs, store, memory.AccessOK{}, time.Minute).WithUploadMaxBytes(5)

	ctx := context.Background()
	user := uuid.New()
	project := uuid.New()
	created, err := svc.Create(ctx, service.CreateInput{
		ProjectID: project, UserID: user, ContentType: "video/webm", Filename: "bug.webm",
	})
	if err != nil {
		t.Fatal(err)
	}
	store.Put(created.Recording.StorageKey, "video/webm", []byte("123456"))
	if _, err := svc.Complete(ctx, user, project, created.Recording.ID); err != domain.ErrObjectTooLarge {
		t.Fatalf("got %v", err)
	}
}

func TestCompleteRejectsContentTypeMismatch(t *testing.T) {
	outbox := memory.NewOutboxRepo()
	recs := memory.NewRecordingRepo(outbox)
	store := memory.NewStorage()
	svc := service.New(recs, store, memory.AccessOK{}, time.Minute)

	ctx := context.Background()
	user := uuid.New()
	project := uuid.New()
	created, err := svc.Create(ctx, service.CreateInput{
		ProjectID: project, UserID: user, ContentType: "video/webm", Filename: "bug.webm",
	})
	if err != nil {
		t.Fatal(err)
	}
	store.Put(created.Recording.StorageKey, "video/mp4", []byte("fake-video"))
	if _, err := svc.Complete(ctx, user, project, created.Recording.ID); err != domain.ErrContentTypeMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestCreateRejectsDisallowedContentType(t *testing.T) {
	outbox := memory.NewOutboxRepo()
	svc := service.New(memory.NewRecordingRepo(outbox), memory.NewStorage(), memory.AccessOK{}, time.Minute)
	_, err := svc.Create(context.Background(), service.CreateInput{
		ProjectID: uuid.New(), UserID: uuid.New(), ContentType: "application/pdf",
	})
	if err != domain.ErrInvalidInput {
		t.Fatalf("got %v", err)
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

func TestReprocessFailedRecording(t *testing.T) {
	outbox := memory.NewOutboxRepo()
	recs := memory.NewRecordingRepo(outbox)
	store := memory.NewStorage()
	svc := service.New(recs, store, memory.AccessOK{}, time.Minute)

	ctx := context.Background()
	user := uuid.New()
	project := uuid.New()

	created, err := svc.Create(ctx, service.CreateInput{
		ProjectID: project, UserID: user, ContentType: "video/webm", Filename: "bug.webm",
	})
	if err != nil {
		t.Fatal(err)
	}
	store.Put(created.Recording.StorageKey, "video/webm", []byte("fake-video"))
	if _, err := svc.Complete(ctx, user, project, created.Recording.ID); err != nil {
		t.Fatal(err)
	}

	rec, err := recs.Get(ctx, created.Recording.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec.Status = domain.StatusFailed
	if _, err := recs.Update(ctx, rec); err != nil {
		t.Fatal(err)
	}

	before := len(outbox.Messages())
	dto, err := svc.Reprocess(ctx, user, project, created.Recording.ID)
	if err != nil {
		t.Fatal(err)
	}
	if dto.Status != domain.StatusFailed {
		t.Fatalf("status=%s", dto.Status)
	}
	if len(outbox.Messages()) != before+1 {
		t.Fatalf("outbox=%d want %d", len(outbox.Messages()), before+1)
	}
	last := outbox.Messages()[len(outbox.Messages())-1]
	if last.Topic != domain.TopicRecordingUploaded {
		t.Fatalf("topic=%s", last.Topic)
	}
}

func TestReprocessRequiresOwner(t *testing.T) {
	outbox := memory.NewOutboxRepo()
	recs := memory.NewRecordingRepo(outbox)
	store := memory.NewStorage()
	svc := service.New(recs, store, memory.AccessMemberOnly{}, time.Minute)

	ctx := context.Background()
	user := uuid.New()
	project := uuid.New()
	created, err := svc.Create(ctx, service.CreateInput{
		ProjectID: project, UserID: user, ContentType: "video/webm",
	})
	if err != nil {
		t.Fatal(err)
	}
	store.Put(created.Recording.StorageKey, "video/webm", []byte("x"))
	if _, err := svc.Complete(ctx, user, project, created.Recording.ID); err != nil {
		t.Fatal(err)
	}
	rec, _ := recs.Get(ctx, created.Recording.ID)
	rec.Status = domain.StatusFailed
	_, _ = recs.Update(ctx, rec)

	if _, err := svc.Reprocess(ctx, user, project, created.Recording.ID); err != domain.ErrForbidden {
		t.Fatalf("got %v", err)
	}
}

func TestReprocessUploadingIllegal(t *testing.T) {
	outbox := memory.NewOutboxRepo()
	recs := memory.NewRecordingRepo(outbox)
	svc := service.New(recs, memory.NewStorage(), memory.AccessOK{}, time.Minute)
	ctx := context.Background()
	user := uuid.New()
	project := uuid.New()
	created, err := svc.Create(ctx, service.CreateInput{
		ProjectID: project, UserID: user, ContentType: "video/webm",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reprocess(ctx, user, project, created.Recording.ID); err != domain.ErrIllegalTransition {
		t.Fatalf("got %v", err)
	}
}

func TestSweepAbandonedUploads(t *testing.T) {
	outbox := memory.NewOutboxRepo()
	recs := memory.NewRecordingRepo(outbox)
	store := memory.NewStorage()
	svc := service.New(recs, store, memory.AccessOK{}, time.Minute)
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	svc.SetNow(now)

	ctx := context.Background()
	user := uuid.New()
	project := uuid.New()
	created, err := svc.Create(ctx, service.CreateInput{
		ProjectID: project, UserID: user, ContentType: "video/webm",
	})
	if err != nil {
		t.Fatal(err)
	}
	store.Put(created.Recording.StorageKey, "video/webm", []byte("partial"))

	// Still fresh — should not sweep.
	n, err := svc.SweepAbandonedUploads(ctx, time.Hour, 10)
	if err != nil || n != 0 {
		t.Fatalf("fresh sweep n=%d err=%v", n, err)
	}

	rec, _ := recs.Get(ctx, created.Recording.ID)
	rec.UpdatedAt = now.Add(-2 * time.Hour)
	_, _ = recs.Update(ctx, rec)

	n, err = svc.SweepAbandonedUploads(ctx, time.Hour, 10)
	if err != nil || n != 1 {
		t.Fatalf("stale sweep n=%d err=%v", n, err)
	}
	if _, err := recs.Get(ctx, created.Recording.ID); err != domain.ErrNotFound {
		t.Fatalf("recording still present: %v", err)
	}
	if store.Has(created.Recording.StorageKey) {
		t.Fatal("object should be deleted")
	}
}
