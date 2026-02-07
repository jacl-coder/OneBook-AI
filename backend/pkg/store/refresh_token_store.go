package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrInvalidRefreshToken indicates token not found or expired.
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	// ErrRefreshTokenReplay indicates refresh token replay/reuse was detected.
	ErrRefreshTokenReplay = errors.New("refresh token replay detected")
)

// RefreshTokenStore persists refresh tokens for rotation + replay detection.
type RefreshTokenStore interface {
	NewToken(userID string, ttl time.Duration) (string, error)
	RotateToken(token string, ttl time.Duration) (userID string, newToken string, err error)
	DeleteToken(token string) error
}

type refreshFamily struct {
	userID      string
	currentHash string
	expiry      time.Time
}

// MemoryRefreshTokenStore keeps refresh token families in memory.
type MemoryRefreshTokenStore struct {
	mu           sync.Mutex
	families     map[string]refreshFamily       // familyID -> family
	tokenFamily  map[string]string              // tokenHash -> familyID
	familyTokens map[string]map[string]struct{} // familyID -> token hashes
	userFamilies map[string]map[string]struct{} // userID -> family IDs
}

// NewMemoryRefreshTokenStore constructs an in-memory refresh token store.
func NewMemoryRefreshTokenStore() *MemoryRefreshTokenStore {
	return &MemoryRefreshTokenStore{
		families:     make(map[string]refreshFamily),
		tokenFamily:  make(map[string]string),
		familyTokens: make(map[string]map[string]struct{}),
		userFamilies: make(map[string]map[string]struct{}),
	}
}

// NewToken issues and stores a new refresh token family.
func (s *MemoryRefreshTokenStore) NewToken(userID string, ttl time.Duration) (string, error) {
	token, err := generateRefreshToken()
	if err != nil {
		return "", err
	}
	familyID, err := generateFamilyID()
	if err != nil {
		return "", err
	}
	tokenHash := refreshTokenHash(token)
	now := time.Now().UTC()

	s.mu.Lock()
	s.families[familyID] = refreshFamily{
		userID:      userID,
		currentHash: tokenHash,
		expiry:      now.Add(ttl),
	}
	s.tokenFamily[tokenHash] = familyID
	s.familyTokens[familyID] = map[string]struct{}{tokenHash: {}}
	if s.userFamilies[userID] == nil {
		s.userFamilies[userID] = make(map[string]struct{})
	}
	s.userFamilies[userID][familyID] = struct{}{}
	s.mu.Unlock()
	return token, nil
}

// RotateToken validates token and issues a new token in same family.
func (s *MemoryRefreshTokenStore) RotateToken(token string, ttl time.Duration) (string, string, error) {
	tokenHash := refreshTokenHash(token)
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	familyID, ok := s.tokenFamily[tokenHash]
	if !ok {
		return "", "", ErrInvalidRefreshToken
	}
	family, ok := s.families[familyID]
	if !ok || now.After(family.expiry) {
		s.revokeFamilyLocked(familyID)
		return "", "", ErrInvalidRefreshToken
	}
	if family.currentHash != tokenHash {
		// Reuse of previously rotated token: revoke whole family.
		s.revokeFamilyLocked(familyID)
		return "", "", ErrRefreshTokenReplay
	}

	newToken, err := generateRefreshToken()
	if err != nil {
		return "", "", err
	}
	newHash := refreshTokenHash(newToken)
	family.currentHash = newHash
	family.expiry = now.Add(ttl)
	s.families[familyID] = family
	s.tokenFamily[newHash] = familyID
	if s.familyTokens[familyID] == nil {
		s.familyTokens[familyID] = make(map[string]struct{})
	}
	s.familyTokens[familyID][newHash] = struct{}{}
	return family.userID, newToken, nil
}

// DeleteToken revokes the entire token family containing this token.
func (s *MemoryRefreshTokenStore) DeleteToken(token string) error {
	tokenHash := refreshTokenHash(token)

	s.mu.Lock()
	familyID, ok := s.tokenFamily[tokenHash]
	if ok {
		s.revokeFamilyLocked(familyID)
	}
	s.mu.Unlock()
	return nil
}

