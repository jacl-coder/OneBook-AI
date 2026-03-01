package util

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

type requestIDContextKey string

const (
	requestIDHeader          = "X-Request-Id"
	requestIDCtxKey          = requestIDContextKey("request_id")
	defaultRequestIDFallback = ""
)

// WithRequestID propagates an incoming request id or generates one when absent.
// The id is set on both response header and request context.
// A child slog.Logger carrying "request_id" is also stored in the context
// so that downstream code can call util.LoggerFromContext(ctx) to get a
// logger that automatically includes the request id.
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if requestID == "" {
			requestID = NewID()
		}
		w.Header().Set(requestIDHeader, requestID)

		// Store request ID in context.
		ctx := context.WithValue(r.Context(), requestIDCtxKey, requestID)

		// Inject a logger carrying the request_id into context.
		logger := slog.Default().With("request_id", requestID)
		ctx = ContextWithLogger(ctx, logger)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns request id from context.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return defaultRequestIDFallback
	}
	id, _ := ctx.Value(requestIDCtxKey).(string)
	return id
}

// RequestIDFromRequest returns request id from request context.
func RequestIDFromRequest(r *http.Request) string {
	if r == nil {
		return defaultRequestIDFallback
	}
	return RequestIDFromContext(r.Context())
}
