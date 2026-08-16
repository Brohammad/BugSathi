package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/httpx"
)

func TestCORSPreflight(t *testing.T) {
	h := httpx.CORS([]string{"http://localhost:5173"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/v1/projects", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("preflight=%d", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("origin=%q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	h := httpx.CORS([]string{"http://localhost:5173"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	req.Header.Set("Origin", "http://evil.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unexpected allow origin")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestRateLimitRejects(t *testing.T) {
	cfg := config.RateLimitConfig{RPS: 1, Burst: 1, Window: 60}
	h := httpx.RateLimit(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	req.RemoteAddr = "203.0.113.1:1234"

	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, req)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first=%d", rr1.Code)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req)
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second=%d want 429", rr2.Code)
	}
	if rr2.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
}