func (s *MemoryRefreshTokenStore) revokeFamilyLocked(familyID string) {
	userID := s.families[familyID].userID
	hashes, ok := s.familyTokens[familyID]
	if ok {
		for h := range hashes {
			delete(s.tokenFamily, h)
		}
	}
	delete(s.familyTokens, familyID)
	delete(s.families, familyID)
	if userID != "" {
		if fams, ok := s.userFamilies[userID]; ok {
			delete(fams, familyID)
			if len(fams) == 0 {
				delete(s.userFamilies, userID)
			}
		}
	}
}

// RevokeUserRefreshTokens revokes all refresh token families for a user.
func (s *MemoryRefreshTokenStore) RevokeUserRefreshTokens(userID string) error {
	s.mu.Lock()
	familyIDs := make([]string, 0, len(s.userFamilies[userID]))
	for familyID := range s.userFamilies[userID] {
		familyIDs = append(familyIDs, familyID)
	}
	for _, familyID := range familyIDs {
		s.revokeFamilyLocked(familyID)
	}
	s.mu.Unlock()
	return nil
}

// RedisRefreshTokenStore stores refresh token families in Redis.
type RedisRefreshTokenStore struct {
	client *redis.Client
}

// NewRedisRefreshTokenStore builds a Redis-backed refresh token store.
func NewRedisRefreshTokenStore(addr, password string) *RedisRefreshTokenStore {
	return &RedisRefreshTokenStore{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
		}),
	}
}

// NewToken issues and stores a new refresh token family.
func (s *RedisRefreshTokenStore) NewToken(userID string, ttl time.Duration) (string, error) {
	token, err := generateRefreshToken()
	if err != nil {
		return "", err
	}
	familyID, err := generateFamilyID()
	if err != nil {
		return "", err
	}
	tokenHash := refreshTokenHash(token)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pipe := s.client.TxPipeline()
	pipe.Set(ctx, refreshTokenFamilyRedisKey(tokenHash), familyID, ttl)
	pipe.HSet(ctx, refreshFamilyRedisKey(familyID), map[string]any{
		"userId":      userID,
		"currentHash": tokenHash,
	})
	pipe.Expire(ctx, refreshFamilyRedisKey(familyID), ttl)
	pipe.SAdd(ctx, refreshFamilyTokensRedisKey(familyID), tokenHash)
	pipe.Expire(ctx, refreshFamilyTokensRedisKey(familyID), ttl)
	pipe.SAdd(ctx, refreshUserFamiliesRedisKey(userID), familyID)
	pipe.Expire(ctx, refreshUserFamiliesRedisKey(userID), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", err
	}
	return token, nil
}

