package backend

import (
	"context"
	"github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"time"
)

func requestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = generateID("req")
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
