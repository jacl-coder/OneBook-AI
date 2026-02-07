package server

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	jwt "github.com/golang-jwt/jwt/v5"
	"onebookai/internal/usertoken"
	"onebookai/pkg/domain"
	"onebookai/services/gateway/internal/authclient"
)

func TestAuthenticatedRouteRequiresValidTokenAndAuthoritativeUser(t *testing.T) {
	verifier, signer, err := newJWKSVerifier(t)
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}
	validToken := mustSignUserToken(t, signer, "user-1")
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate invalid key: %v", err)
	}
	invalidToken := mustSignUserToken(t, otherKey, "user-1")

	var meCalls int32
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/me" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&meCalls, 1)
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader != "Bearer "+validToken {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		_ = json.NewEncoder(w).Encode(domain.User{
			ID:     "user-1",
			Email:  "u@example.com",
			Role:   domain.RoleUser,
			Status: domain.StatusActive,
		})
	}))
	defer authSrv.Close()
	redis := miniredis.RunT(t)

	gw, err := New(Config{
		Auth:          authclient.NewClient(authSrv.URL),
		TokenVerifier: verifier,
		RedisAddr:     redis.Addr(),
	})
	if err != nil {
		t.Fatalf("new gateway server: %v", err)
	}
	gwSrv := httptest.NewServer(gw.Router())
	defer gwSrv.Close()

	// 1) Missing token.
	resp, err := http.Get(gwSrv.URL + "/api/users/me")
	if err != nil {
		t.Fatalf("request missing token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing token expected 401, got %d", resp.StatusCode)
	}

	// 2) Invalid signature token should be blocked before auth/me.
	req, _ := http.NewRequest(http.MethodGet, gwSrv.URL+"/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+invalidToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request invalid token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid token expected 401, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&meCalls); got != 0 {
		t.Fatalf("auth/me should not be called for invalid signature token, got %d calls", got)
	}

	// 3) Valid token should pass and /api/users/me should make one authoritative call via middleware.
	req, _ = http.NewRequest(http.MethodGet, gwSrv.URL+"/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request valid token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("valid token expected 200, got %d", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&meCalls); got != 1 {
		t.Fatalf("expected one auth/me call for GET /api/users/me, got %d", got)
	}
}

func TestAdminRouteUsesAuthoritativeRole(t *testing.T) {
	verifier, signer, err := newJWKSVerifier(t)
	if err != nil {
		t.Fatalf("new verifier: %v", err)
	}
	token := mustSignUserToken(t, signer, "user-1")

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/me":
			_ = json.NewEncoder(w).Encode(domain.User{
				ID:     "user-1",
				Email:  "u@example.com",
				Role:   domain.RoleUser, // explicitly non-admin
				Status: domain.StatusActive,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer authSrv.Close()
	redis := miniredis.RunT(t)

	gw, err := New(Config{
		Auth:          authclient.NewClient(authSrv.URL),
		TokenVerifier: verifier,
		RedisAddr:     redis.Addr(),
	})
	if err != nil {
		t.Fatalf("new gateway server: %v", err)
	}
	gwSrv := httptest.NewServer(gw.Router())
	defer gwSrv.Close()

	req, _ := http.NewRequest(http.MethodGet, gwSrv.URL+"/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request admin route: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-admin expected 403, got %d", resp.StatusCode)
	}
}

func newJWKSVerifier(t *testing.T) (*usertoken.Verifier, *rsa.PrivateKey, error) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{
				{
					"kty": "RSA",
					"kid": "kid-1",
					"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
				},
			},
		})
	}))
	t.Cleanup(jwksServer.Close)

	verifier, err := usertoken.NewVerifier(usertoken.Config{
		JWKSURL:  jwksServer.URL,
		Issuer:   "onebook-auth",
		Audience: "onebook-api",
		Leeway:   30 * time.Second,
	})
	if err != nil {
		return nil, nil, err
	}
	return verifier, key, nil
}

func mustSignUserToken(t *testing.T, key *rsa.PrivateKey, subject string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.RegisteredClaims{
		Subject:   subject,
		Issuer:    "onebook-auth",
		Audience:  jwt.ClaimStrings{"onebook-api"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Second)),
	})
	token.Header["kid"] = "kid-1"
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}
