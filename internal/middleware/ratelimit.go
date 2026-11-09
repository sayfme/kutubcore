package middleware

import (
	"sync"
	"time"

	"net/http"
)

// RateLimiter is a tiny fixed-window limiter for auth endpoints
// (Blueprint §9/§14: rate limits on the API).
type RateLimiter struct {
	mu        sync.Mutex
	hits      map[string]int
	window    time.Duration
	maxPerWin int
	resetAt   time.Time
}

func NewRateLimiter(maxPerMin int, window time.Duration) *RateLimiter {
	if maxPerMin <= 0 {
		maxPerMin = 30
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{hits: map[string]int{}, window: window, maxPerWin: maxPerMin, resetAt: time.Now().Add(window)}
}

// Allow reports whether key may proceed. Key by IP + path.
func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if time.Now().After(l.resetAt) {
		l.hits = map[string]int{}
		l.resetAt = time.Now().Add(l.window)
	}
	l.hits[key]++
	return l.hits[key] <= l.maxPerWin
}

// RateLimitAuth wraps auth routes (login/register) to slow brute force.
func RateLimitAuth(l *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr + ":" + r.URL.Path
			if !l.Allow(key) {
				writeError(w, http.StatusTooManyRequests, "RATE_LIMIT", "Too many requests. Try again later.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
