package store

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

func TestJWTSessionStoreEnforcesAudience(t *testing.T) {
	signing := newRSStoreWithOptions(t, "aud-signing", nil, JWTOptions{
		Issuer:   "issuer-a",
		Audience: "aud-a",
		Leeway:   time.Second,
	})
	verify := newRSStoreWithOptions(t, "aud-verify", nil, JWTOptions{
		Issuer:   "issuer-a",
		Audience: "aud-b",
		Leeway:   time.Second,
	})

	token, err := signing.NewSession("user-claim")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if _, _, err := verify.GetUserIDByToken(token); err == nil {
		t.Fatalf("expected audience mismatch to fail")
	}
}

func TestJWTSessionStoreRevokesByJTI(t *testing.T) {
	revoker := NewMemoryTokenRevoker()
	store := newRSStoreWithOptions(t, "revoke-jti", revoker, JWTOptions{})

	token, err := store.NewSession("user-revoke")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if err := store.DeleteSession(token); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if _, ok, err := store.GetUserIDByToken(token); err == nil || ok {
		t.Fatalf("expected revoked token to fail, ok=%v err=%v", ok, err)
	}
}

func TestJWTSessionStoreRevokesByUserCutoff(t *testing.T) {
	revoker := NewMemoryTokenRevoker()
	store := newRSStoreWithOptions(t, "revoke-user", revoker, JWTOptions{})

	token, err := store.NewSession("user-cutoff")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if err := revoker.RevokeUser("user-cutoff", time.Now().UTC()); err != nil {
		t.Fatalf("revoke user: %v", err)
	}
	if _, ok, err := store.GetUserIDByToken(token); err == nil || ok {
		t.Fatalf("expected user-revoked token to fail, ok=%v err=%v", ok, err)
	}
}

func TestJWTRS256SessionStoreNewSessionAndJWKS(t *testing.T) {
	privatePath, publicPath := writeRSAKeyPairFiles(t, "active")

	s, err := NewJWTRS256SessionStoreFromPEM(
		privatePath,
		publicPath,
		"kid-active",
		nil,
		time.Minute,
		NewMemoryTokenRevoker(),
	)
	if err != nil {
		t.Fatalf("new rs256 store: %v", err)
	}

	token, err := s.NewSession("user-1")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if token == "" {
		t.Fatalf("expected token")
	}

	userID, ok, err := s.GetUserIDByToken(token)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if !ok || userID != "user-1" {
		t.Fatalf("unexpected verify result: ok=%v userID=%q", ok, userID)
	}

	keys := s.JWKS()
	if len(keys) != 1 {
		t.Fatalf("expected 1 jwk, got %d", len(keys))
	}
	if keys[0].Kid != "kid-active" {
		t.Fatalf("unexpected kid: %q", keys[0].Kid)
	}
	if keys[0].Kty != "RSA" || keys[0].Use != "sig" || keys[0].Alg != "RS256" {
		t.Fatalf("unexpected jwk fields: %+v", keys[0])
	}
	if keys[0].N == "" || keys[0].E == "" {
		t.Fatalf("expected RSA modulus/exponent in jwks")
	}
}

func TestJWTRS256SessionStoreVerifiesPreviousKeyDuringRotation(t *testing.T) {
	oldPrivatePath, oldPublicPath := writeRSAKeyPairFiles(t, "old")
	newPrivatePath, newPublicPath := writeRSAKeyPairFiles(t, "new")

	oldStore, err := NewJWTRS256SessionStoreFromPEM(
		oldPrivatePath,
		oldPublicPath,
		"kid-old",
		nil,
		time.Minute,
		nil,
	)
	if err != nil {
		t.Fatalf("new old store: %v", err)
	}
	oldToken, err := oldStore.NewSession("user-2")
	if err != nil {
		t.Fatalf("old token: %v", err)
	}

	newStore, err := NewJWTRS256SessionStoreFromPEM(
		newPrivatePath,
		newPublicPath,
		"kid-new",
		map[string]string{"kid-old": oldPublicPath},
		time.Minute,
		nil,
	)
	if err != nil {
		t.Fatalf("new rotated store: %v", err)
	}

	userID, ok, err := newStore.GetUserIDByToken(oldToken)
	if err != nil {
		t.Fatalf("verify old token with rotated store: %v", err)
	}
	if !ok || userID != "user-2" {
		t.Fatalf("unexpected verify result: ok=%v userID=%q", ok, userID)
	}

	keys := newStore.JWKS()
	if len(keys) != 2 {
		t.Fatalf("expected 2 jwks entries, got %d", len(keys))
	}
}

