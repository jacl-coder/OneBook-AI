package server

import (
	"context"
	"encoding/base64"
	"math/big"
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

func TestGoogleJWKPublicKeyFromModulusExponent(t *testing.T) {
	modulus := new(big.Int).SetUint64(65537 * 65539)
	key := googleJWK{
		KID: "test-key",
		KTY: "RSA",
		N:   base64.RawURLEncoding.EncodeToString(modulus.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}),
	}
	pub, err := key.publicKey()
	if err != nil {
		t.Fatalf("publicKey() error = %v", err)
	}
	if pub.E != 65537 {
		t.Fatalf("public exponent = %d, want 65537", pub.E)
	}
	if pub.N.Cmp(modulus) != 0 {
		t.Fatalf("public modulus = %s, want %s", pub.N.String(), modulus.String())
	}
}
