package kafka_test

import (
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/kafka"
)

func TestBackoffCaps(t *testing.T) {
	max := 5 * time.Second
	d := kafka.Backoff(10, time.Second, max)
	if d > max+max/4 || d < max-max/4 {
		t.Fatalf("expected ~max, got %v", d)
	}
}

func TestBackoffGrows(t *testing.T) {
	a := kafka.Backoff(1, time.Second, time.Minute)
	b := kafka.Backoff(3, time.Second, time.Minute)
	if b <= a {
		t.Fatalf("attempt 3 should backoff longer: %v vs %v", b, a)
	}
}