// RotateToken validates token and issues a new token in same family.
func (s *RedisRefreshTokenStore) RotateToken(token string, ttl time.Duration) (string, string, error) {
	tokenHash := refreshTokenHash(token)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}

		familyID, err := s.client.Get(ctx, refreshTokenFamilyRedisKey(tokenHash)).Result()
		if err == redis.Nil {
			return "", "", ErrInvalidRefreshToken
		}
		if err != nil {
			return "", "", err
		}

		familyKey := refreshFamilyRedisKey(familyID)
		var (
			userID       string
			newToken     string
			shouldRevoke bool
		)

		err = s.client.Watch(ctx, func(tx *redis.Tx) error {
			familyData, err := tx.HGetAll(ctx, familyKey).Result()
			if err != nil {
				return err
			}
			if len(familyData) == 0 {
				shouldRevoke = true
				return ErrInvalidRefreshToken
			}

			currentHash := familyData["currentHash"]
			userID = familyData["userId"]
			if currentHash == "" || userID == "" {
				shouldRevoke = true
				return ErrInvalidRefreshToken
			}
			if currentHash != tokenHash {
				shouldRevoke = true
				return ErrRefreshTokenReplay
			}

			newToken, err = generateRefreshToken()
			if err != nil {
				return err
			}
			newHash := refreshTokenHash(newToken)

			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, refreshTokenFamilyRedisKey(newHash), familyID, ttl)
				pipe.HSet(ctx, familyKey, map[string]any{
					"userId":      userID,
					"currentHash": newHash,
				})
				pipe.Expire(ctx, familyKey, ttl)
				pipe.SAdd(ctx, refreshFamilyTokensRedisKey(familyID), newHash)
				pipe.Expire(ctx, refreshFamilyTokensRedisKey(familyID), ttl)
				pipe.SAdd(ctx, refreshUserFamiliesRedisKey(userID), familyID)
				pipe.Expire(ctx, refreshUserFamiliesRedisKey(userID), ttl)
				return nil
			})
			return err
		}, familyKey)

		if err == redis.TxFailedErr {
			continue
		}
		if err != nil {
			if shouldRevoke {
				_ = s.revokeFamily(ctx, familyID, userID)
			}
			if errors.Is(err, ErrRefreshTokenReplay) {
				return "", "", ErrRefreshTokenReplay
			}
			if errors.Is(err, ErrInvalidRefreshToken) {
				return "", "", ErrInvalidRefreshToken
			}
			return "", "", err
		}
		return userID, newToken, nil
	}
}

// DeleteToken revokes the entire token family containing this token.
func (s *RedisRefreshTokenStore) DeleteToken(token string) error {
	tokenHash := refreshTokenHash(token)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	familyID, err := s.client.Get(ctx, refreshTokenFamilyRedisKey(tokenHash)).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	familyData, err := s.client.HGetAll(ctx, refreshFamilyRedisKey(familyID)).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	return s.revokeFamily(ctx, familyID, familyData["userId"])
}

func (s *RedisRefreshTokenStore) revokeFamily(ctx context.Context, familyID, userID string) error {
	if userID == "" {
		familyData, err := s.client.HGetAll(ctx, refreshFamilyRedisKey(familyID)).Result()
		if err != nil && err != redis.Nil {
			return err
		}
		userID = familyData["userId"]
	}
	hashes, err := s.client.SMembers(ctx, refreshFamilyTokensRedisKey(familyID)).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	pipe := s.client.TxPipeline()
	for _, tokenHash := range hashes {
		pipe.Del(ctx, refreshTokenFamilyRedisKey(tokenHash))
	}
	pipe.Del(ctx, refreshFamilyTokensRedisKey(familyID))
	pipe.Del(ctx, refreshFamilyRedisKey(familyID))
	if userID != "" {
		pipe.SRem(ctx, refreshUserFamiliesRedisKey(userID), familyID)
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return err
	}
	return nil
}

// RevokeUserRefreshTokens revokes all refresh token families for a user.
func (s *RedisRefreshTokenStore) RevokeUserRefreshTokens(userID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	familyIDs, err := s.client.SMembers(ctx, refreshUserFamiliesRedisKey(userID)).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	for _, familyID := range familyIDs {
		if err := s.revokeFamily(ctx, familyID, userID); err != nil {
			return err
		}
	}
	if err := s.client.Del(ctx, refreshUserFamiliesRedisKey(userID)).Err(); err != nil && err != redis.Nil {
		return err
	}
	return nil
}

func generateRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func generateFamilyID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func refreshTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func refreshTokenFamilyRedisKey(tokenHash string) string {
	return fmt.Sprintf("refresh:token:%s", tokenHash)
}

func refreshFamilyRedisKey(familyID string) string {
	return fmt.Sprintf("refresh:family:%s", familyID)
}

func refreshFamilyTokensRedisKey(familyID string) string {
	return fmt.Sprintf("refresh:family_tokens:%s", familyID)
}

func refreshUserFamiliesRedisKey(userID string) string {
	return fmt.Sprintf("refresh:user_families:%s", userID)
}
