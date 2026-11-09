// Package middleware contains HTTP middleware.
// Blueprint §9: request IDs for tracing on every request.
package middleware

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// RequestID assigns a UUID-ish request ID, logs duration, and recovers panics.
func RequestID(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = newID()
		}
		w.Header().Set("X-Request-Id", id)
		start := time.Now()

		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic recovered", "id", id, "recover", rec)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)

		if logger != nil {
			logger.Info("request", "id", id, "method", r.Method, "path", r.URL.Path,
				"status", "ok", "duration_ms", time.Since(start).Milliseconds())
		}
	})
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b)
}
