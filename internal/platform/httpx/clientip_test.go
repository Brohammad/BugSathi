package httpx_test

import (
	"net/http/httptest"
	"testing"

	"github.com/Brohammad/BugSathi/internal/platform/httpx"
)

func TestClientIPIgnoresXFFWhenProxyUntrusted(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.50:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")

	got := httpx.ClientIP(req, nil)
	if got != "203.0.113.50" {
		t.Fatalf("got %q want direct remote", got)
	}
}

func TestClientIPUsesXFFFromTrustedProxy(t *testing.T) {
	trusted, err := httpx.ParseTrustedNetworks([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.1.2.3:8080"
	req.Header.Set("X-Forwarded-For", "198.51.100.10, 10.1.2.3")

	got := httpx.ClientIP(req, trusted)
	if got != "198.51.100.10" {
		t.Fatalf("got %q want leftmost client IP", got)
	}
}

func TestClientIPPrefersXRealIPFromTrustedProxy(t *testing.T) {
	trusted, err := httpx.ParseTrustedNetworks([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Real-IP", "203.0.113.9")
	req.Header.Set("X-Forwarded-For", "1.1.1.1")

	got := httpx.ClientIP(req, trusted)
	if got != "203.0.113.9" {
		t.Fatalf("got %q want X-Real-IP", got)
	}
}

func TestParseTrustedNetworksRejectsBadCIDR(t *testing.T) {
	if _, err := httpx.ParseTrustedNetworks([]string{"not-a-cidr/99"}); err == nil {
		t.Fatal("expected error")
	}
}
