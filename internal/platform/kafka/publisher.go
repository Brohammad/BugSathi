package kafka

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	kafkago "github.com/segmentio/kafka-go"
)

type Publisher struct {
	writer *kafkago.Writer
}

func NewPublisher(cfg config.KafkaConfig) *Publisher {
	return &Publisher{
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(cfg.Brokers...),
			Balancer:     &kafkago.Hash{},
			RequiredAcks: kafkago.RequireOne,
			Async:        false,
			BatchTimeout: 10 * time.Millisecond,
		},
	}
}

func (p *Publisher) Publish(ctx context.Context, topic, key string, payload []byte, headers map[string]string) error {
	msg := kafkago.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now().UTC(),
	}
	for k, v := range headers {
		msg.Headers = append(msg.Headers, kafkago.Header{Key: k, Value: []byte(v)})
	}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka publish %s: %w", topic, err)
	}
	return nil
}

func (p *Publisher) Close() error {
	if p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

// EnsureTopic creates a topic if the broker allows it (best-effort for local Redpanda).
func EnsureTopic(brokers []string, topic string, partitions int) error {
	if partitions <= 0 {
		partitions = 3
	}
	conn, err := kafkago.Dial("tcp", brokers[0])
	if err != nil {
		return err
	}
	defer conn.Close()
	controller, err := conn.Controller()
	if err != nil {
		return err
	}
	controllerConn, err := kafkago.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		return err
	}
	defer controllerConn.Close()
	err = controllerConn.CreateTopics(kafkago.TopicConfig{
		Topic:             topic,
		NumPartitions:     partitions,
		ReplicationFactor: 1,
	})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}
	return nil
}
