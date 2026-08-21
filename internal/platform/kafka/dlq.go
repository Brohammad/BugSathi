package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

// DLQTopic returns the dead-letter topic name for a source topic.
func DLQTopic(source string) string {
	return source + ".dlq"
}

// DeadLetter is the envelope written to *.dlq topics.
type DeadLetter struct {
	SchemaVersion  int             `json:"schema_version"`
	SourceTopic    string          `json:"source_topic"`
	SourcePartition int            `json:"source_partition"`
	SourceOffset   int64           `json:"source_offset"`
	Key            string          `json:"key,omitempty"`
	Attempts       int             `json:"attempts"`
	Error          string          `json:"error"`
	Payload        json.RawMessage `json:"payload"`
	DeadLetteredAt time.Time       `json:"dead_lettered_at"`
}

// AttemptCounter tracks consecutive handler failures for a Kafka message identity.
// Implementations may be in-process or durable (Postgres).
type AttemptCounter interface {
	Inc(topic string, partition int, offset int64) int
	Clear(topic string, partition int, offset int64)
}

// AttemptTracker counts consecutive failures in process memory.
type AttemptTracker struct {
	mu   sync.Mutex
	byID map[string]int
}

func NewAttemptTracker() *AttemptTracker {
	return &AttemptTracker{byID: map[string]int{}}
}

func msgID(topic string, partition int, offset int64) string {
	return fmt.Sprintf("%s/%d/%d", topic, partition, offset)
}

// Inc increments and returns the attempt count for the message.
func (t *AttemptTracker) Inc(topic string, partition int, offset int64) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	id := msgID(topic, partition, offset)
	t.byID[id]++
	return t.byID[id]
}

// Clear removes the attempt counter after success or DLQ.
func (t *AttemptTracker) Clear(topic string, partition int, offset int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.byID, msgID(topic, partition, offset))
}

// PublishDeadLetter writes an envelope to the source topic's DLQ.
func PublishDeadLetter(ctx context.Context, pub *Publisher, msg kafkago.Message, attempts int, cause error) error {
	if pub == nil {
		return fmt.Errorf("dlq publisher is nil")
	}
	errMsg := "unknown"
	if cause != nil {
		errMsg = cause.Error()
	}
	env := DeadLetter{
		SchemaVersion:   1,
		SourceTopic:     msg.Topic,
		SourcePartition: msg.Partition,
		SourceOffset:    msg.Offset,
		Key:             string(msg.Key),
		Attempts:        attempts,
		Error:           errMsg,
		Payload:         append(json.RawMessage(nil), msg.Value...),
		DeadLetteredAt:  time.Now().UTC(),
	}
	body, err := json.Marshal(env)
	if err != nil {
		return err
	}
	key := string(msg.Key)
	if key == "" {
		key = fmt.Sprintf("%d-%d", msg.Partition, msg.Offset)
	}
	return pub.Publish(ctx, DLQTopic(msg.Topic), key, body, map[string]string{
		"x-bugsathi-dlq":          "1",
		"x-bugsathi-source-topic": msg.Topic,
	})
}
