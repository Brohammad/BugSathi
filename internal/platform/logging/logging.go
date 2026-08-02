package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey string

const (
	RequestIDKey     ctxKey = "request_id"
	CorrelationIDKey ctxKey = "correlation_id"
	RecordingIDKey   ctxKey = "recording_id"
)

// New creates a JSON slog logger at the given level (debug|info|warn|error).
func New(level string) *slog.Logger {
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn", "warning":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lv})
	return slog.New(handler)
}

// WithContext returns a logger enriched with correlation fields from ctx.
func WithContext(ctx context.Context, log *slog.Logger) *slog.Logger {
	attrs := make([]any, 0, 6)
	if v, ok := ctx.Value(RequestIDKey).(string); ok && v != "" {
		attrs = append(attrs, "request_id", v)
	}
	if v, ok := ctx.Value(CorrelationIDKey).(string); ok && v != "" {
		attrs = append(attrs, "correlation_id", v)
	}
	if v, ok := ctx.Value(RecordingIDKey).(string); ok && v != "" {
		attrs = append(attrs, "recording_id", v)
	}
	if len(attrs) == 0 {
		return log
	}
	return log.With(attrs...)
}

func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

func ContextWithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, id)
}

func ContextWithRecordingID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RecordingIDKey, id)
}

func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(RequestIDKey).(string)
	return v
}

func CorrelationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(CorrelationIDKey).(string)
	return v
}
