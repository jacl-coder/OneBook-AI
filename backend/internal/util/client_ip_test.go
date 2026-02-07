package util

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	trusted, err := NewTrustedProxies([]string{"10.0.0.0/8", "192.168.1.10"})
	if err != nil {
		t.Fatalf("new trusted proxies: %v", err)
	}

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xrip       string
		trusted    *TrustedProxies
		want       string
	}{
		{
			name:       "no trusted proxies ignores forwarded headers",
			remoteAddr: "198.51.100.10:1234",
			xff:        "203.0.113.5",
			xrip:       "203.0.113.6",
			want:       "198.51.100.10",
		},
		{
			name:       "trusted remote accepts x-forwarded-for",
			remoteAddr: "10.0.0.20:1234",
			xff:        "203.0.113.5",
			trusted:    trusted,
			want:       "203.0.113.5",
		},
		{
			name:       "trusted chain picks first untrusted from right",
			remoteAddr: "10.0.0.20:1234",
			xff:        "203.0.113.5, 10.0.0.10",
			trusted:    trusted,
			want:       "203.0.113.5",
		},
		{
			name:       "falls back to x-real-ip when xff unusable",
			remoteAddr: "10.0.0.20:1234",
			xff:        "invalid",
			xrip:       "203.0.113.7",
			trusted:    trusted,
			want:       "203.0.113.7",
		},
		{
			name:       "all proxies trusted returns leftmost hop",
			remoteAddr: "10.0.0.20:1234",
			xff:        "10.0.0.5, 10.0.0.10",
			trusted:    trusted,
			want:       "10.0.0.5",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.xrip != "" {
				req.Header.Set("X-Real-IP", tc.xrip)
			}
			if got := ClientIP(req, tc.trusted); got != tc.want {
				t.Fatalf("client ip = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewTrustedProxies(t *testing.T) {
	if _, err := NewTrustedProxies([]string{"10.0.0.0/8", "192.168.1.1"}); err != nil {
		t.Fatalf("expected valid entries, got err: %v", err)
	}
	if _, err := NewTrustedProxies([]string{"bad-cidr"}); err == nil {
		t.Fatalf("expected parse error for invalid entry")
	}
}
