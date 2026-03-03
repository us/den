package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides per-key rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
	stopCh   chan struct{}
}

// NewRateLimiter creates a new rate limiter with a cleanup goroutine.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
		stopCh:   make(chan struct{}),
	}

	// Cleanup old entries periodically
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.mu.Lock()
				rl.limiters = make(map[string]*rate.Limiter)
				rl.mu.Unlock()
			case <-rl.stopCh:
				return
			}
		}
	}()

	return rl
}

// Close stops the cleanup goroutine.
func (rl *RateLimiter) Close() {
	close(rl.stopCh)
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	l, exists := rl.limiters[key]
	if !exists {
		l = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[key] = l
	}
	return l
}

// Middleware returns a rate limiting middleware.
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				// Strip port from RemoteAddr to rate limit by IP, not IP:port
				host, _, err := net.SplitHostPort(r.RemoteAddr)
				if err != nil {
					key = r.RemoteAddr
				} else {
					key = host
				}
			}

			limiter := rl.getLimiter(key)
			if !limiter.Allow() {
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
