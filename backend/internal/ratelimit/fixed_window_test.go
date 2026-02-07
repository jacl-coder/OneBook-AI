package ratelimit

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestFixedWindowLimiterRedis(t *testing.T) {
	redis := miniredis.RunT(t)
	limiter, err := NewRedisFixedWindowLimiter(redis.Addr(), "", "test:ratelimit", 2, time.Second)
	if err != nil {
		t.Fatalf("new redis limiter: %v", err)
	}
	if !limiter.Allow("ip-1") {
		t.Fatalf("first request should pass")
	}
	if !limiter.Allow("ip-1") {
		t.Fatalf("second request should pass")
	}
	if limiter.Allow("ip-1") {
		t.Fatalf("third request should be blocked")
	}
}

func TestFixedWindowLimiterRedisFailClosed(t *testing.T) {
	redis := miniredis.RunT(t)
	limiter, err := NewRedisFixedWindowLimiter(redis.Addr(), "", "test:ratelimit", 1, time.Second)
	if err != nil {
		t.Fatalf("new redis limiter: %v", err)
	}
	redis.Close()
	if limiter.Allow("ip-1") {
		t.Fatalf("limiter should fail closed on redis errors")
	}
}

func TestFixedWindowLimiterRequiresRedisAddr(t *testing.T) {
	limiter, err := NewRedisFixedWindowLimiter("", "", "test:ratelimit", 1, time.Second)
	if err == nil || limiter != nil {
		t.Fatalf("expected constructor error for empty redis addr")
	}
}
