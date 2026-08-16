package kafka

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"
)

// MessageReader is the subset of kafka-go Reader used by pipeline consumers.
// *kafkago.Reader satisfies this interface.
type MessageReader interface {
	FetchMessage(ctx context.Context) (kafkago.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafkago.Message) error
}
