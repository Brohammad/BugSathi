package kafka_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/kafka"
	kafkago "github.com/segmentio/kafka-go"
)

type fetchOp struct {
	msg kafkago.Message
	err error
}

// fakeReader scripts FetchMessage results and records commits.
// Each FetchMessage consumes one op (like kafka-go advancing the cursor).
type fakeReader struct {
	mu      sync.Mutex
	ops     []fetchOp
	commits []kafkago.Message
}

func (f *fakeReader) FetchMessage(ctx context.Context) (kafkago.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return kafkago.Message{}, err
	}
	if len(f.ops) == 0 {
		// Block until canceled — mimics a quiet partition.
		f.mu.Unlock()
		<-ctx.Done()
		f.mu.Lock()
		return kafkago.Message{}, ctx.Err()
	}
	op := f.ops[0]
	f.ops = f.ops[1:]
	return op.msg, op.err
}

func (f *fakeReader) CommitMessages(_ context.Context, msgs ...kafkago.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits = append(f.commits, msgs...)
	return nil
}

func (f *fakeReader) committedOffsets() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]int64, len(f.commits))
	for i, m := range f.commits {
		out[i] = m.Offset
	}
	return out
}

func testRetry() config.KafkaRetryConfig {
	return config.KafkaRetryConfig{
		Base:        time.Microsecond,
		Max:         time.Millisecond,
		MaxAttempts: 3,
	}
}

func silentLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func msgAt(topic string, offset int64, value string) kafkago.Message {
	return kafkago.Message{
		Topic:     topic,
		Partition: 0,
		Offset:    offset,
		Key:       []byte("rec-1"),
		Value:     []byte(value),
	}
}

func TestHandleWithRetries_RetriesSameOffsetUntilSuccess(t *testing.T) {
	r := &fakeReader{}
	msg := msgAt("bugsathi.recording.uploaded", 42, `{}`)
	attempts := kafka.NewAttemptTracker()
	calls := 0

	err := kafka.HandleWithRetries(
		context.Background(),
		r,
		msg,
		testRetry(),
		attempts,
		func(context.Context) error {
			calls++
			if calls < 3 {
				return errors.New("transient")
			}
			return nil
		},
		func(context.Context, kafkago.Message, int, error) error {
			t.Fatal("deadLetter must not run on success")
			return nil
		},
		silentLog(),
	)
	if err != nil {
		t.Fatalf("HandleWithRetries: %v", err)
	}
	if calls != 3 {
		t.Fatalf("handler calls=%d want 3 (2 failures + 1 success)", calls)
	}
	got := r.committedOffsets()
	if len(got) != 1 || got[0] != 42 {
		t.Fatalf("commits=%v want [42]", got)
	}
}

func TestHandleWithRetries_DeadLettersAfterMaxAttempts(t *testing.T) {
	r := &fakeReader{}
	msg := msgAt("bugsathi.recording.uploaded", 7, `{}`)
	attempts := kafka.NewAttemptTracker()
	calls := 0
	dlqAttempts := 0
	var dlqCause error

	err := kafka.HandleWithRetries(
		context.Background(),
		r,
		msg,
		testRetry(), // MaxAttempts=3
		attempts,
		func(context.Context) error {
			calls++
			return errors.New("still broken")
		},
		func(_ context.Context, m kafkago.Message, n int, cause error) error {
			dlqAttempts = n
			dlqCause = cause
			if m.Offset != 7 {
				t.Fatalf("dlq offset=%d", m.Offset)
			}
			// Production deadLetter commits after publish.
			return r.CommitMessages(context.Background(), m)
		},
		silentLog(),
	)
	if err != nil {
		t.Fatalf("HandleWithRetries: %v", err)
	}
	if calls != 3 {
		t.Fatalf("handler calls=%d want 3", calls)
	}
	if dlqAttempts != 3 {
		t.Fatalf("dlq attempts=%d want 3", dlqAttempts)
	}
	if dlqCause == nil || dlqCause.Error() != "still broken" {
		t.Fatalf("dlq cause=%v", dlqCause)
	}
	got := r.committedOffsets()
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("commits=%v want [7] after DLQ", got)
	}
}

