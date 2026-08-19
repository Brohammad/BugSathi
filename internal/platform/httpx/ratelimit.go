package httpx

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"golang.org/x/time/rate"
)

type rateMetrics interface {
	IncRateLimited(route string)
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit applies per-IP token buckets. Auth routes use the stricter auth limiter.
// Forwarding headers are used for the client IP only when the connection comes from
// a trusted proxy (see TRUSTED_PROXIES).
func RateLimit(cfg config.RateLimitConfig, trusted []TrustedNetwork, metrics rateMetrics, next http.Handler) http.Handler {
	if !cfg.Enabled() {
		return next
	}
	var mu sync.Mutex
	entries := map[string]*limiterEntry{}
	authEntries := map[string]*limiterEntry{}

	cleanup := func() {
		cutoff := time.Now().Add(-10 * time.Minute)
		for k, e := range entries {
			if e.lastSeen.Before(cutoff) {
				delete(entries, k)
			}
		}
		for k, e := range authEntries {
			if e.lastSeen.Before(cutoff) {
				delete(authEntries, k)
			}
		}
	}

	get := func(key string, auth bool) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		if len(entries) > 10000 {
			cleanup()
		}
		m := entries
		rps := cfg.RPS
		burst := cfg.Burst
		if auth && cfg.AuthRPS > 0 {
			m = authEntries
			rps = cfg.AuthRPS
			burst = cfg.AuthBurst
		}
		e, ok := m[key]
		if !ok {
			e = &limiterEntry{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
			m[key] = e
		}
		e.lastSeen = time.Now()
		return e.limiter
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := ClientIP(r, trusted)
		authRoute := strings.HasPrefix(r.URL.Path, "/v1/auth/")
		lim := get(key, authRoute)
		if !lim.Allow() {
			if metrics != nil {
				route := r.Pattern
				if route == "" {
					route = r.URL.Path
				}
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
