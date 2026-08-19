package httpx

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	goredis "github.com/redis/go-redis/v9"
)

// RateLimitRedis applies per-IP fixed-window counters stored in Redis for multi-replica APIs.
func RateLimitRedis(cfg config.RateLimitConfig, trusted []TrustedNetwork, metrics rateMetrics, rdb *goredis.Client, next http.Handler) http.Handler {
	if !cfg.Enabled() || rdb == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := ClientIP(r, trusted)
		authRoute := strings.HasPrefix(r.URL.Path, "/v1/auth/")
		max := cfg.Burst
		if authRoute && cfg.AuthBurst > 0 {
			max = cfg.AuthBurst
		}
		if max < 1 {
			max = 1
		}
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

func redisAllow(ctx context.Context, rdb *goredis.Client, key string, max int, window time.Duration) bool {
	if window <= 0 {
		window = time.Minute
	}
	n, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return true
	}
	if n == 1 {
		_ = rdb.Expire(ctx, key, window).Err()
	}
	return n <= int64(max)
}
