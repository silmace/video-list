package app

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

func requestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

func sanitizeRequestID(id string) string {
	// Remove non-printable and unsafe characters for logging
	id = strings.TrimSpace(id)
	// Allow only alphanumeric, hyphens, underscores, and dots
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_.]`)
	id = re.ReplaceAllString(id, "")
	// Limit length to prevent log flooding
	if len(id) > 64 {
		id = id[:64]
	}
	// If nothing left or too short, generate a new one
	if len(id) == 0 {
		id = generateID("req")
	}
	return id
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateID("req")
		} else {
			requestID = sanitizeRequestID(requestID)
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		durationMs := time.Since(start).Milliseconds()
		LoggerWith(logrus.Fields{
			"request_id":  requestIDFromContext(r.Context()),
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      recorder.status,
			"duration_ms": durationMs,
			"remote_addr": r.RemoteAddr,
		}).Info("request completed")
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
