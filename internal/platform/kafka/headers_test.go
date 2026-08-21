package kafka_test

import (
	"testing"

	"github.com/Brohammad/BugSathi/internal/platform/kafka"
	kafkago "github.com/segmentio/kafka-go"
)

func TestCorrelationIDPrefersHeader(t *testing.T) {
	msg := kafkago.Message{
		Headers: []kafkago.Header{
			{Key: kafka.HeaderCorrelationID, Value: []byte("from-header")},
		},
	}
	if got := kafka.CorrelationID(msg, "from-payload"); got != "from-header" {
		t.Fatalf("got %q", got)
	}
}

func TestCorrelationIDFallsBackToPayload(t *testing.T) {
	msg := kafkago.Message{}
	if got := kafka.CorrelationID(msg, "from-payload"); got != "from-payload" {
		t.Fatalf("got %q", got)
	}
}
