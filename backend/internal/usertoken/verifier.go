package usertoken

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

const (
	defaultIssuer       = "onebook-auth"
	defaultAudience     = "onebook-api"
	defaultLeeway       = 30 * time.Second
	defaultJWKSCacheTTL = 5 * time.Minute
)

var errUnknownKey = errors.New("unknown token key")

// Config configures user access-token verification.
type Config struct {
	JWKSURL    string
	Issuer     string
	Audience   string
	Leeway     time.Duration
	HTTPClient *http.Client
}

// Verifier validates user access tokens and extracts subject (RS256 + JWKS).
type Verifier struct {
	issuer     string
	audience   string
	leeway     time.Duration
	jwksURL    string
	httpClient *http.Client

	mu         sync.RWMutex
	rsaKeys    map[string]any
	keysExpire time.Time
}

// NewVerifier creates a token verifier.
func NewVerifier(cfg Config) (*Verifier, error) {
	issuer := strings.TrimSpace(cfg.Issuer)
	if issuer == "" {
		issuer = defaultIssuer
	}
	audience := strings.TrimSpace(cfg.Audience)
	if audience == "" {
		audience = defaultAudience
	}
	leeway := cfg.Leeway
	if leeway <= 0 {
		leeway = defaultLeeway
	}

	v := &Verifier{
		issuer:   issuer,
		audience: audience,
		leeway:   leeway,
	}

	jwksURL := strings.TrimSpace(cfg.JWKSURL)
	if jwksURL == "" {
		return nil, errors.New("token verifier requires jwksURL")
	}
	v.jwksURL = jwksURL
	if cfg.HTTPClient != nil {
		v.httpClient = cfg.HTTPClient
	} else {
		v.httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	if err := v.refreshJWKS(); err != nil {
		return nil, err
	}

	return v, nil
}

// VerifySubject validates the token and returns subject user ID.
func (v *Verifier) VerifySubject(token string) (string, error) {
	claims, err := v.verifyJWKS(token)
	if err != nil {
		return "", err
	}
	subject := strings.TrimSpace(claims.Subject)
	if subject == "" {
		return "", errors.New("token subject missing")
	}
	return subject, nil
}

func (v *Verifier) verifyJWKS(token string) (jwt.RegisteredClaims, error) {
	claims, err := v.parseJWKS(token)
	if err == nil {
		return claims, nil
	}
	if !errors.Is(err, errUnknownKey) && !v.keysExpired() {
		return claims, err
	}
	if refreshErr := v.refreshJWKS(); refreshErr != nil {
		return claims, refreshErr
	}
	return v.parseJWKS(token)
}

func (v *Verifier) parseJWKS(token string) (jwt.RegisteredClaims, error) {
	claims := jwt.RegisteredClaims{}
	keys := v.copyKeys()
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		kid = strings.TrimSpace(kid)
		if kid == "" {
			return nil, errUnknownKey
		}
		key, ok := keys[kid]
		if !ok {
			return nil, errUnknownKey
		}
		return key, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(v.leeway),
	)
	if err != nil || !parsed.Valid {
		if err == nil {
			err = errors.New("invalid token")
		}
		return claims, err
	}
	return claims, nil
}

func (v *Verifier) keysExpired() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return time.Now().UTC().After(v.keysExpire)
}

func (v *Verifier) copyKeys() map[string]any {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make(map[string]any, len(v.rsaKeys))
	for kid, key := range v.rsaKeys {
		out[kid] = key
	}
	return out
}

func (v *Verifier) refreshJWKS() error {
	req, err := http.NewRequest(http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch jwks: status %d", resp.StatusCode)
	}

	var payload struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}

	keys := make(map[string]any, len(payload.Keys))
	for _, k := range payload.Keys {
		if strings.ToUpper(strings.TrimSpace(k.Kty)) != "RSA" {
			continue
		}
		kid := strings.TrimSpace(k.Kid)
		if kid == "" {
			continue
		}
		pub, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[kid] = pub
	}
	if len(keys) == 0 {
		return errors.New("jwks contains no usable rsa keys")
	}

	ttl := parseCacheMaxAge(resp.Header.Get("Cache-Control"))
	if ttl <= 0 {
		ttl = defaultJWKSCacheTTL
	}

	v.mu.Lock()
	v.rsaKeys = keys
	v.keysExpire = time.Now().UTC().Add(ttl)
	v.mu.Unlock()
	return nil
}

func parseRSAPublicKey(nRaw, eRaw string) (any, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(nRaw))
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(eRaw))
	if err != nil {
		return nil, err
	}
	n := new(big.Int).SetBytes(nBytes)
	eBig := new(big.Int).SetBytes(eBytes)
	if n.Sign() <= 0 || !eBig.IsInt64() {
		return nil, errors.New("invalid rsa key")
	}
	e := int(eBig.Int64())
	if e <= 0 {
		return nil, errors.New("invalid rsa exponent")
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}

func parseCacheMaxAge(cacheControl string) time.Duration {
	cacheControl = strings.TrimSpace(cacheControl)
	if cacheControl == "" {
		return 0
	}
	parts := strings.Split(cacheControl, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(strings.ToLower(part), "max-age=") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(part), "max-age="))
		secs, err := time.ParseDuration(raw + "s")
		if err != nil {
			return 0
		}
		return secs
	}
	return 0
}
