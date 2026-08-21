package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	kafkago "github.com/segmentio/kafka-go"
)

// DeadLetterFunc publishes a DLQ envelope and must commit the source message
// when successful (so the consumer can advance past poison offsets).
type DeadLetterFunc func(ctx context.Context, msg kafkago.Message, attempts int, cause error) error

// HandleWithRetries runs handle against the same Kafka message until it succeeds
// or attempts reach MaxAttempts. It never fetches the next message: callers must
// only FetchMessage again after this returns nil.
//
// On success: commits msg.
// On MaxAttempts: calls deadLetter (which is expected to DLQ + commit).
// On ctx cancel during backoff: returns ctx.Err() without committing.
func HandleWithRetries(
	ctx context.Context,
	reader MessageReader,
	msg kafkago.Message,
	retry config.KafkaRetryConfig,
	attempts AttemptCounter,
	handle func(context.Context) error,
	deadLetter DeadLetterFunc,
	log *slog.Logger,
) error {
	if log == nil {
		log = slog.Default()
	}
	max := retry.MaxAttempts
	if max <= 0 {
		max = 5
	}
	for {
		err := handle(ctx)
		if err == nil {
			if err := reader.CommitMessages(ctx, msg); err != nil {
				return fmt.Errorf("commit: %w", err)
			}
			if attempts != nil {
				attempts.Clear(msg.Topic, msg.Partition, msg.Offset)
			}
			return nil
		}

		n := 1
		if attempts != nil {
			n = attempts.Inc(msg.Topic, msg.Partition, msg.Offset)
		}
		log.Error("handler failed; will retry same offset",
			"topic", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
			"attempt", n,
			"max_attempts", max,
			"error", err,
		)
		if n >= max {
			if err := deadLetter(ctx, msg, n, err); err != nil {
				return fmt.Errorf("dead letter: %w", err)
			}
			return nil
		}
		if err := SleepCtx(ctx, Backoff(n, retry.Base, retry.Max)); err != nil {
			return err
		}
	}
}

// FetchWithRetry calls FetchMessage until it succeeds or ctx is done.
// Transient broker errors do not return to the caller — they are logged and retried.
// This keeps the worker process alive across brief Kafka blips.
func FetchWithRetry(
	ctx context.Context,
	reader MessageReader,
	retry config.KafkaRetryConfig,
	log *slog.Logger,
) (kafkago.Message, error) {
	if log == nil {
		log = slog.Default()
	}
	attempt := 0
	for {
		msg, err := reader.FetchMessage(ctx)
		if err == nil {
			return msg, nil
		}
		if ctx.Err() != nil {
			return kafkago.Message{}, ctx.Err()
		}
		attempt++
		delay := Backoff(attempt, retry.Base, retry.Max)
		log.Error("kafka fetch failed; retrying",
			"attempt", attempt,
			"backoff", delay.String(),
			"error", err,
		)
		if err := SleepCtx(ctx, delay); err != nil {
			return kafkago.Message{}, err
		}
	}
}

// SleepCtx waits for d or returns when ctx is canceled.
func SleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
