package util

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// WithRequestLog emits a structured log for each HTTP request.
// It includes request_id so logs can be correlated across services.
func WithRequestLog(service string, next http.Handler) http.Handler {
	service = strings.TrimSpace(service)
	if service == "" {
		service = "unknown"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		slog.Info(
			"http_request",
			"service", service,
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", RequestIDFromRequest(r),
		)
	})
}
