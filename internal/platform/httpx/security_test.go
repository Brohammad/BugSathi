package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Brohammad/BugSathi/internal/platform/httpx"
)

func TestSecurityHeadersCSP(t *testing.T) {
	h := httpx.SecurityHeaders(false, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if got := rr.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'none'") {
		t.Fatalf("csp=%q", got)
	}
	if rr.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS must not be set outside production")
	}
}

func TestSecurityHeadersHSTSInProduction(t *testing.T) {
	h := httpx.SecurityHeaders(true, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if got := rr.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains" {
		t.Fatalf("hsts=%q", got)
	}
}
