package service_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/media/adapter/memory"
	"github.com/Brohammad/BugSathi/internal/media/domain"
	"github.com/Brohammad/BugSathi/internal/media/port"
	"github.com/Brohammad/BugSathi/internal/media/service"
	uploaddomain "github.com/Brohammad/BugSathi/internal/uploads/domain"
	"github.com/google/uuid"
)

// blockingExtractor counts ffmpeg runs and can hold a job open so a second
// delivery overlaps with the first.
type blockingExtractor struct {
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
	err     error
}

func (e *blockingExtractor) Extract(ctx context.Context, inputPath, outputDir string) (port.Result, error) {
	e.calls.Add(1)
	if e.entered != nil {
		e.entered <- struct{}{}
	}
	if e.release != nil {
		select {
		case <-e.release:
		case <-ctx.Done():
			return port.Result{}, ctx.Err()
		}
	}
	if e.err != nil {
		return port.Result{}, e.err
	}
	return memory.FakeExtractor{}.Extract(ctx, inputPath, outputDir)
}

type fixture struct {
	store   *memory.Store
	objects *memory.Objects
	id      uuid.UUID
	project uuid.UUID
	key     string
}

func newFixture(t *testing.T, status uploaddomain.Status) fixture {
	t.Helper()
	f := fixture{
		store:   memory.NewStore(),
		objects: memory.NewObjects(),
		id:      uuid.New(),
		project: uuid.New(),
	}
	f.key = "projects/" + f.project.String() + "/recordings/" + f.id.String() + "/source.webm"
	f.objects.Put(f.key, []byte("not-a-real-video-but-fake-extractor-ignores-it"))
	f.store.Seed(uploaddomain.Recording{
		ID: f.id, ProjectID: f.project, Status: status,
		StorageKey: f.key, ContentType: "video/webm", CorrelationID: "c1",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	return f
}

func (f fixture) event() domain.RecordingUploadedEvent {
	return domain.RecordingUploadedEvent{
		RecordingID: f.id.String(), ProjectID: f.project.String(),
		ObjectKey: f.key, CorrelationID: "c1",
	}
}

func TestHandleUploadedExtractsFrames(t *testing.T) {
	f := newFixture(t, uploaddomain.StatusUploaded)
	svc := service.New(f.store, f.objects, memory.FakeExtractor{}, service.ClaimConfig{Owner: "worker-a"})

	if err := svc.HandleUploaded(context.Background(), f.event()); err != nil {
		t.Fatal(err)
	}
	if f.store.Status(f.id) != uploaddomain.StatusReady {
		t.Fatalf("status=%s", f.store.Status(f.id))
	}
	arts, _ := f.store.ListArtifacts(context.Background(), f.id)
	if len(arts) < 2 {
		t.Fatalf("artifacts=%d", len(arts))
	}
	if len(f.store.Outbox()) != 1 {
		t.Fatalf("outbox=%d", len(f.store.Outbox()))
	}
	if _, held := f.store.Claim(f.id); held {
		t.Fatal("claim should be released once the recording is READY")
	}

	// idempotent second pass: republish the event, do not re-run ffmpeg
	if err := svc.HandleUploaded(context.Background(), f.event()); err != nil {
		t.Fatal(err)
	}
	if len(f.store.Outbox()) < 2 {
		t.Fatalf("expected republish outbox, got %d", len(f.store.Outbox()))
	}
	if _, held := f.store.Claim(f.id); held {
		t.Fatal("republish must not take a claim")
	}
}

func TestHandleUploadedSkipsWhenAnotherWorkerHoldsClaim(t *testing.T) {
	f := newFixture(t, uploaddomain.StatusProcessing)
	f.store.SeedClaim(f.id, "worker-b", time.Now().Add(time.Minute))
	extractor := &blockingExtractor{}
	svc := service.New(f.store, f.objects, extractor, service.ClaimConfig{Owner: "worker-a"})

	err := svc.HandleUploaded(context.Background(), f.event())
	if !errors.Is(err, domain.ErrClaimHeld) {
		t.Fatalf("err=%v, want ErrClaimHeld", err)
	}
	if extractor.calls.Load() != 0 {
		t.Fatalf("ffmpeg ran %d times under a live claim", extractor.calls.Load())
	}
	if claim, _ := f.store.Claim(f.id); claim.Owner != "worker-b" {
		t.Fatalf("claim owner=%q, want worker-b", claim.Owner)
	}
}

func TestConcurrentDeliveriesRunFFmpegOnce(t *testing.T) {
	f := newFixture(t, uploaddomain.StatusUploaded)
	extractor := &blockingExtractor{entered: make(chan struct{}, 2), release: make(chan struct{})}
	first := service.New(f.store, f.objects, extractor, service.ClaimConfig{Owner: "worker-a"})
	second := service.New(f.store, f.objects, extractor, service.ClaimConfig{Owner: "worker-b"})

	firstErr := make(chan error, 1)
	go func() { firstErr <- first.HandleUploaded(context.Background(), f.event()) }()
	<-extractor.entered // worker-a is mid-ffmpeg and holds the claim

	if err := second.HandleUploaded(context.Background(), f.event()); !errors.Is(err, domain.ErrClaimHeld) {
		t.Fatalf("second delivery err=%v, want ErrClaimHeld", err)
	}

	close(extractor.release)
	if err := <-firstErr; err != nil {
		t.Fatalf("first delivery: %v", err)
	}
	if got := extractor.calls.Load(); got != 1 {
		t.Fatalf("ffmpeg ran %d times, want 1", got)
	}
	if len(f.store.Outbox()) != 1 {
		t.Fatalf("outbox=%d, want 1 FramesExtracted", len(f.store.Outbox()))
	}
	if f.store.Status(f.id) != uploaddomain.StatusReady {
		t.Fatalf("status=%s", f.store.Status(f.id))
	}
}

func TestExpiredClaimIsReclaimed(t *testing.T) {
	f := newFixture(t, uploaddomain.StatusProcessing)
	// A worker died mid-job: status is PROCESSING but the lease has lapsed.
	f.store.SeedClaim(f.id, "dead-worker", time.Now().Add(-time.Minute))
	extractor := &blockingExtractor{}
	svc := service.New(f.store, f.objects, extractor, service.ClaimConfig{Owner: "worker-a"})

	if err := svc.HandleUploaded(context.Background(), f.event()); err != nil {
		t.Fatal(err)
	}
	if extractor.calls.Load() != 1 {
		t.Fatalf("ffmpeg ran %d times, want 1", extractor.calls.Load())
	}
	if f.store.Status(f.id) != uploaddomain.StatusReady {
		t.Fatalf("status=%s", f.store.Status(f.id))
	}
}

func TestClaimStolenMidJobAbortsWork(t *testing.T) {
	f := newFixture(t, uploaddomain.StatusUploaded)
	extractor := &blockingExtractor{entered: make(chan struct{}, 1), release: make(chan struct{})}
	defer close(extractor.release)
	svc := service.New(f.store, f.objects, extractor, service.ClaimConfig{
		Owner: "worker-a", Lease: 300 * time.Millisecond, RenewInterval: 10 * time.Millisecond,
	})

	errCh := make(chan error, 1)
	go func() { errCh <- svc.HandleUploaded(context.Background(), f.event()) }()
	<-extractor.entered
	f.store.SeedClaim(f.id, "worker-b", time.Now().Add(time.Minute))

	select {
	case err := <-errCh:
		if !errors.Is(err, domain.ErrClaimLost) {
			t.Fatalf("err=%v, want ErrClaimLost", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("renewal did not abort the job after the claim was stolen")
	}
	if len(f.store.Outbox()) != 0 {
		t.Fatalf("outbox=%d, want no event from an abandoned job", len(f.store.Outbox()))
	}
	if f.store.Status(f.id) == uploaddomain.StatusReady {
		t.Fatal("abandoned job must not finalize the recording")
	}
	if claim, _ := f.store.Claim(f.id); claim.Owner != "worker-b" {
		t.Fatalf("claim owner=%q, want worker-b", claim.Owner)
	}
}

func TestExtractionFailureReleasesClaim(t *testing.T) {
	f := newFixture(t, uploaddomain.StatusUploaded)
	extractor := &blockingExtractor{err: errors.New("ffmpeg exploded")}
	svc := service.New(f.store, f.objects, extractor, service.ClaimConfig{Owner: "worker-a"})

	if err := svc.HandleUploaded(context.Background(), f.event()); err == nil {
		t.Fatal("expected extraction error")
	}
	if f.store.Status(f.id) != uploaddomain.StatusFailed {
		t.Fatalf("status=%s, want FAILED", f.store.Status(f.id))
	}
	if _, held := f.store.Claim(f.id); held {
		t.Fatal("failed job must release the claim so a retry can pick it up")
	}

	// The retry is free to re-claim and succeed.
	retry := service.New(f.store, f.objects, memory.FakeExtractor{}, service.ClaimConfig{Owner: "worker-b"})
	if err := retry.HandleUploaded(context.Background(), f.event()); err != nil {
		t.Fatal(err)
	}
	if f.store.Status(f.id) != uploaddomain.StatusReady {
		t.Fatalf("status=%s, want READY", f.store.Status(f.id))
	}
}

func TestClaimRenewalExtendsLease(t *testing.T) {
	f := newFixture(t, uploaddomain.StatusUploaded)
	extractor := &blockingExtractor{entered: make(chan struct{}, 1), release: make(chan struct{})}
	svc := service.New(f.store, f.objects, extractor, service.ClaimConfig{
		Owner: "worker-a", Lease: 500 * time.Millisecond, RenewInterval: 20 * time.Millisecond,
	})

	errCh := make(chan error, 1)
	go func() { errCh <- svc.HandleUploaded(context.Background(), f.event()) }()
	<-extractor.entered

	before, _ := f.store.Claim(f.id)
	time.Sleep(200 * time.Millisecond)
	after, _ := f.store.Claim(f.id)
	if !after.ExpiresAt.After(before.ExpiresAt) {
		t.Fatalf("lease not renewed: before=%s after=%s", before.ExpiresAt, after.ExpiresAt)
	}

	close(extractor.release)
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}
