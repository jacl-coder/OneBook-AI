package oauth

import (
	"encoding/base64"
	"math/big"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestRSAJWKPublicKeyFromModulusExponent(t *testing.T) {
	modulus := new(big.Int).SetUint64(65537 * 65539)
	key := rsaJWK{
		KID: "test-key",
		KTY: "RSA",
		N:   base64.RawURLEncoding.EncodeToString(modulus.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}),
	}
	pub, err := key.publicKey("test")
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

func TestNewProvidersIncludesMicrosoft(t *testing.T) {
	providers := NewProviders(Config{})
	if _, ok := providers[ProviderGoogle]; !ok {
		t.Fatal("google provider missing")
	}
	if _, ok := providers[ProviderMicrosoft]; !ok {
		t.Fatal("microsoft provider missing")
	}
}

func TestMicrosoftAuthCodeURLUsesConfiguredTenant(t *testing.T) {
	provider := newMicrosoftProvider(Config{
		MicrosoftClientID:    "client-id",
		MicrosoftRedirectURL: "http://localhost:8081/api/auth/oauth/microsoft/callback",
		MicrosoftTenant:      "organizations",
	})
	got, err := provider.AuthCodeURL("state", "nonce", "verifier")
	if err != nil {
		t.Fatalf("AuthCodeURL() error = %v", err)
	}
	if want := "https://login.microsoftonline.com/organizations/oauth2/v2.0/authorize"; !strings.HasPrefix(got, want) {
		t.Fatalf("authorize URL = %q, want prefix %q", got, want)
	}
}

func TestValidMicrosoftIssuer(t *testing.T) {
	if !validMicrosoftIssuer("https://login.microsoftonline.com/11111111-1111-1111-1111-111111111111/v2.0", "11111111-1111-1111-1111-111111111111", "common") {
		t.Fatal("common tenant issuer rejected")
	}
	if !validMicrosoftIssuer("https://login.microsoftonline.com/11111111-1111-1111-1111-111111111111/v2.0", "11111111-1111-1111-1111-111111111111", "11111111-1111-1111-1111-111111111111") {
		t.Fatal("matching tenant issuer rejected")
	}
	if validMicrosoftIssuer("https://login.microsoftonline.com/22222222-2222-2222-2222-222222222222/v2.0", "22222222-2222-2222-2222-222222222222", "11111111-1111-1111-1111-111111111111") {
		t.Fatal("mismatched tenant issuer accepted")
	}
}

func TestMicrosoftEmailFromClaims(t *testing.T) {
	tests := []struct {
		name  string
		claim jwt.MapClaims
		want  string
		ok    bool
	}{
		{
			name:  "email claim",
			claim: map[string]any{"email": "User@Example.com"},
			want:  "user@example.com",
			ok:    true,
		},
		{
			name:  "preferred username fallback",
			claim: map[string]any{"preferred_username": "User@Example.com"},
			want:  "user@example.com",
			ok:    true,
		},
		{
			name:  "upn fallback",
			claim: map[string]any{"upn": "User@Example.com"},
			want:  "user@example.com",
			ok:    true,
		},
		{
			name:  "non email preferred username ignored",
			claim: map[string]any{"preferred_username": "user-name"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := microsoftEmailFromClaims(tt.claim)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("microsoftEmailFromClaims() = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}