func TestJWTRS256SessionStoreRejectsUnknownKid(t *testing.T) {
	oldPrivatePath, oldPublicPath := writeRSAKeyPairFiles(t, "old")
	newPrivatePath, newPublicPath := writeRSAKeyPairFiles(t, "new")

	oldStore, err := NewJWTRS256SessionStoreFromPEM(
		oldPrivatePath,
		oldPublicPath,
		"kid-old",
		nil,
		time.Minute,
		nil,
	)
	if err != nil {
		t.Fatalf("new old store: %v", err)
	}
	oldToken, err := oldStore.NewSession("user-3")
	if err != nil {
		t.Fatalf("old token: %v", err)
	}

	newStore, err := NewJWTRS256SessionStoreFromPEM(
		newPrivatePath,
		newPublicPath,
		"kid-new",
		nil,
		time.Minute,
		nil,
	)
	if err != nil {
		t.Fatalf("new rotated store: %v", err)
	}

	if _, _, err := newStore.GetUserIDByToken(oldToken); err == nil {
		t.Fatalf("expected error for unknown kid")
	}
}

func TestJWTSessionStoreRejectsFutureIssuedAt(t *testing.T) {
	s := newRSStoreWithOptions(t, "future-iat", nil, JWTOptions{
		Issuer:   "issuer-a",
		Audience: "aud-a",
		Leeway:   time.Second,
	})

	privatePath, _ := writeRSAKeyPairFiles(t, "future-iat-sign")
	privateKey, err := loadRSAPrivateKeyFromPEMFile(privatePath)
	if err != nil {
		t.Fatalf("load private key: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Subject:   "user-future",
		Issuer:    "issuer-a",
		Audience:  jwt.ClaimStrings{"aud-a"},
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(2 * time.Minute)),
		NotBefore: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		ID:        "jti-future",
	})
	token.Header["kid"] = "jwt-active"
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, _, err := s.GetUserIDByToken(signed); err == nil {
		t.Fatalf("expected future iat token to fail")
	}
}

func TestJWTSessionStoreRequiresKidHeader(t *testing.T) {
	privatePath, publicPath := writeRSAKeyPairFiles(t, "missing-kid")
	s, err := NewJWTRS256SessionStoreFromPEM(
		privatePath,
		publicPath,
		"jwt-active",
		nil,
		time.Minute,
		nil,
	)
	if err != nil {
		t.Fatalf("new rs256 store: %v", err)
	}
	privateKey, err := loadRSAPrivateKeyFromPEMFile(privatePath)
	if err != nil {
		t.Fatalf("load private key: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Subject:   "user-missing-kid",
		Issuer:    "onebook-auth",
		Audience:  jwt.ClaimStrings{"onebook-api"},
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		NotBefore: jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(5 * time.Minute)),
		ID:        "jti-missing-kid",
	})
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, _, err := s.GetUserIDByToken(signed); err == nil {
		t.Fatalf("expected missing kid token to fail")
	}
}

func TestJWTSessionStoreRequiresJTIClaim(t *testing.T) {
	privatePath, publicPath := writeRSAKeyPairFiles(t, "missing-jti")
	s, err := NewJWTRS256SessionStoreFromPEM(
		privatePath,
		publicPath,
		"jwt-active",
		nil,
		time.Minute,
		nil,
	)
	if err != nil {
		t.Fatalf("new rs256 store: %v", err)
	}
	privateKey, err := loadRSAPrivateKeyFromPEMFile(privatePath)
	if err != nil {
		t.Fatalf("load private key: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Subject:   "user-missing-jti",
		Issuer:    "onebook-auth",
		Audience:  jwt.ClaimStrings{"onebook-api"},
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		NotBefore: jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(5 * time.Minute)),
	})
	token.Header["kid"] = "jwt-active"
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, _, err := s.GetUserIDByToken(signed); err == nil {
		t.Fatalf("expected missing jti token to fail")
	}
}

func writeRSAKeyPairFiles(t *testing.T, prefix string) (string, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	dir := t.TempDir()
	privatePath := filepath.Join(dir, prefix+"-private.pem")
	publicPath := filepath.Join(dir, prefix+"-public.pem")

	privateDER := x509.MarshalPKCS1PrivateKey(key)
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateDER})
	if err := os.WriteFile(privatePath, privatePEM, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	if err := os.WriteFile(publicPath, publicPEM, 0o644); err != nil {
		t.Fatalf("write public key: %v", err)
	}

	return privatePath, publicPath
}

func newRSStoreWithOptions(t *testing.T, prefix string, revoker TokenRevoker, opts JWTOptions) *JWTSessionStore {
	t.Helper()
	privatePath, publicPath := writeRSAKeyPairFiles(t, prefix)
	store, err := NewJWTRS256SessionStoreFromPEMWithOptions(
		privatePath,
		publicPath,
		"jwt-active",
		nil,
		time.Minute,
		revoker,
		opts,
	)
	if err != nil {
		t.Fatalf("new rs store: %v", err)
	}
	return store
}
