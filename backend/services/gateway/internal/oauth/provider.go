package oauth

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	ProviderGoogle     = "google"
	googleAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL     = "https://oauth2.googleapis.com/token"
	googleJWKSURL      = "https://www.googleapis.com/oauth2/v3/certs"
)

// Config contains OAuth provider credentials.
type Config struct {
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
}

// Provider implements provider-specific OAuth authorization, token exchange, and identity verification.
type Provider interface {
	Name() string
	Configured() bool
	AuthCodeURL(state, nonce, codeVerifier string) (string, error)
	ExchangeCode(context.Context, string, string) (TokenResponse, error)
	VerifyIDToken(context.Context, string, string) (Identity, error)
}

// TokenResponse is the provider-neutral token exchange result used by the gateway callback flow.
type TokenResponse struct {
	IDToken string `json:"id_token"`
}

// Identity is the verified OAuth identity returned by a provider.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

type providerFactory func(Config) Provider

var providerFactories = map[string]providerFactory{
	ProviderGoogle: newGoogleProvider,
}

type googleProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	httpClient   *http.Client
}

// NewProviders creates every supported OAuth provider from shared config.
func NewProviders(cfg Config) map[string]Provider {
	providers := make(map[string]Provider, len(providerFactories))
	for name, factory := range providerFactories {
		providers[name] = factory(cfg)
	}
	return providers
}

func newGoogleProvider(cfg Config) Provider {
	return googleProvider{
		clientID:     strings.TrimSpace(cfg.GoogleClientID),
		clientSecret: strings.TrimSpace(cfg.GoogleClientSecret),
		redirectURL:  strings.TrimSpace(cfg.GoogleRedirectURL),
		httpClient:   http.DefaultClient,
	}
}

func (p googleProvider) Name() string {
	return ProviderGoogle
}

func (p googleProvider) Configured() bool {
	return p.clientID != "" && p.clientSecret != "" && p.redirectURL != ""
}

func (p googleProvider) client() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return http.DefaultClient
}

func (p googleProvider) AuthCodeURL(state, nonce, codeVerifier string) (string, error) {
	authURL, err := url.Parse(googleAuthorizeURL)
	if err != nil {
		return "", err
	}
	query := authURL.Query()
	query.Set("client_id", p.clientID)
	query.Set("redirect_uri", p.redirectURL)
	query.Set("response_type", "code")
	query.Set("scope", "openid email profile")
	query.Set("state", state)
	query.Set("nonce", nonce)
	query.Set("code_challenge", pkceChallenge(codeVerifier))
	query.Set("code_challenge_method", "S256")
	query.Set("prompt", "select_account")
	authURL.RawQuery = query.Encode()
	return authURL.String(), nil
}

func (p googleProvider) ExchangeCode(ctx context.Context, code, codeVerifier string) (TokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)
	form.Set("code", code)
	form.Set("code_verifier", codeVerifier)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", p.redirectURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client().Do(req)
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TokenResponse{}, fmt.Errorf("google token exchange failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return TokenResponse{}, err
	}
	if strings.TrimSpace(tokenResp.IDToken) == "" {
		return TokenResponse{}, errors.New("google token response missing id_token")
	}
	return tokenResp, nil
}

func (p googleProvider) VerifyIDToken(ctx context.Context, rawToken, nonce string) (Identity, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("unexpected google signing method %s", token.Method.Alg())
		}
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("google token missing kid")
		}
		return p.publicKey(ctx, kid)
	})
	if err != nil {
		return Identity{}, err
	}
	if !token.Valid {
		return Identity{}, errors.New("google id token invalid")
	}
	if !validGoogleIssuer(claimString(claims, "iss")) {
		return Identity{}, errors.New("google id token issuer invalid")
	}
	if !claimAudienceContains(claims, p.clientID) {
		return Identity{}, errors.New("google id token audience invalid")
	}
	if exp, err := claims.GetExpirationTime(); err != nil || exp == nil || time.Now().After(exp.Time) {
		return Identity{}, errors.New("google id token expired")
	}
	if tokenNonce := claimString(claims, "nonce"); tokenNonce == "" || tokenNonce != nonce {
		return Identity{}, errors.New("google id token nonce invalid")
	}
	subject := claimString(claims, "sub")
	if subject == "" {
		return Identity{}, errors.New("google id token subject missing")
	}
	return Identity{
		Subject:       subject,
		Email:         claimString(claims, "email"),
		EmailVerified: claimBool(claims, "email_verified"),
		Name:          claimString(claims, "name"),
		Picture:       claimString(claims, "picture"),
	}, nil
}

type googleJWKS struct {
	Keys []googleJWK `json:"keys"`
}

type googleJWK struct {
	KID string   `json:"kid"`
	KTY string   `json:"kty"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5C []string `json:"x5c"`
}

func (p googleProvider) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleJWKSURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google jwks fetch failed: status=%d", resp.StatusCode)
	}
	var jwks googleJWKS
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, err
	}
	for _, key := range jwks.Keys {
		if key.KID != kid {
			continue
		}
		return key.publicKey()
	}
	return nil, errors.New("google jwks kid not found")
}

func (key googleJWK) publicKey() (*rsa.PublicKey, error) {
	if len(key.X5C) > 0 {
		rawCert, err := base64.StdEncoding.DecodeString(key.X5C[0])
		if err != nil {
			return nil, err
		}
		cert, err := x509.ParseCertificate(rawCert)
		if err != nil {
			return nil, err
		}
		pub, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("google cert public key is not rsa")
		}
		return pub, nil
	}
	if strings.TrimSpace(key.KTY) != "" && !strings.EqualFold(key.KTY, "RSA") {
		return nil, fmt.Errorf("google jwk key type %q is not rsa", key.KTY)
	}
	if strings.TrimSpace(key.N) == "" || strings.TrimSpace(key.E) == "" {
		return nil, errors.New("google jwk missing rsa key material")
	}
	modulus, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("decode google jwk modulus: %w", err)
	}
	exponent, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("decode google jwk exponent: %w", err)
	}
	e := new(big.Int).SetBytes(exponent)
	if !e.IsInt64() {
		return nil, errors.New("google jwk exponent is too large")
	}
	eInt := int(e.Int64())
	if eInt <= 1 {
		return nil, errors.New("google jwk exponent is invalid")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulus),
		E: eInt,
	}, nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func validGoogleIssuer(issuer string) bool {
	return issuer == "https://accounts.google.com" || issuer == "accounts.google.com"
}

func claimString(claims jwt.MapClaims, key string) string {
	value, _ := claims[key].(string)
	return strings.TrimSpace(value)
}

func claimBool(claims jwt.MapClaims, key string) bool {
	switch value := claims[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(value, "true")
	default:
		return false
	}
}

func claimAudienceContains(claims jwt.MapClaims, audience string) bool {
	switch value := claims["aud"].(type) {
	case string:
		return value == audience
	case []string:
		for _, item := range value {
			if item == audience {
				return true
			}
		}
	case []any:
		for _, item := range value {
			if str, ok := item.(string); ok && str == audience {
				return true
			}
		}
	}
	return false
}
