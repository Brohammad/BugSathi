package httpx

import (
	"net/http"

	"github.com/Brohammad/BugSathi/internal/platform/logging"
	"github.com/google/uuid"
)

const (
	HeaderRequestID     = "X-Request-ID"
	HeaderCorrelationID = "X-Correlation-ID"
)

// RequestIDs injects request_id and correlation_id into the request context and response headers.
func RequestIDs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(HeaderRequestID)
		if reqID == "" {
			reqID = uuid.NewString()
		}
		corrID := r.Header.Get(HeaderCorrelationID)
		if corrID == "" {
			corrID = reqID
		}

		ctx := logging.ContextWithRequestID(r.Context(), reqID)
		ctx = logging.ContextWithCorrelationID(ctx, corrID)

		w.Header().Set(HeaderRequestID, reqID)
		w.Header().Set(HeaderCorrelationID, corrID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
