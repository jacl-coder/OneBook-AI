package servicetoken

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

func TestSignerVerifierRS256(t *testing.T) {
	privatePath, publicPath := writeRSAKeyPairFiles(t, "svc")
	signer, err := NewSignerWithOptions(SignerOptions{
		PrivateKeyPath: privatePath,
		KeyID:          "internal-active",
		Issuer:         "book-service",
		TTL:            2 * time.Second,
	})
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	verifier, err := NewVerifierWithOptions(VerifierOptions{
		PublicKeyPath:  publicPath,
		DefaultKeyID:   "internal-active",
		Audience:       "ingest",
		AllowedIssuers: []string{"book-service"},
		Leeway:         time.Second,
	})
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}
	token, err := signer.Sign("ingest")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := verifier.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Issuer != "book-service" {
		t.Fatalf("unexpected issuer: %s", claims.Issuer)
	}
}

func TestSignerRequiresPrivateKey(t *testing.T) {
	if _, err := NewSignerWithOptions(SignerOptions{Issuer: "book-service"}); err == nil {
		t.Fatalf("expected missing key path to fail")
	}
}

func TestVerifierRejectsWrongAudience(t *testing.T) {
	privatePath, publicPath := writeRSAKeyPairFiles(t, "aud")
	signer, _ := NewSignerWithOptions(SignerOptions{
		PrivateKeyPath: privatePath,
		KeyID:          "internal-active",
		Issuer:         "book-service",
		TTL:            time.Minute,
	})
	verifier, _ := NewVerifierWithOptions(VerifierOptions{
		PublicKeyPath:  publicPath,
		DefaultKeyID:   "internal-active",
		Audience:       "indexer",
		AllowedIssuers: []string{"book-service"},
		Leeway:         time.Second,
	})
	token, _ := signer.Sign("ingest")
	if _, err := verifier.Verify(token); err == nil {
		t.Fatalf("expected audience mismatch")
	}
}

func TestVerifierRejectsUnknownKid(t *testing.T) {
	privatePath, publicPath := writeRSAKeyPairFiles(t, "kid")
	signer, _ := NewSignerWithOptions(SignerOptions{
		PrivateKeyPath: privatePath,
		KeyID:          "kid-1",
		Issuer:         "book-service",
		TTL:            time.Minute,
	})
	verifier, _ := NewVerifierWithOptions(VerifierOptions{
		PublicKeyPath:  publicPath,
		DefaultKeyID:   "kid-2",
		Audience:       "ingest",
		AllowedIssuers: []string{"book-service"},
		Leeway:         time.Second,
	})
	token, _ := signer.Sign("ingest")
	if _, err := verifier.Verify(token); err == nil {
		t.Fatalf("expected unknown kid to fail")
	}
}

func TestVerifierRejectsFutureIssuedAt(t *testing.T) {
	privatePath, publicPath := writeRSAKeyPairFiles(t, "iat")
	verifier, err := NewVerifierWithOptions(VerifierOptions{
		PublicKeyPath:  publicPath,
		DefaultKeyID:   "internal-active",
		Audience:       "ingest",
		AllowedIssuers: []string{"book-service"},
		Leeway:         time.Second,
	})
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}
	privateKey, err := loadRSAPrivateKeyFromPEMFile(privatePath)
	if err != nil {
		t.Fatalf("load private key: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Issuer:    "book-service",
		Subject:   "book-service",
		Audience:  jwt.ClaimStrings{"ingest"},
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(2 * time.Minute)),
		NotBefore: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		ID:        "jti-1",
	})
	token.Header["kid"] = "internal-active"
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := verifier.Verify(signed); err == nil {
		t.Fatalf("expected future iat token to fail")
	}
}

func TestVerifierRequiresKidHeader(t *testing.T) {
	privatePath, publicPath := writeRSAKeyPairFiles(t, "missing-kid")
	verifier, err := NewVerifierWithOptions(VerifierOptions{
		PublicKeyPath:  publicPath,
		DefaultKeyID:   "internal-active",
		Audience:       "ingest",
		AllowedIssuers: []string{"book-service"},
		Leeway:         time.Second,
	})
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}
	privateKey, err := loadRSAPrivateKeyFromPEMFile(privatePath)
	if err != nil {
		t.Fatalf("load private key: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Issuer:    "book-service",
		Subject:   "book-service",
		Audience:  jwt.ClaimStrings{"ingest"},
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		NotBefore: jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(5 * time.Minute)),
		ID:        "jti-missing-kid",
	})
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := verifier.Verify(signed); err == nil {
		t.Fatalf("expected missing kid token to fail")
	}
}

func TestParseVerifyPublicKeys(t *testing.T) {
	parsed, err := ParseVerifyPublicKeys("k1=/a.pem,k2=/b.pem")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("unexpected parsed size: %d", len(parsed))
	}
}

func TestBearerToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer abc")
	token, ok := BearerToken(req)
	if !ok || token != "abc" {
		t.Fatalf("expected bearer token")
	}
}

func writeRSAKeyPairFiles(t *testing.T, prefix string) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	dir := t.TempDir()
	privatePath := filepath.Join(dir, prefix+"-private.pem")
	publicPath := filepath.Join(dir, prefix+"-public.pem")
	privateDER := x509.MarshalPKCS1PrivateKey(key)
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateDER})
	if err := os.WriteFile(privatePath, privatePEM, 0o600); err != nil {
		t.Fatalf("write private: %v", err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public: %v", err)
	}
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	if err := os.WriteFile(publicPath, publicPEM, 0o644); err != nil {
		t.Fatalf("write public: %v", err)
	}
	return privatePath, publicPath
}
