package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Brohammad/BugSathi/internal/ai/domain"
	"github.com/Brohammad/BugSathi/internal/platform/config"
	pkafka "github.com/Brohammad/BugSathi/internal/platform/kafka"
	"github.com/Brohammad/BugSathi/internal/platform/logging"
	"github.com/Brohammad/BugSathi/internal/platform/observability"
	kafkago "github.com/segmentio/kafka-go"
)

// StartedConsumer observes AnalysisStarted events (metrics + structured logs).
// It does not drive pipeline work — that remains in the frames consumer.
type StartedConsumer struct {
	reader  pkafka.MessageReader
	log     *slog.Logger
	metrics *observability.Metrics
	retry   config.KafkaRetryConfig
	closer  func() error
}

func NewStartedConsumer(
	cfg config.KafkaConfig,
	retry config.KafkaRetryConfig,
	log *slog.Logger,
	metrics *observability.Metrics,
) *StartedConsumer {
	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        cfg.Brokers,
		GroupID:        "bugsathi-ai-started",
		Topic:          domain.TopicAnalysisStarted,
		MinBytes:       1,
		MaxBytes:       10e6,
		MaxWait:        time.Second,
		CommitInterval: 0,
		StartOffset:    kafkago.FirstOffset,
		Dialer:         pkafka.Dialer(cfg),
	})
	return &StartedConsumer{
		reader: r, log: log, metrics: metrics, retry: retry, closer: r.Close,
	}
}

func (c *StartedConsumer) Close() error {
	if c.closer == nil {
		return nil
	}
	return c.closer()
}

func (c *StartedConsumer) Run(ctx context.Context) error {
	c.log.Info("analysis started consumer started", "topic", domain.TopicAnalysisStarted, "group", "bugsathi-ai-started")
	for {
		msg, err := pkafka.FetchWithRetry(ctx, c.reader, c.retry, c.log)
		if err != nil {
			return err
		}
		var evt domain.AnalysisStartedEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			c.log.Error("analysis started payload invalid; committing", "error", err, "offset", msg.Offset)
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}
		msgCtx := logging.ContextWithCorrelationID(ctx, pkafka.CorrelationID(msg, evt.CorrelationID))
		if rid := pkafka.HeaderValue(msg, pkafka.HeaderRecordingID); rid != "" {
			msgCtx = logging.ContextWithRecordingID(msgCtx, rid)
		} else {
			msgCtx = logging.ContextWithRecordingID(msgCtx, evt.RecordingID)
		}
		log := logging.WithContext(msgCtx, c.log)
		log.Info("analysis started",
			"recording_id", evt.RecordingID,
			"report_id", evt.ReportID,
			"project_id", evt.ProjectID,
		)
		if c.metrics != nil {
			c.metrics.IncAnalysisStarted()
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}
