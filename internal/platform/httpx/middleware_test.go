package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDsGeneratesAndEchoes(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := RequestIDs(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get(HeaderRequestID) == "" {
		t.Fatal("expected X-Request-ID")
	}
	if rr.Header().Get(HeaderCorrelationID) == "" {
		t.Fatal("expected X-Correlation-ID")
	}
}

func TestRequestIDsPreservesIncoming(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := RequestIDs(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderRequestID, "req-1")
	req.Header.Set(HeaderCorrelationID, "corr-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get(HeaderRequestID); got != "req-1" {
		t.Fatalf("request id = %q", got)
	}
	if got := rr.Header().Get(HeaderCorrelationID); got != "corr-1" {
		t.Fatalf("correlation id = %q", got)
	}
}
