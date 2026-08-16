package kafka_test

import (
	"testing"

	"github.com/Brohammad/BugSathi/internal/platform/kafka"
)

func TestDLQTopic(t *testing.T) {
	got := kafka.DLQTopic("bugsathi.recording.uploaded")
	want := "bugsathi.recording.uploaded.dlq"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestAttemptTracker(t *testing.T) {
	tr := kafka.NewAttemptTracker()
	if n := tr.Inc("t", 0, 1); n != 1 {
		t.Fatalf("n=%d", n)
	}
	if n := tr.Inc("t", 0, 1); n != 2 {
		t.Fatalf("n=%d", n)
	}
	if n := tr.Inc("t", 0, 2); n != 1 {
		t.Fatalf("other offset n=%d", n)
	}
	tr.Clear("t", 0, 1)
	if n := tr.Inc("t", 0, 1); n != 1 {
		t.Fatalf("after clear n=%d", n)
	}
}
