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
	reader     *kafkago.Reader
	svc        *service.Service
	log        *slog.Logger
	metrics    *observability.Metrics
	retry      config.KafkaRetryConfig
	failStreak int
}

func NewConsumer(cfg config.KafkaConfig, retry config.KafkaRetryConfig, svc *service.Service, log *slog.Logger, metrics *observability.Metrics) *Consumer {
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
		svc:     svc,
		log:     log,
		metrics: metrics,
		retry:   retry,
	}
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
			_ = c.reader.CommitMessages(ctx, msg)
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
			log.Error("ai handling failed", "error", err, "recording_id", evt.RecordingID)
			c.failStreak++
			time.Sleep(pkafka.Backoff(c.failStreak, c.retry.Base, c.retry.Max))
			continue
		}
		span.End()
		if c.metrics != nil {
			c.metrics.ObservePipeline("ai", nil, time.Since(start))
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
		c.failStreak = 0
	}
}

func (c *Consumer) Close() error { return c.reader.Close() }
