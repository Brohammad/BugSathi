package observability

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
)

// Metrics holds Prometheus instruments for API and worker.
type Metrics struct {
	HTTPRequests    *prometheus.CounterVec
	HTTPDuration    *prometheus.HistogramVec
	PipelineJobs    *prometheus.CounterVec
	PipelineDuration *prometheus.HistogramVec
	AIDuration      *prometheus.HistogramVec
	OutboxPending   prometheus.Gauge
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bugsathi_http_requests_total",
			Help: "HTTP requests by method, route, and status class.",
		}, []string{"service", "method", "route", "status"}),
		HTTPDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "bugsathi_http_request_duration_seconds",
			Help:    "HTTP request latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"service", "method", "route", "status"}),
		PipelineJobs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bugsathi_pipeline_jobs_total",
			Help: "Pipeline jobs by stage and result.",
		}, []string{"stage", "result"}),
		PipelineDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "bugsathi_pipeline_duration_seconds",
			Help:    "Pipeline stage latency.",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 15, 30, 60, 120},
		}, []string{"stage"}),
		AIDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "bugsathi_ai_analyze_duration_seconds",
			Help:    "AI analyzer call latency.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 15, 30, 60},
		}, []string{"provider", "result"}),
		OutboxPending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "bugsathi_outbox_pending",
			Help: "Unpublished outbox rows.",
		}),
	}
	reg.MustRegister(m.HTTPRequests, m.HTTPDuration, m.PipelineJobs, m.PipelineDuration, m.AIDuration, m.OutboxPending)
	return m
}

func (m *Metrics) ObserveHTTP(service, method, route string, status int, d time.Duration) {
	st := statusClass(status)
	m.HTTPRequests.WithLabelValues(service, method, route, st).Inc()
	m.HTTPDuration.WithLabelValues(service, method, route, st).Observe(d.Seconds())
}

func (m *Metrics) ObservePipeline(stage string, err error, d time.Duration) {
	result := "ok"
	if err != nil {
		result = "error"
	}
	m.PipelineJobs.WithLabelValues(stage, result).Inc()
	m.PipelineDuration.WithLabelValues(stage).Observe(d.Seconds())
}

func (m *Metrics) ObserveAI(provider string, err error, d time.Duration) {
	result := "ok"
	if err != nil {
		result = "error"
	}
	m.AIDuration.WithLabelValues(provider, result).Observe(d.Seconds())
}

func statusClass(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	case code >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}

// Handler returns the Prometheus scrape handler.
func Handler(reg prometheus.Gatherer) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}

// SetupTracing configures a global TracerProvider. Empty endpoint = noop.
func SetupTracing(ctx context.Context, service, endpoint string) (func(context.Context) error, error) {
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	opts := []otlptracehttp.Option{otlptracehttp.WithInsecure()}
	if strings.Contains(endpoint, "://") {
		opts = append(opts, otlptracehttp.WithEndpointURL(endpoint))
	} else {
		opts = append(opts, otlptracehttp.WithEndpoint(endpoint))
	}
	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(service),
		),
	)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp.Shutdown, nil
}

func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func SpanAttrsFromContext(ctx context.Context) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	if v := logging.CorrelationIDFromContext(ctx); v != "" {
		attrs = append(attrs, attribute.String("correlation_id", v))
	}
	if v := logging.RequestIDFromContext(ctx); v != "" {
		attrs = append(attrs, attribute.String("request_id", v))
	}
	if v, ok := ctx.Value(logging.RecordingIDKey).(string); ok && v != "" {
		attrs = append(attrs, attribute.String("recording_id", v))
	}
	return attrs
}
