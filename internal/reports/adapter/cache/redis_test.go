package cache_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/reports/adapter/cache"
	"github.com/Brohammad/BugSathi/internal/reports/domain"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/google/uuid"
)

func TestRedisReportCacheRoundTrip(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	c := cache.NewRedisReportCache(rdb, time.Minute)

	id := uuid.New()
	want := domain.Detail{
		Report: domain.Report{
			ID: id, Title: "Bug", Summary: "s", Steps: json.RawMessage(`[]`),
		},
		RecordingStatus: "READY",
		Metadata:        json.RawMessage(`{}`),
	}
	c.Set(id, want)
	got, ok := c.Get(id)
	if !ok || got.Report.Title != "Bug" {
		t.Fatalf("get ok=%v got=%+v", ok, got)
	}
}
