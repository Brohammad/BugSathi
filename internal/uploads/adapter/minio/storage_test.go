package minio

import (
	"testing"

	"github.com/Brohammad/BugSathi/internal/platform/config"
)

func TestNewSharesSignerWhenPublicEndpointUnset(t *testing.T) {
	s, err := New(config.MinIOConfig{
		Endpoint:  "minio:9000",
		AccessKey: "ak",
		SecretKey: "sk",
		Bucket:    "bugsathi",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.signer != s.client {
		t.Fatal("expected signer to reuse the internal client")
	}
	if got := s.client.EndpointURL(); got.Host != "minio:9000" || got.Scheme != "http" {
		t.Fatalf("internal endpoint = %s", got)
	}
}

func TestNewSignsAgainstPublicEndpoint(t *testing.T) {
	s, err := New(config.MinIOConfig{
		Endpoint:       "minio:9000",
		PublicEndpoint: "s3.example.com",
		PublicUseSSL:   true,
		AccessKey:      "ak",
		SecretKey:      "sk",
		Bucket:         "bugsathi",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.signer == s.client {
		t.Fatal("expected a distinct presign client")
	}
	internal := s.client.EndpointURL()
	public := s.signer.EndpointURL()
	if internal.Host != "minio:9000" || internal.Scheme != "http" {
		t.Fatalf("internal endpoint = %s", internal)
	}
	if public.Host != "s3.example.com" || public.Scheme != "https" {
		t.Fatalf("public endpoint = %s", public)
	}
}
