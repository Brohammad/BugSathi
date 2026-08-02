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
	"github.com/Brohammad/BugSathi/internal/platform/logging"
	"github.com/Brohammad/BugSathi/internal/platform/observability"
	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type Consumer struct {
	reader  *kafkago.Reader
	svc     *service.Service
	log     *slog.Logger
	metrics *observability.Metrics
}

func NewConsumer(cfg config.KafkaConfig, svc *service.Service, log *slog.Logger, metrics *observability.Metrics) *Consumer {
	return &Consumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:        cfg.Brokers,
			GroupID:        "bugsathi-media",
			Topic:          domain.TopicRecordingUploaded,
			MinBytes:       1,
			MaxBytes:       10e6,
			MaxWait:        time.Second,
			CommitInterval: 0,
			StartOffset:    kafkago.FirstOffset,
		}),
		svc:     svc,
		log:     log,
		metrics: metrics,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("media consumer started", "topic", domain.TopicRecordingUploaded, "group", "bugsathi-media")
	tr := observability.Tracer("bugsathi/media")
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetch: %w", err)
		}

		var evt domain.RecordingUploadedEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			c.log.Error("invalid RecordingUploaded payload", "error", err)
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
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

		start := time.Now()
		spanCtx, span := tr.Start(msgCtx, "media.HandleUploaded")
		span.SetAttributes(
			attribute.String("recording_id", evt.RecordingID),
			attribute.String("correlation_id", evt.CorrelationID),
		)
		err = c.svc.HandleUploaded(spanCtx, evt)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.End()
			if c.metrics != nil {
				c.metrics.ObservePipeline("media", err, time.Since(start))
			}
			log.Error("media handling failed", "error", err, "recording_id", evt.RecordingID)
			time.Sleep(time.Second)
			continue
		}
		span.End()
		if c.metrics != nil {
			c.metrics.ObservePipeline("media", nil, time.Since(start))
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
