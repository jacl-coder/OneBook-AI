package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var fixedWindowScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return count
`)

// FixedWindowLimiter limits requests per key in a fixed time window.
// It uses Redis-backed distributed mode only.
type FixedWindowLimiter struct {
	limit  int
	window time.Duration

	redisClient *redis.Client
	redisPrefix string
}

// NewRedisFixedWindowLimiter creates a Redis-backed distributed limiter.
func NewRedisFixedWindowLimiter(addr, password, prefix string, limit int, window time.Duration) (*FixedWindowLimiter, error) {
	if limit <= 0 || window <= 0 {
		return nil, errors.New("rate limiter requires positive limit and window")
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, errors.New("rate limiter redis addr is required")
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "onebook:ratelimit"
	}
	return &FixedWindowLimiter{
		limit:  limit,
		window: window,
		redisClient: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
		}),
		redisPrefix: prefix,
	}, nil
}

// Allow returns true when the key is within quota.
// On Redis failures, it fails closed and returns false.
func (l *FixedWindowLimiter) Allow(key string) bool {
	if l == nil {
		return false
	}
	key = strings.TrimSpace(key)
	if key == "" {
		key = "unknown"
	}
	return l.allowRedis(key)
}

func (l *FixedWindowLimiter) allowRedis(key string) bool {
	windowMs := l.window.Milliseconds()
	if windowMs <= 0 {
		return true
	}
	windowSlot := time.Now().UTC().UnixMilli() / windowMs
	redisKey := fmt.Sprintf("%s:%s:%d", l.redisPrefix, key, windowSlot)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := fixedWindowScript.Run(ctx, l.redisClient, []string{redisKey}, windowMs).Int64()
	if err != nil {
		return false
	}
	return res <= int64(l.limit)
}
