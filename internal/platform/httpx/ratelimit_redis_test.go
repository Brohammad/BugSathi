package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/httpx"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func TestRateLimitRedisRejects(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	cfg := config.RateLimitConfig{RPS: 1, Burst: 1, Window: time.Second}
	h := httpx.RateLimitRedis(cfg, nil, nil, rdb, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		t.Fatalf("second=%d", rr2.Code)
	}
}

func TestRateLimitRedisFailsClosed(t *testing.T) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:         "127.0.0.1:1",
		MaxRetries:   0,
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
	})
	cfg := config.RateLimitConfig{RPS: 10, Burst: 10, Window: time.Minute}
	h := httpx.RateLimitRedis(cfg, nil, nil, rdb, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("redis down should fail closed, got %d", rr.Code)
	}
}

func TestRedisWindowCapacityUsesRPSOrBurst(t *testing.T) {
	cfg := config.RateLimitConfig{RPS: 20, Burst: 5, Window: time.Minute}
	if got := httpx.RedisWindowCapacityForTest(cfg, false); got != 1200 {
		t.Fatalf("got %d want 1200 (RPS×window)", got)
	}
	cfg.Burst = 2000
	if got := httpx.RedisWindowCapacityForTest(cfg, false); got != 2000 {
		t.Fatalf("got %d want 2000 (burst)", got)
	}
}
