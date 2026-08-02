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
	kafkago "github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafkago.Reader
	svc    *service.Service
	log    *slog.Logger
}

func NewConsumer(cfg config.KafkaConfig, svc *service.Service, log *slog.Logger) *Consumer {
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
		svc: svc,
		log: log,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("ai consumer started", "topic", domain.TopicFramesExtracted, "group", "bugsathi-ai")
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
		c.log.Info("processing FramesExtracted",
			"recording_id", evt.RecordingID,
			"correlation_id", evt.CorrelationID,
			"frames", len(evt.FrameKeys),
		)
		if err := c.svc.HandleFramesExtracted(ctx, evt); err != nil {
			c.log.Error("ai handling failed", "error", err, "recording_id", evt.RecordingID)
			time.Sleep(time.Second)
			continue
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}

func (c *Consumer) Close() error { return c.reader.Close() }
