package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"onebookai/pkg/domain"
	"onebookai/services/gateway/internal/authclient"
)

func TestLoginRateLimit(t *testing.T) {
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/login" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token": "t",
			"user": domain.User{
				ID:     "u-1",
				Email:  "u@example.com",
				Role:   domain.RoleUser,
				Status: domain.StatusActive,
			},
		})
	}))
	defer authSrv.Close()
	redis := miniredis.RunT(t)

	gw, err := New(Config{
		Auth:                       authclient.NewClient(authSrv.URL),
		RedisAddr:                  redis.Addr(),
		SignupRateLimitPerMinute:   10,
		LoginRateLimitPerMinute:    1,
		RefreshRateLimitPerMinute:  10,
		PasswordRateLimitPerMinute: 10,
	})
	if err != nil {
		t.Fatalf("new gateway server: %v", err)
	}
	gwSrv := httptest.NewServer(gw.Router())
	defer gwSrv.Close()

	body := []byte(`{"email":"u@example.com","password":"pass"}`)
	resp1, err := http.Post(gwSrv.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("first login request failed: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first request expected 200, got %d", resp1.StatusCode)
	}

	resp2, err := http.Post(gwSrv.URL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("second login request failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second request expected 429, got %d", resp2.StatusCode)
	}
}

func TestGatewayServerRequiresRedisRateLimiter(t *testing.T) {
	_, err := New(Config{
		SignupRateLimitPerMinute:   1,
		LoginRateLimitPerMinute:    1,
		RefreshRateLimitPerMinute:  1,
		PasswordRateLimitPerMinute: 1,
	})
	if err == nil {
		t.Fatalf("expected redis-backed limiter initialization to fail without redis addr")
	}
}
