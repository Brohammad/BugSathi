package kafka

import (
	kafkago "github.com/segmentio/kafka-go"
)

const (
	HeaderCorrelationID = "correlation_id"
	HeaderRecordingID   = "recording_id"
)

// HeaderValue returns the first header value for key, or "".
func HeaderValue(msg kafkago.Message, key string) string {
	for _, h := range msg.Headers {
		if h.Key == key && len(h.Value) > 0 {
			return string(h.Value)
		}
	}
	return ""
}

// CorrelationID prefers the Kafka header, then falls back to payload-derived value.
func CorrelationID(msg kafkago.Message, payloadFallback string) string {
	if v := HeaderValue(msg, HeaderCorrelationID); v != "" {
		return v
	}
	return payloadFallback
}
