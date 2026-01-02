package store

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenRevoker tracks revoked tokens until expiry.
type TokenRevoker interface {
	Revoke(token string, ttl time.Duration) error
	IsRevoked(token string) (bool, error)
}

// MemoryTokenRevoker keeps revoked tokens in-memory (single instance only).
type MemoryTokenRevoker struct {
	mu     sync.Mutex
	tokens map[string]time.Time
}

// NewMemoryTokenRevoker builds an in-memory revoker.
func NewMemoryTokenRevoker() *MemoryTokenRevoker {
	return &MemoryTokenRevoker{
		tokens: make(map[string]time.Time),
	}
}

// Revoke marks a token as revoked until its expiry.
func (r *MemoryTokenRevoker) Revoke(token string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	r.mu.Lock()
	r.tokens[token] = time.Now().Add(ttl)
	r.mu.Unlock()
	return nil
}

// IsRevoked checks if the token is revoked.
func (r *MemoryTokenRevoker) IsRevoked(token string) (bool, error) {
	r.mu.Lock()
	expiry, ok := r.tokens[token]
	if !ok {
		r.mu.Unlock()
		return false, nil
	}
	if time.Now().After(expiry) {
		delete(r.tokens, token)
		r.mu.Unlock()
		return false, nil
	}
	r.mu.Unlock()
	return true, nil
}

// RedisTokenRevoker stores revoked tokens in Redis with TTL.
type RedisTokenRevoker struct {
	client *redis.Client
}

// NewRedisTokenRevoker builds a Redis-backed revoker.
func NewRedisTokenRevoker(addr, password string) *RedisTokenRevoker {
	return &RedisTokenRevoker{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
		}),
	}
}

// Revoke marks a token as revoked until expiry.
func (r *RedisTokenRevoker) Revoke(token string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return r.client.Set(ctx, revocationKey(token), "1", ttl).Err()
}

// IsRevoked checks if the token is revoked.
func (r *RedisTokenRevoker) IsRevoked(token string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res, err := r.client.Exists(ctx, revocationKey(token)).Result()
	if err != nil {
		return false, err
	}
	return res > 0, nil
}

func revocationKey(token string) string {
	return "revoked:" + token
}
