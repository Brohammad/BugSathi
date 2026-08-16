package kafka

import (
	"math/rand"
	"time"
)

// Backoff returns exponential delay capped at max, with ±25% jitter.
func Backoff(attempt int, base, max time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if base <= 0 {
		base = time.Second
	}
	if max <= 0 {
		max = 30 * time.Second
	}
	d := base
	for i := 1; i < attempt; i++ {
		if d >= max {
			return jitter(max)
		}
		d *= 2
	}
	if d > max {
		d = max
	}
	return jitter(d)
}

func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	// ±25%
	frac := 0.75 + rand.Float64()*0.5
	return time.Duration(float64(d) * frac)
}
