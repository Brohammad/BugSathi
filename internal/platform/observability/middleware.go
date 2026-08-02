package observability

import (
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Middleware records RED metrics and an HTTP server span.
func Middleware(service string, m *Metrics, next http.Handler) http.Handler {
	tr := Tracer("bugsathi/http")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		route := r.Pattern
		if route == "" {
			route = r.URL.Path
		}
		ctx, span := tr.Start(r.Context(), fmt.Sprintf("%s %s", r.Method, route))
		defer span.End()
		span.SetAttributes(append(
			SpanAttrsFromContext(ctx),
			attribute.String("http.method", r.Method),
			attribute.String("http.route", route),
		)...)

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r.WithContext(ctx))

		if sw.status >= 500 {
			span.SetStatus(codes.Error, http.StatusText(sw.status))
		}
		span.SetAttributes(attribute.Int("http.status_code", sw.status))
		m.ObserveHTTP(service, r.Method, route, sw.status, time.Since(start))
	})
}
