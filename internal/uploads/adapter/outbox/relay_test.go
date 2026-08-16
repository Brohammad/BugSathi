package outbox_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/uploads/adapter/memory"
	"github.com/Brohammad/BugSathi/internal/uploads/adapter/outbox"
	"github.com/Brohammad/BugSathi/internal/uploads/port"
)

type spyPublisher struct {
	mu       sync.Mutex
	topics   []string
	keys     []string
	delay    time.Duration
	failOnce atomic.Bool
	calls    atomic.Int64
}

func (s *spyPublisher) Publish(_ context.Context, topic, key string, _ []byte, _ map[string]string) error {
	s.calls.Add(1)
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	if s.failOnce.CompareAndSwap(true, false) {
		return errors.New("kafka unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.topics = append(s.topics, topic)
	s.keys = append(s.keys, key)
	return nil
}

func (s *spyPublisher) publishedKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.keys...)
}

func TestRelayFlush_PublishesAndMarks(t *testing.T) {
	repo := memory.NewOutboxRepo()
	recs := memory.NewRecordingRepo(repo)
	if err := recs.InsertOutbox(context.Background(), "bugsathi.recording.uploaded", "rec-a", []byte(`{}`), "c1"); err != nil {
		t.Fatal(err)
	}
	pub := &spyPublisher{}
	relay := outbox.NewRelay(repo, pub, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := relay.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if pub.calls.Load() != 1 {
		t.Fatalf("publish calls=%d want 1", pub.calls.Load())
	}
	// Second flush should claim nothing.
	if err := relay.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if pub.calls.Load() != 1 {
		t.Fatalf("after second flush calls=%d want 1", pub.calls.Load())
	}
}

func TestRelayFlush_ConcurrentRelaysDoNotDoublePublish(t *testing.T) {
	repo := memory.NewOutboxRepo()
	recs := memory.NewRecordingRepo(repo)
	if err := recs.InsertOutbox(context.Background(), "bugsathi.recording.uploaded", "rec-shared", []byte(`{}`), "c1"); err != nil {
		t.Fatal(err)
	}

	pub := &spyPublisher{delay: 20 * time.Millisecond}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	apiRelay := outbox.NewRelay(repo, pub, log)
	workerRelay := outbox.NewRelay(repo, pub, log)

	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	go func() {
		defer wg.Done()
		errs <- apiRelay.Flush(context.Background())
	}()
	go func() {
		defer wg.Done()
		errs <- workerRelay.Flush(context.Background())
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("flush: %v", err)
		}
	}

	if pub.calls.Load() != 1 {
		t.Fatalf("concurrent relays published %d times want 1 (double-claim)", pub.calls.Load())
	}
	keys := pub.publishedKeys()
	if len(keys) != 1 || keys[0] != "rec-shared" {
		t.Fatalf("keys=%v", keys)
	}
}

func TestRelayFlush_PublishFailureLeavesRowClaimable(t *testing.T) {
	repo := memory.NewOutboxRepo()
	recs := memory.NewRecordingRepo(repo)
	if err := recs.InsertOutbox(context.Background(), "bugsathi.recording.uploaded", "rec-retry", []byte(`{}`), "c1"); err != nil {
		t.Fatal(err)
	}
	pub := &spyPublisher{}
	pub.failOnce.Store(true)
	relay := outbox.NewRelay(repo, pub, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := relay.Flush(context.Background()); err == nil {
		t.Fatal("expected publish error")
	}
	// Row must still be unpublished so a later flush can succeed.
	if err := relay.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if pub.calls.Load() != 2 {
		t.Fatalf("calls=%d want 2 (fail then success)", pub.calls.Load())
	}
	keys := pub.publishedKeys()
	if len(keys) != 1 || keys[0] != "rec-retry" {
		t.Fatalf("keys=%v want [rec-retry]", keys)
	}
}

func TestWithClaimed_EmptyBatch(t *testing.T) {
	repo := memory.NewOutboxRepo()
	called := false
	err := repo.WithClaimed(context.Background(), 10, func(context.Context, []port.OutboxMessage) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("fn must not run when there is nothing to claim")
	}
}
