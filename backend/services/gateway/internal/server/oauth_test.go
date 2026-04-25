package server

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestOAuthStateStoreConsumeIsSingleUse(t *testing.T) {
	redis := miniredis.RunT(t)
	store := newOAuthStateStore(redis.Addr(), "", "test:oauth")
	ctx := context.Background()
	payload := oauthState{
		Provider:     oauthProviderGoogle,
		Nonce:        "nonce",
		CodeVerifier: "verifier",
		ReturnTo:     "http://localhost:5173/chat",
		CreatedAt:    time.Now().UTC(),
	}
	if err := store.Save(ctx, "state-id", payload); err != nil {
		t.Fatalf("save state: %v", err)
	}
	got, err := store.Consume(ctx, "state-id")
	if err != nil {
		t.Fatalf("consume state: %v", err)
	}
	if got.Provider != payload.Provider || got.Nonce != payload.Nonce || got.CodeVerifier != payload.CodeVerifier {
		t.Fatalf("unexpected state payload: %#v", got)
	}
	if _, err := store.Consume(ctx, "state-id"); err != errOAuthStateInvalid {
		t.Fatalf("second consume err = %v, want %v", err, errOAuthStateInvalid)
	}
}

func TestOAuthReturnToUsesConfiguredAppBaseAndRejectsExternalURL(t *testing.T) {
	srv := &Server{oauth: oauthConfig{AppBaseURL: "http://localhost:5173"}}
	req := httptest.NewRequest("GET", "/api/auth/oauth/google/start?returnTo=https://evil.example/chat", nil)
	if got := srv.oauthReturnTo(req, "/chat"); got != "http://localhost:5173/chat" {
		t.Fatalf("external returnTo fallback = %q", got)
	}

	req = httptest.NewRequest("GET", "/api/auth/oauth/google/start?returnTo=/library", nil)
	if got := srv.oauthReturnTo(req, "/chat"); got != "http://localhost:5173/library" {
		t.Fatalf("relative returnTo = %q", got)
	}
}
