package util

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithRequestIDPropagatesIncomingHeader(t *testing.T) {
	const incoming = "req-incoming-123"
	handler := WithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := RequestIDFromRequest(r); got != incoming {
			t.Fatalf("unexpected request id in context: got %q want %q", got, incoming)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", incoming)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got != incoming {
		t.Fatalf("unexpected response request id: got %q want %q", got, incoming)
	}
}

func TestWithRequestIDGeneratesWhenMissing(t *testing.T) {
	handler := WithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := RequestIDFromRequest(r); got == "" {
			t.Fatal("expected generated request id in context")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got == "" {
		t.Fatal("expected generated request id header")
	}
}
