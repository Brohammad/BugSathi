package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Brohammad/BugSathi/internal/media/domain"
	"github.com/Brohammad/BugSathi/internal/media/service"
	"github.com/Brohammad/BugSathi/internal/platform/config"
	pkafka "github.com/Brohammad/BugSathi/internal/platform/kafka"
	"github.com/Brohammad/BugSathi/internal/platform/logging"
	"github.com/Brohammad/BugSathi/internal/platform/observability"
	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type Consumer struct {
	reader   pkafka.MessageReader
	svc      *service.Service
	log      *slog.Logger
	metrics  *observability.Metrics
	retry    config.KafkaRetryConfig
	pub      *pkafka.Publisher
	attempts *pkafka.AttemptTracker
	closer   func() error
}

func NewConsumer(
	cfg config.KafkaConfig,
	retry config.KafkaRetryConfig,
	svc *service.Service,
	log *slog.Logger,
	metrics *observability.Metrics,
	pub *pkafka.Publisher,
) *Consumer {
	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        cfg.Brokers,
		GroupID:        "bugsathi-media",
		Topic:          domain.TopicRecordingUploaded,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        time.Second,
		CommitInterval: 0,
		StartOffset:    kafkago.FirstOffset,
	})
	return &Consumer{
		reader:   r,
		svc:      svc,
		log:      log,
		metrics:  metrics,
		retry:    retry,
		pub:      pub,
		attempts: pkafka.NewAttemptTracker(),
		closer:   r.Close,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("media consumer started", "topic", domain.TopicRecordingUploaded, "group", "bugsathi-media")
	tr := observability.Tracer("bugsathi/media")
	for {
		msg, err := pkafka.FetchWithRetry(ctx, c.reader, c.retry, c.log)
		if err != nil {
			// ctx canceled / shutdown
			return nil
		}

		var evt domain.RecordingUploadedEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			c.log.Error("invalid RecordingUploaded payload", "error", err)
			if err := c.deadLetter(ctx, msg, 1, err); err != nil {
				return err
			}
			continue
		}

		msgCtx := logging.ContextWithCorrelationID(ctx, evt.CorrelationID)
		msgCtx = logging.ContextWithRecordingID(msgCtx, evt.RecordingID)
		log := logging.WithContext(msgCtx, c.log)
		log.Info("processing RecordingUploaded",
			"recording_id", evt.RecordingID,
			"partition", msg.Partition,
			"offset", msg.Offset,
		)

		err = pkafka.HandleWithRetries(ctx, c.reader, msg, c.retry, c.attempts,
			func(attemptCtx context.Context) error {
				start := time.Now()
				spanCtx, span := tr.Start(attemptCtx, "media.HandleUploaded")
				span.SetAttributes(
					attribute.String("recording_id", evt.RecordingID),
					attribute.String("correlation_id", evt.CorrelationID),
				)
				hErr := c.svc.HandleUploaded(spanCtx, evt)
				if hErr != nil {
					span.RecordError(hErr)
					span.SetStatus(codes.Error, hErr.Error())
					span.End()
					if c.metrics != nil {
						c.metrics.ObservePipeline("media", hErr, time.Since(start))
					}
					return hErr
				}
				span.End()
				if c.metrics != nil {
					c.metrics.ObservePipeline("media", nil, time.Since(start))
				}
				return nil
			},
			c.deadLetter,
			log,
		)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
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

func (c *Consumer) Close() error {
	if c.closer != nil {
		return c.closer()
	}
	return nil
}
