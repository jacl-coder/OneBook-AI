package store

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisRefreshTokenStoreRotateAndDelete(t *testing.T) {
	redis := miniredis.RunT(t)
	s := NewRedisRefreshTokenStore(redis.Addr(), "")

	token, err := s.NewToken("user-1", time.Minute)
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	userID, nextToken, err := s.RotateToken(token, time.Minute)
	if err != nil {
		t.Fatalf("rotate token: %v", err)
	}
	if userID != "user-1" {
		t.Fatalf("unexpected user id: %q", userID)
	}
	if nextToken == "" || nextToken == token {
		t.Fatalf("expected rotated token")
	}

	if err := s.DeleteToken(nextToken); err != nil {
		t.Fatalf("delete token: %v", err)
	}
	if _, _, err := s.RotateToken(nextToken, time.Minute); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected invalid token after delete, got: %v", err)
	}
}

func TestRedisRefreshTokenStoreDetectsReplay(t *testing.T) {
	redis := miniredis.RunT(t)
	s := NewRedisRefreshTokenStore(redis.Addr(), "")

	token, err := s.NewToken("user-2", time.Minute)
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	_, nextToken, err := s.RotateToken(token, time.Minute)
	if err != nil {
		t.Fatalf("first rotate: %v", err)
	}

	if _, _, err := s.RotateToken(token, time.Minute); !errors.Is(err, ErrRefreshTokenReplay) {
		t.Fatalf("expected replay detection, got: %v", err)
	}
	if _, _, err := s.RotateToken(nextToken, time.Minute); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected family revoked after replay, got: %v", err)
	}
}

func TestRedisRefreshTokenStoreConcurrentRotateRevokesFamilyOnReplay(t *testing.T) {
	redis := miniredis.RunT(t)
	s := NewRedisRefreshTokenStore(redis.Addr(), "")

	token, err := s.NewToken("user-3", time.Minute)
	if err != nil {
		t.Fatalf("new token: %v", err)
	}

	const workers = 2
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)

	errs := make(chan error, workers)
	newTokens := make(chan string, workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			_, nextToken, rotateErr := s.RotateToken(token, time.Minute)
			if rotateErr == nil {
				newTokens <- nextToken
			}
			errs <- rotateErr
		}()
	}

	close(start)
	wg.Wait()
	close(errs)
	close(newTokens)

	successes := 0
	replays := 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrRefreshTokenReplay):
			replays++
		default:
			t.Fatalf("unexpected rotate error: %v", err)
		}
	}
	if successes != 1 || replays != 1 {
		t.Fatalf("expected one success and one replay, got successes=%d replays=%d", successes, replays)
	}

	for issued := range newTokens {
		if _, _, err := s.RotateToken(issued, time.Minute); !errors.Is(err, ErrInvalidRefreshToken) {
			t.Fatalf("expected family revoked after replay race, got: %v", err)
		}
	}
}