func TestHandleWithRetries_DoesNotAdvancePastFailedMessage(t *testing.T) {
	// Simulates the consumer outer loop: fetch → handle-with-retries → fetch next.
	// A buggy continue-after-failure would fetch offset 2 before committing 1.
	r := &fakeReader{
		ops: []fetchOp{
			{msg: msgAt("t", 1, `a`)},
			{msg: msgAt("t", 2, `b`)},
		},
	}
	retry := testRetry()
	attempts := kafka.NewAttemptTracker()
	var handled []int64

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	processOne := func() error {
		msg, err := kafka.FetchWithRetry(ctx, r, retry, silentLog())
		if err != nil {
			return err
		}
		failsLeft := 0
		if msg.Offset == 1 {
			failsLeft = 2 // fail twice on offset 1, then succeed
		}
		return kafka.HandleWithRetries(ctx, r, msg, retry, attempts,
			func(context.Context) error {
				handled = append(handled, msg.Offset)
				if failsLeft > 0 {
					failsLeft--
					return errors.New("fail")
				}
				return nil
			},
			func(context.Context, kafkago.Message, int, error) error {
				t.Fatal("unexpected DLQ")
				return nil
			},
			silentLog(),
		)
	}

	if err := processOne(); err != nil {
		t.Fatalf("first message: %v", err)
	}
	if err := processOne(); err != nil {
		t.Fatalf("second message: %v", err)
	}

	// Offset 1 must be attempted 3 times before offset 2 is ever handled.
	wantHandled := []int64{1, 1, 1, 2}
	if len(handled) != len(wantHandled) {
		t.Fatalf("handled=%v want %v", handled, wantHandled)
	}
	for i := range wantHandled {
		if handled[i] != wantHandled[i] {
			t.Fatalf("handled=%v want %v", handled, wantHandled)
		}
	}
	got := r.committedOffsets()
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("commits=%v want [1 2]", got)
	}
}

func TestFetchWithRetry_SurvivesTransientErrors(t *testing.T) {
	r := &fakeReader{
		ops: []fetchOp{
			{err: errors.New("broker unavailable")},
			{err: errors.New("connection reset")},
			{msg: msgAt("t", 9, `ok`)},
		},
	}
	retry := config.KafkaRetryConfig{
		Base:        time.Microsecond,
		Max:         time.Millisecond,
		MaxAttempts: 5,
	}
	msg, err := kafka.FetchWithRetry(context.Background(), r, retry, silentLog())
	if err != nil {
		t.Fatalf("FetchWithRetry returned error %v (must retry transient fetch failures)", err)
	}
	if msg.Offset != 9 {
		t.Fatalf("offset=%d want 9", msg.Offset)
	}
}

func TestFetchWithRetry_ReturnsOnContextCancel(t *testing.T) {
	r := &fakeReader{
		ops: []fetchOp{
			{err: errors.New("broker unavailable")},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel during the backoff after the first failed fetch.
	go func() {
		time.Sleep(2 * time.Millisecond)
		cancel()
	}()
	retry := config.KafkaRetryConfig{Base: 50 * time.Millisecond, Max: 50 * time.Millisecond}
	_, err := kafka.FetchWithRetry(ctx, r, retry, silentLog())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
}

func TestRunLoop_FetchErrorsDoNotStopConsumption(t *testing.T) {
	// Mirrors cmd/worker: if Run returns, stop() cancels everything.
	// FetchWithRetry must absorb transient errors so Run keeps going.
	r := &fakeReader{
		ops: []fetchOp{
			{err: errors.New("i/o timeout")},
			{msg: msgAt("bugsathi.recording.uploaded", 1, `{"schema_version":1}`)},
		},
	}
	retry := testRetry()
	attempts := kafka.NewAttemptTracker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runOnce := func() error {
		msg, err := kafka.FetchWithRetry(ctx, r, retry, silentLog())
		if err != nil {
			return err // only shutdown
		}
		return kafka.HandleWithRetries(ctx, r, msg, retry, attempts,
			func(context.Context) error { return nil },
			func(context.Context, kafkago.Message, int, error) error {
				t.Fatal("unexpected DLQ")
				return nil
			},
			silentLog(),
		)
	}

	if err := runOnce(); err != nil {
		t.Fatalf("runOnce returned %v — transient fetch must not terminate the loop", err)
	}
	if got := r.committedOffsets(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("commits=%v", got)
	}
}
