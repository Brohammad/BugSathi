package httpx

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	goredis "github.com/redis/go-redis/v9"
)

// RateLimitRedis applies per-IP fixed-window counters stored in Redis for multi-replica APIs.
// Window capacity matches the in-memory limiter intent: at least Burst, or RPS×window when larger.
// Redis errors fail closed (429) so an outage cannot silently disable abuse protection.
func RateLimitRedis(cfg config.RateLimitConfig, trusted []TrustedNetwork, metrics rateMetrics, rdb *goredis.Client, next http.Handler) http.Handler {
	if !cfg.Enabled() || rdb == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := ClientIP(r, trusted)
		authRoute := strings.HasPrefix(r.URL.Path, "/v1/auth/")
		max := redisWindowCapacity(cfg, authRoute)
		route := r.Pattern
		if route == "" {
			route = r.URL.Path
		}
		redisKey := "bugsathi:rl:" + route + ":" + key
		if !redisAllow(r.Context(), rdb, redisKey, max, cfg.Window) {
			if metrics != nil {
				metrics.IncRateLimited(route)
			}
			retry := int(cfg.Window.Seconds())
			if retry < 1 {
				retry = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func redisWindowCapacity(cfg config.RateLimitConfig, auth bool) int {
	rps := cfg.RPS
	burst := cfg.Burst
	if auth && cfg.AuthRPS > 0 {
		rps = cfg.AuthRPS
		burst = cfg.AuthBurst
	}
	window := cfg.Window
	if window <= 0 {
		window = time.Minute
	}
	fromRPS := int(math.Ceil(rps * window.Seconds()))
	max := burst
	if fromRPS > max {
		max = fromRPS
	}
	if max < 1 {
		max = 1
	}
	return max
}

func redisAllow(ctx context.Context, rdb *goredis.Client, key string, max int, window time.Duration) bool {
	if window <= 0 {
		window = time.Minute
	}
	n, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return false
	}
	if n == 1 {
		_ = rdb.Expire(ctx, key, window).Err()
	}
	return n <= int64(max)
}

// RedisWindowCapacityForTest exposes redisWindowCapacity for unit tests.
func RedisWindowCapacityForTest(cfg config.RateLimitConfig, auth bool) int {
	return redisWindowCapacity(cfg, auth)
}
