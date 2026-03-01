package util

import (
	"net/http"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status       int
	bytesWritten int64
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytesWritten += int64(n)
	return n, err
}

// slowRequestThreshold is the duration above which a request is flagged.
const slowRequestThreshold = 5 * time.Second

// skipPaths are health-check endpoints excluded from request logging.
var skipPaths = map[string]bool{
	"/healthz": true,
	"/readyz":  true,
}

// WithRequestLog emits a structured log for each HTTP request.
// Log level is chosen by response status: 5xx → Error, 4xx → Warn, else Info.
// It includes request_id so logs can be correlated across services.
func WithRequestLog(service string, next http.Handler) http.Handler {
	service = strings.TrimSpace(service)
	if service == "" {
		service = "unknown"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip health-check endpoints.
		if skipPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		duration := time.Since(start)

		log := LoggerFromContext(r.Context())
		attrs := []any{
			"service", service,
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"bytes_written", rec.bytesWritten,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
			"content_length", r.ContentLength,
		}

		// Log level based on status code.
		switch {
		case status >= 500:
			log.Error("http_request", attrs...)
		case status >= 400:
			log.Warn("http_request", attrs...)
		default:
			log.Info("http_request", attrs...)
		}

		// Flag slow requests.
		if duration >= slowRequestThreshold {
			log.Warn("slow_request",
				"service", service,
				"method", r.Method,
				"path", r.URL.Path,
				"duration_ms", duration.Milliseconds(),
			)
		}
	})
}
