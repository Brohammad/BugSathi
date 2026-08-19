package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Brohammad/BugSathi/internal/reports/domain"
	goredis "github.com/redis/go-redis/v9"
	"github.com/google/uuid"
)

const detailKeyPrefix = "bugsathi:report:detail:"

// RedisReportCache shares report detail aggregates across API replicas.
type RedisReportCache struct {
	rdb *goredis.Client
	ttl time.Duration
}

func NewRedisReportCache(rdb *goredis.Client, ttl time.Duration) *RedisReportCache {
	return &RedisReportCache{rdb: rdb, ttl: ttl}
}

func (c *RedisReportCache) Enabled() bool {
	return c != nil && c.rdb != nil && c.ttl > 0
}

func (c *RedisReportCache) Get(id uuid.UUID) (domain.Detail, bool) {
	if !c.Enabled() {
		return domain.Detail{}, false
	}
	raw, err := c.rdb.Get(context.Background(), detailKeyPrefix+id.String()).Bytes()
	if err != nil {
		return domain.Detail{}, false
	}
	var d domain.Detail
	if err := json.Unmarshal(raw, &d); err != nil {
		return domain.Detail{}, false
	}
	return d, true
}

func (c *RedisReportCache) Set(id uuid.UUID, d domain.Detail) {
	if !c.Enabled() {
		return
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return
	}
	_ = c.rdb.Set(context.Background(), detailKeyPrefix+id.String(), raw, c.ttl).Err()
}

func (c *RedisReportCache) Invalidate(id uuid.UUID) {
	if !c.Enabled() {
		return
	}
	_ = c.rdb.Del(context.Background(), detailKeyPrefix+id.String()).Err()
}
