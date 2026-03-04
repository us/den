package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// maxLimiterEntries caps the number of tracked rate limiter entries
	// to prevent memory exhaustion from large IP address spaces (IPv6).
	maxLimiterEntries = 100_000
)

// limiterEntry wraps a rate.Limiter with its last access time for TTL-based cleanup.
type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// RateLimiter provides per-key rate limiting.
type RateLimiter struct {
	mu        sync.Mutex
	limiters  map[string]*limiterEntry
	rate      rate.Limit
	burst     int
	stopCh    chan struct{}
	closeOnce sync.Once
}

// NewRateLimiter creates a new rate limiter with a cleanup goroutine.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*limiterEntry),
		rate:     rate.Limit(rps),
		burst:    burst,
		stopCh:   make(chan struct{}),
	}

	// Cleanup stale entries periodically (TTL-based, not full reset)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.mu.Lock()
				cutoff := time.Now().Add(-10 * time.Minute)
				for key, entry := range rl.limiters {
					if entry.lastAccess.Before(cutoff) {
						delete(rl.limiters, key)
					}
				}
				rl.mu.Unlock()
			case <-rl.stopCh:
				return
			}
		}
	}()

	return rl
}

// Close stops the cleanup goroutine. Safe to call multiple times.
func (rl *RateLimiter) Close() {
	rl.closeOnce.Do(func() { close(rl.stopCh) })
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.limiters[key]
	if !exists {
		// Cap the number of tracked entries to prevent memory exhaustion
		if len(rl.limiters) >= maxLimiterEntries {
			// Return a restrictive limiter without storing it
			return rate.NewLimiter(rate.Limit(0.1), 1)
		}
		entry = &limiterEntry{
			limiter: rate.NewLimiter(rl.rate, rl.burst),
		}
		rl.limiters[key] = entry
	}
	entry.lastAccess = time.Now()
	return entry.limiter
}

// Middleware returns a rate limiting middleware.
// Always rate limits by IP address to prevent key manipulation attacks.
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Always use IP as rate limit key to prevent attackers from
			// sending arbitrary X-API-Key values to get separate buckets.
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				host = r.RemoteAddr
			}
			key := "ip:" + host

			limiter := rl.getLimiter(key)
			if !limiter.Allow() {
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
