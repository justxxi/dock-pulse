package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/justxxi/dock-pulse/internal/auth"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

func MiddlewareChain(logger *slog.Logger, authenticator *auth.Authenticator, readOnly bool) func(http.Handler) http.Handler {
	rateLimiter := NewRateLimiter(10, time.Second)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				b := make([]byte, 8)
				_, _ = rand.Read(b)
				reqID = hex.EncodeToString(b)
			}
			w.Header().Set("X-Request-ID", reqID)

			ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
			r = r.WithContext(ctx)

			w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("X-Frame-Options", "DENY")

			if r.URL.Path != "/api/health" && authenticator.IsRequired() {
				if !authenticator.AuthenticateRequest(r) {
					writeJSONError(w, http.StatusUnauthorized, "unauthorized", "Authentication token is invalid or missing", reqID)
					return
				}
			}

			if readOnly && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete) {
				writeJSONError(w, http.StatusForbidden, "read_only", "Mutative actions are disabled in read-only mode", reqID)
				return
			}

			if r.Method == http.MethodPost {
				ip := r.RemoteAddr
				if !rateLimiter.Allow(ip) {
					writeJSONError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "Rate limit exceeded for mutative requests", reqID)
					return
				}
			}

			wrapped := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}
			start := time.Now()

			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered in HTTP handler", "panic", rec, "request_id", reqID, "path", r.URL.Path)
					writeJSONError(wrapped, http.StatusInternalServerError, "internal_error", "Internal server error occurred", reqID)
				}
			}()

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
				"request_id", reqID,
				"bytes", wrapped.written,
			)
		})
	}
}

type RateLimiter struct {
	mu     sync.Mutex
	limits map[string][]time.Time
	rate   int
	window time.Duration
}

func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limits: make(map[string][]time.Time),
		rate:   rate,
		window: window,
	}
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	timestamps := rl.limits[ip]
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.rate {
		rl.limits[ip] = valid
		return false
	}

	valid = append(valid, now)
	rl.limits[ip] = valid
	return true
}

func writeJSONError(w http.ResponseWriter, status int, code, message, reqID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":{"code":"%s","message":"%s","request_id":"%s"}}`, code, message, reqID)
}
