package store

import (
	"errors"
	"testing"
	"time"
)

func TestMemoryRefreshTokenStoreRotateAndDelete(t *testing.T) {
	s := NewMemoryRefreshTokenStore()

	token, err := s.NewToken("user-1", time.Minute)
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	if token == "" {
		t.Fatalf("expected token")
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

func TestMemoryRefreshTokenStoreDetectsReplay(t *testing.T) {
	s := NewMemoryRefreshTokenStore()

	token, err := s.NewToken("user-2", time.Minute)
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	_, nextToken, err := s.RotateToken(token, time.Minute)
	if err != nil {
		t.Fatalf("first rotate: %v", err)
	}

	// Reusing old token should be detected as replay and revoke family.
	if _, _, err := s.RotateToken(token, time.Minute); !errors.Is(err, ErrRefreshTokenReplay) {
		t.Fatalf("expected replay detection, got: %v", err)
	}
	if _, _, err := s.RotateToken(nextToken, time.Minute); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected family revoked after replay, got: %v", err)
	}
}

func TestMemoryRefreshTokenStoreRevokeUserRefreshTokens(t *testing.T) {
	s := NewMemoryRefreshTokenStore()

	t1, err := s.NewToken("user-3", time.Minute)
	if err != nil {
		t.Fatalf("new token 1: %v", err)
	}
	t2, err := s.NewToken("user-3", time.Minute)
	if err != nil {
		t.Fatalf("new token 2: %v", err)
	}

	if err := s.RevokeUserRefreshTokens("user-3"); err != nil {
		t.Fatalf("revoke user tokens: %v", err)
	}
	if _, _, err := s.RotateToken(t1, time.Minute); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected token 1 invalid after user revoke, got: %v", err)
	}
	if _, _, err := s.RotateToken(t2, time.Minute); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected token 2 invalid after user revoke, got: %v", err)
	}
}
