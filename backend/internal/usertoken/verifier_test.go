package usertoken

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

func TestNewVerifierRequiresJWKSURL(t *testing.T) {
	if _, err := NewVerifier(Config{}); err == nil {
		t.Fatalf("expected missing jwks url to fail")
	}
}

func TestJWKSVerifySubjectAndRefreshOnUnknownKid(t *testing.T) {
	key1, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key1: %v", err)
	}
	key2, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key2: %v", err)
	}

	active := "kid-1"
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=1")
		resp := map[string]any{"keys": []map[string]string{toJWK(active, publicKeyByKid(active, key1.PublicKey, key2.PublicKey))}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer jwksServer.Close()

	v, err := NewVerifier(Config{
		JWKSURL:  jwksServer.URL,
		Issuer:   "issuer-a",
		Audience: "aud-a",
	})
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}

	// First token uses kid-1.
	token1 := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Subject:   "user-a",
		Issuer:    "issuer-a",
		Audience:  jwt.ClaimStrings{"aud-a"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Second)),
	})
	token1.Header["kid"] = "kid-1"
	signed1, err := token1.SignedString(key1)
	if err != nil {
		t.Fatalf("sign token1: %v", err)
	}

	if sub, err := v.VerifySubject(signed1); err != nil || sub != "user-a" {
		t.Fatalf("verify token1 failed: sub=%s err=%v", sub, err)
	}

	// Rotate to kid-2; verifier should refresh JWKS on unknown kid and pass.
	active = "kid-2"
	token2 := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Subject:   "user-b",
		Issuer:    "issuer-a",
		Audience:  jwt.ClaimStrings{"aud-a"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Second)),
	})
	token2.Header["kid"] = "kid-2"
	signed2, err := token2.SignedString(key2)
	if err != nil {
		t.Fatalf("sign token2: %v", err)
	}

	if sub, err := v.VerifySubject(signed2); err != nil || sub != "user-b" {
		t.Fatalf("verify token2 failed: sub=%s err=%v", sub, err)
	}
}

func TestJWKSRejectsFutureIssuedAt(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{"keys": []map[string]string{toJWK("kid-1", key.PublicKey)}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer jwksServer.Close()

	v, err := NewVerifier(Config{
		JWKSURL:  jwksServer.URL,
		Issuer:   "issuer-a",
		Audience: "aud-a",
		Leeway:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Subject:   "user-1",
		Issuer:    "issuer-a",
		Audience:  jwt.ClaimStrings{"aud-a"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(2 * time.Minute)),
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Second)),
	})
	token.Header["kid"] = "kid-1"
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := v.VerifySubject(signed); err == nil {
		t.Fatalf("expected future iat token to fail")
	}
}

func toJWK(kid string, key rsa.PublicKey) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"kid": kid,
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(bigIntFromInt(key.E).Bytes()),
	}
}

func publicKeyByKid(kid string, key1, key2 rsa.PublicKey) rsa.PublicKey {
	if kid == "kid-2" {
		return key2
	}
	return key1
}

func bigIntFromInt(v int) *big.Int {
	return big.NewInt(int64(v))
}
