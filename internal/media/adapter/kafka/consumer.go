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
			GroupID:        "bugsathi-media",
			Topic:          domain.TopicRecordingUploaded,
			MinBytes:       1,
			MaxBytes:       10e6,
			MaxWait:        time.Second,
			CommitInterval: 0, // manual commit after successful handling
			StartOffset:    kafkago.FirstOffset,
		}),
		svc: svc,
		log: log,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("media consumer started", "topic", domain.TopicRecordingUploaded, "group", "bugsathi-media")
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
			// skip poison by committing
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				return err
			}
			continue
		}

		c.log.Info("processing RecordingUploaded",
			"recording_id", evt.RecordingID,
			"correlation_id", evt.CorrelationID,
			"partition", msg.Partition,
			"offset", msg.Offset,
		)

		if err := c.svc.HandleUploaded(ctx, evt); err != nil {
			c.log.Error("media handling failed", "error", err, "recording_id", evt.RecordingID)
			// do not commit — allow retry
			time.Sleep(time.Second)
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
