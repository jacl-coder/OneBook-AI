package servicetoken

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

const (
	// DefaultTokenTTL is the default lifetime for internal service tokens.
	DefaultTokenTTL = 60 * time.Second
	// DefaultLeeway is clock skew tolerance for token validation.
	DefaultLeeway = 15 * time.Second
	// DefaultKeyID is the default key id used for internal RS256 JWT.
	DefaultKeyID = "internal-active"
)

// Signer issues short-lived internal service JWTs.
type Signer struct {
	issuer string
	ttl    time.Duration

	rsaSigner *rsa.PrivateKey
	rsaKid    string
}

// SignerOptions configures internal service token signing.
type SignerOptions struct {
	PrivateKeyPath string
	KeyID          string
	Issuer         string
	TTL            time.Duration
}

// Verifier validates internal service JWTs against audience and issuer allowlist.
type Verifier struct {
	audience       string
	allowedIssuers map[string]struct{}
	leeway         time.Duration

	rsaVerifiers map[string]*rsa.PublicKey
}

// VerifierOptions configures internal service token verification.
type VerifierOptions struct {
	PublicKeyPath      string
	VerifyPublicKeyMap map[string]string
	DefaultKeyID       string
	Audience           string
	AllowedIssuers     []string
	Leeway             time.Duration
}

// NewSignerWithOptions creates a signer using RS256.
func NewSignerWithOptions(opts SignerOptions) (*Signer, error) {
	opts.Issuer = strings.TrimSpace(opts.Issuer)
	if opts.Issuer == "" {
		return nil, errors.New("service token issuer is required")
	}
	if opts.TTL <= 0 {
		opts.TTL = DefaultTokenTTL
	}
	keyID := strings.TrimSpace(opts.KeyID)
	if keyID == "" {
		keyID = DefaultKeyID
	}
	path := strings.TrimSpace(opts.PrivateKeyPath)
	if path == "" {
		return nil, errors.New("service token private key path is required")
	}
	key, err := loadRSAPrivateKeyFromPEMFile(path)
	if err != nil {
		return nil, fmt.Errorf("load internal jwt private key: %w", err)
	}
	return &Signer{
		issuer:    opts.Issuer,
		ttl:       opts.TTL,
		rsaSigner: key,
		rsaKid:    keyID,
	}, nil
}

// Sign issues a token for the given audience.
func (s *Signer) Sign(audience string) (string, error) {
	audience = strings.TrimSpace(audience)
	if audience == "" {
		return "", errors.New("service token audience is required")
	}
	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Issuer:    s.issuer,
		Subject:   s.issuer,
		Audience:  jwt.ClaimStrings{audience},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		ID:        randomHexID(12),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	t.Header["kid"] = s.rsaKid
	return t.SignedString(s.rsaSigner)
}

// NewVerifierWithOptions creates a verifier using RSA public keys.
func NewVerifierWithOptions(opts VerifierOptions) (*Verifier, error) {
	audience := strings.TrimSpace(opts.Audience)
	if audience == "" {
		return nil, errors.New("service token audience is required")
	}
	issuers := make(map[string]struct{})
	for _, issuer := range opts.AllowedIssuers {
		issuer = strings.TrimSpace(issuer)
		if issuer == "" {
			continue
		}
		issuers[issuer] = struct{}{}
	}
	if len(issuers) == 0 {
		return nil, errors.New("at least one allowed issuer is required")
	}
	leeway := opts.Leeway
	if leeway <= 0 {
		leeway = DefaultLeeway
	}
	verifier := &Verifier{
		audience:       audience,
		allowedIssuers: issuers,
		leeway:         leeway,
		rsaVerifiers:   make(map[string]*rsa.PublicKey),
	}
	defaultKid := strings.TrimSpace(opts.DefaultKeyID)
	if defaultKid == "" {
		defaultKid = DefaultKeyID
	}
	if path := strings.TrimSpace(opts.PublicKeyPath); path != "" {
		pub, err := loadRSAPublicKeyFromPEMFile(path)
		if err != nil {
			return nil, fmt.Errorf("load internal jwt public key: %w", err)
		}
		verifier.rsaVerifiers[defaultKid] = pub
	}
	for kid, path := range opts.VerifyPublicKeyMap {
		kid = strings.TrimSpace(kid)
		path = strings.TrimSpace(path)
		if kid == "" || path == "" {
			continue
		}
		pub, err := loadRSAPublicKeyFromPEMFile(path)
		if err != nil {
			return nil, fmt.Errorf("load internal verify key %q: %w", kid, err)
		}
		verifier.rsaVerifiers[kid] = pub
	}
	if len(verifier.rsaVerifiers) == 0 {
		return nil, errors.New("internal service verifier requires rsa public key")
	}
	return verifier, nil
}

// Verify validates token signature, expiry, audience, and issuer.
func (v *Verifier) Verify(token string) (jwt.RegisteredClaims, error) {
	claims := jwt.RegisteredClaims{}
	token = strings.TrimSpace(token)
	if token == "" {
		return claims, errors.New("token required")
	}
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, errors.New("unsupported signing method")
		}
		kid, _ := t.Header["kid"].(string)
		kid = strings.TrimSpace(kid)
		if kid == "" {
			return nil, errors.New("token key id required")
		}
		pub, ok := v.rsaVerifiers[kid]
		if !ok {
			return nil, errors.New("unknown token key")
		}
		return pub, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
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
	if _, ok := v.allowedIssuers[claims.Issuer]; !ok {
		return claims, errors.New("issuer not allowed")
	}
	if claims.ID == "" {
		return claims, errors.New("jti required")
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return claims, errors.New("subject required")
	}
	return claims, nil
}

// BearerToken extracts a bearer token from request header.
func BearerToken(r *http.Request) (string, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return "", false
	}
	return token, true
}

func randomHexID(nBytes int) string {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", buf)
}

func loadRSAPrivateKeyFromPEMFile(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid pem")
	}
	if pkcs1, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return pkcs1, nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	privateKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not rsa")
	}
	return privateKey, nil
}

func loadRSAPublicKeyFromPEMFile(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid pem")
	}
	if pubAny, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		pub, ok := pubAny.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("public key is not rsa")
		}
		return pub, nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("certificate key is not rsa")
	}
	return pub, nil
}

// ParseVerifyPublicKeys parses "kid=path,kid2=path2" into a map.
func ParseVerifyPublicKeys(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	pairs := strings.Split(raw, ",")
	out := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid verify key entry %q", pair)
		}
		kid := strings.TrimSpace(parts[0])
		path := strings.TrimSpace(parts[1])
		if kid == "" || path == "" {
			return nil, fmt.Errorf("invalid verify key entry %q", pair)
		}
		out[kid] = path
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
