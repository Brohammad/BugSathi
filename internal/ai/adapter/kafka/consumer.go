package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Brohammad/BugSathi/internal/ai/domain"
	"github.com/Brohammad/BugSathi/internal/ai/service"
	"github.com/Brohammad/BugSathi/internal/platform/config"
	pkafka "github.com/Brohammad/BugSathi/internal/platform/kafka"
	"github.com/Brohammad/BugSathi/internal/platform/logging"
	"github.com/Brohammad/BugSathi/internal/platform/observability"
	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type Consumer struct {
	reader   *kafkago.Reader
	svc      *service.Service
	log      *slog.Logger
	metrics  *observability.Metrics
	retry    config.KafkaRetryConfig
	pub      *pkafka.Publisher
	attempts *pkafka.AttemptTracker
}

func NewConsumer(
	cfg config.KafkaConfig,
	retry config.KafkaRetryConfig,
	svc *service.Service,
	log *slog.Logger,
	metrics *observability.Metrics,
	pub *pkafka.Publisher,
) *Consumer {
	return &Consumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:        cfg.Brokers,
			GroupID:        "bugsathi-ai",
			Topic:          domain.TopicFramesExtracted,
			MinBytes:       1,
			MaxBytes:       10e6,
			MaxWait:        time.Second,
			CommitInterval: 0,
			StartOffset:    kafkago.FirstOffset,
		}),
		svc:      svc,
		log:      log,
		metrics:  metrics,
		retry:    retry,
		pub:      pub,
		attempts: pkafka.NewAttemptTracker(),
	}
}

func (c *Consumer) maxAttempts() int {
	if c.retry.MaxAttempts <= 0 {
		return 5
	}
	return c.retry.MaxAttempts
}

func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("ai consumer started", "topic", domain.TopicFramesExtracted, "group", "bugsathi-ai")
	tr := observability.Tracer("bugsathi/ai")
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch: %w", err)
		}
		var evt domain.FramesExtractedEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			c.log.Error("invalid FramesExtracted payload", "error", err)
			if err := c.deadLetter(ctx, msg, 1, err); err != nil {
				return err
			}
			continue
		}
		msgCtx := logging.ContextWithCorrelationID(ctx, evt.CorrelationID)
		msgCtx = logging.ContextWithRecordingID(msgCtx, evt.RecordingID)
		log := logging.WithContext(msgCtx, c.log)
		log.Info("processing FramesExtracted",
			"recording_id", evt.RecordingID,
			"frames", len(evt.FrameKeys),
		)

		start := time.Now()
		spanCtx, span := tr.Start(msgCtx, "ai.HandleFramesExtracted")
		span.SetAttributes(
			attribute.String("recording_id", evt.RecordingID),
			attribute.String("correlation_id", evt.CorrelationID),
		)
		err = c.svc.HandleFramesExtracted(spanCtx, evt)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.End()
			if c.metrics != nil {
				c.metrics.ObservePipeline("ai", err, time.Since(start))
			}
			n := c.attempts.Inc(msg.Topic, msg.Partition, msg.Offset)
			log.Error("ai handling failed", "error", err, "recording_id", evt.RecordingID, "attempt", n)
			if n >= c.maxAttempts() {
				if err := c.deadLetter(ctx, msg, n, err); err != nil {
					return err
				}
				continue
			}
			time.Sleep(pkafka.Backoff(n, c.retry.Base, c.retry.Max))
			continue
		}
		span.End()
		if c.metrics != nil {
			c.metrics.ObservePipeline("ai", nil, time.Since(start))
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
		c.attempts.Clear(msg.Topic, msg.Partition, msg.Offset)
	}
}

func (c *Consumer) deadLetter(ctx context.Context, msg kafkago.Message, attempts int, cause error) error {
	if err := pkafka.PublishDeadLetter(ctx, c.pub, msg, attempts, cause); err != nil {
		return fmt.Errorf("publish dlq: %w", err)
	}
	if c.metrics != nil {
		c.metrics.IncDLQ(msg.Topic)
	}
	c.log.Warn("message dead-lettered",
		"topic", msg.Topic,
		"partition", msg.Partition,
		"offset", msg.Offset,
		"attempts", attempts,
		"error", cause,
	)
	c.attempts.Clear(msg.Topic, msg.Partition, msg.Offset)
	return c.reader.CommitMessages(ctx, msg)
}

func (c *Consumer) Close() error { return c.reader.Close() }
