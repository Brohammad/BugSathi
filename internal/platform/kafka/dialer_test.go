package kafka_test

import (
	"testing"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/kafka"
)

func TestDialerUsesClientID(t *testing.T) {
	d := kafka.Dialer(config.KafkaConfig{ClientID: "bugsathi-api"})
	if d.ClientID != "bugsathi-api" {
		t.Fatalf("ClientID=%q", d.ClientID)
	}
}

func TestDialerDefaultClientID(t *testing.T) {
	d := kafka.Dialer(config.KafkaConfig{})
	if d.ClientID != "bugsathi" {
		t.Fatalf("ClientID=%q", d.ClientID)
	}
}
