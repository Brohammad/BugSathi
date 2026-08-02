package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Brohammad/BugSathi/internal/platform/httpx"
	"github.com/Brohammad/BugSathi/internal/platform/observability"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHTTPMiddlewareMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg)
	h := httpx.RequestIDs(observability.Middleware("api", m, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`ok`))
	})))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d", rr.Code)
	}

	if err := testutil.GatherAndCompare(reg, strings.NewReader(`
# HELP bugsathi_http_requests_total HTTP requests by method, route, and status class.
# TYPE bugsathi_http_requests_total counter
bugsathi_http_requests_total{method="POST",route="/v1/projects",service="api",status="2xx"} 1
`), "bugsathi_http_requests_total"); err != nil {
		t.Fatal(err)
	}
}
