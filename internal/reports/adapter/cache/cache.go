package cache

import (
	"sync"
	"time"

	"github.com/Brohammad/BugSathi/internal/reports/domain"
	"github.com/google/uuid"
)

type entry struct {
	detail domain.Detail
	exp    time.Time
}

// ReportCache is a process-local TTL cache for report detail aggregates.
type ReportCache struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[uuid.UUID]entry
	now func() time.Time
}

func NewReportCache(ttl time.Duration) *ReportCache {
	return &ReportCache{
		ttl: ttl,
		m:   make(map[uuid.UUID]entry),
		now: time.Now,
	}
}

func (c *ReportCache) Enabled() bool {
	return c != nil && c.ttl > 0
}

func (c *ReportCache) Get(id uuid.UUID) (domain.Detail, bool) {
	if !c.Enabled() {
		return domain.Detail{}, false
	}
	c.mu.RLock()
	e, ok := c.m[id]
	c.mu.RUnlock()
	if !ok || c.now().After(e.exp) {
		if ok {
			c.mu.Lock()
			delete(c.m, id)
			c.mu.Unlock()
		}
		return domain.Detail{}, false
	}
	return e.detail, true
}

func (c *ReportCache) Set(id uuid.UUID, d domain.Detail) {
	if !c.Enabled() {
		return
	}
	c.mu.Lock()
	c.m[id] = entry{detail: d, exp: c.now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *ReportCache) Invalidate(id uuid.UUID) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.m, id)
	c.mu.Unlock()
}
