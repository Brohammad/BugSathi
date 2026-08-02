package cache_test

import (
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/reports/adapter/cache"
	"github.com/Brohammad/BugSathi/internal/reports/domain"
	"github.com/google/uuid"
)

func TestReportCacheTTL(t *testing.T) {
	c := cache.NewReportCache(50 * time.Millisecond)
	id := uuid.New()
	c.Set(id, domain.Detail{Report: domain.Report{ID: id, Title: "x"}})
	if d, ok := c.Get(id); !ok || d.Report.Title != "x" {
		t.Fatalf("miss %+v %v", d, ok)
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get(id); ok {
		t.Fatal("expected expiry")
	}
}

func TestReportCacheDisabled(t *testing.T) {
	c := cache.NewReportCache(0)
	id := uuid.New()
	c.Set(id, domain.Detail{Report: domain.Report{ID: id}})
	if _, ok := c.Get(id); ok {
		t.Fatal("disabled cache should miss")
	}
}
