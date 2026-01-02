package store

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"onebookai/internal/util"
)

// RedisSessionStore keeps sessions in Redis with TTL.
type RedisSessionStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisSessionStore builds a Redis-backed session store.
func NewRedisSessionStore(addr, password string, ttl time.Duration) *RedisSessionStore {
	return &RedisSessionStore{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
		}),
		ttl: ttl,
	}
}

// NewSession writes a token -> userID mapping with TTL.
func (s *RedisSessionStore) NewSession(userID string) (string, error) {
	token := util.NewID()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.client.Set(ctx, token, userID, s.ttl).Err(); err != nil {
		return "", err
	}
	return token, nil
}

// GetUserIDByToken resolves token to user ID.
func (s *RedisSessionStore) GetUserIDByToken(token string) (string, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	val, err := s.client.Get(ctx, token).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

// DeleteSession removes a token mapping.
func (s *RedisSessionStore) DeleteSession(token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.client.Del(ctx, token).Err(); err != nil && err != redis.Nil {
		return err
	}
	return nil
}
