package kafka

import (
	"strings"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	kafkago "github.com/segmentio/kafka-go"
)

// Dialer returns a kafka-go dialer that advertises cfg.ClientID to brokers.
func Dialer(cfg config.KafkaConfig) *kafkago.Dialer {
	id := strings.TrimSpace(cfg.ClientID)
	if id == "" {
		id = "bugsathi"
	}
	return &kafkago.Dialer{
		Timeout:  10 * time.Second,
		ClientID: id,
	}
}
