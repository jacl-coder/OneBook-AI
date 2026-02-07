package store

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenRevoker tracks revoked tokens until expiry.
type TokenRevoker interface {
	Revoke(token string, ttl time.Duration) error
	IsRevoked(token string) (bool, error)
}

// UserTokenRevoker is an optional capability that revokes all tokens for a
// given user at/after a timestamp.
type UserTokenRevoker interface {
	RevokeUser(userID string, since time.Time) error
	RevokedAfter(userID string) (time.Time, error)
}

// MemoryTokenRevoker keeps revoked tokens in-memory (single instance only).
type MemoryTokenRevoker struct {
	mu               sync.Mutex
	tokens           map[string]time.Time
	userRevokedAfter map[string]time.Time
}

// NewMemoryTokenRevoker builds an in-memory revoker.
func NewMemoryTokenRevoker() *MemoryTokenRevoker {
	return &MemoryTokenRevoker{
		tokens:           make(map[string]time.Time),
		userRevokedAfter: make(map[string]time.Time),
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

// RevokeUser revokes all tokens issued to the user before/at the cutoff time.
func (r *MemoryTokenRevoker) RevokeUser(userID string, since time.Time) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	if since.IsZero() {
		since = time.Now().UTC()
	}
	since = since.UTC()
	r.mu.Lock()
	prev := r.userRevokedAfter[userID]
	if prev.IsZero() || since.After(prev) {
		r.userRevokedAfter[userID] = since
	}
	r.mu.Unlock()
	return nil
}

// RevokedAfter returns the user revocation cutoff timestamp.
func (r *MemoryTokenRevoker) RevokedAfter(userID string) (time.Time, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return time.Time{}, nil
	}
	r.mu.Lock()
	cutoff := r.userRevokedAfter[userID]
	r.mu.Unlock()
	return cutoff, nil
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

// RevokeUser revokes all tokens issued to the user before/at the cutoff time.
func (r *RedisTokenRevoker) RevokeUser(userID string, since time.Time) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	if since.IsZero() {
		since = time.Now().UTC()
	}
	since = since.UTC()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := userRevocationCutoffKey(userID)
	prevRaw, err := r.client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	prev, parseErr := time.Parse(time.RFC3339Nano, prevRaw)
	if parseErr == nil && !since.After(prev) {
		return nil
	}
	return r.client.Set(ctx, key, since.Format(time.RFC3339Nano), 0).Err()
}

// RevokedAfter returns the user revocation cutoff timestamp.
func (r *RedisTokenRevoker) RevokedAfter(userID string) (time.Time, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return time.Time{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := r.client.Get(ctx, userRevocationCutoffKey(userID)).Result()
	if err == redis.Nil {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	tm, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, nil
	}
	return tm.UTC(), nil
}

func revocationKey(token string) string {
	return "revoked:" + token
}

func userRevocationCutoffKey(userID string) string {
	return "revoked-user-since:" + userID
}
