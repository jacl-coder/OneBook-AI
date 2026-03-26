package util

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithRequestLogDoesNotDuplicateContextAttrs(t *testing.T) {
	var buf bytes.Buffer

	originalDefault := slog.Default()
	logger := slog.New(slog.NewJSONHandler(&buf, nil)).With("service", "book")
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(originalDefault)
	})

	handler := WithRequestID(WithRequestLog("book", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/books", nil)
	req.Header.Set("X-Request-Id", "req-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logLine := strings.TrimSpace(buf.String())
	if logLine == "" {
		t.Fatal("expected log output")
	}
	if got := strings.Count(logLine, `"service"`); got != 1 {
		t.Fatalf("expected one service field in log line, got %d: %s", got, logLine)
	}
	if got := strings.Count(logLine, `"request_id"`); got != 1 {
		t.Fatalf("expected one request_id field in log line, got %d: %s", got, logLine)
	}
}
