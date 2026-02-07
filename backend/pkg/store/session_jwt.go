package store

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

const (
	defaultJWTIssuer   = "onebook-auth"
	defaultJWTAudience = "onebook-api"
)

var defaultJWTLeeway = 30 * time.Second

// JWTOptions configures JWT claim validation behavior.
type JWTOptions struct {
	Issuer   string
	Audience string
	Leeway   time.Duration
}

// JWTSessionStore issues and validates JWT tokens.
// It uses RS256 with kid/JWKS.
type JWTSessionStore struct {
	ttl     time.Duration
	revoker TokenRevoker

	rsaSigner    *rsa.PrivateKey
	rsaSignerKid string
	rsaVerifiers map[string]*rsa.PublicKey

	issuer   string
	audience string
	leeway   time.Duration
}

// NewJWTRS256SessionStoreFromPEM builds a RS256 JWT session store from PEM files.
// verifyKeyFiles maps kid -> public key path and can include previous keys.
func NewJWTRS256SessionStoreFromPEM(
	privateKeyPath string,
	publicKeyPath string,
	keyID string,
	verifyKeyFiles map[string]string,
	ttl time.Duration,
	revoker TokenRevoker,
) (*JWTSessionStore, error) {
	return NewJWTRS256SessionStoreFromPEMWithOptions(
		privateKeyPath,
		publicKeyPath,
		keyID,
		verifyKeyFiles,
		ttl,
		revoker,
		JWTOptions{},
	)
}

// NewJWTRS256SessionStoreFromPEMWithOptions builds RS256 store with custom claim options.
func NewJWTRS256SessionStoreFromPEMWithOptions(
	privateKeyPath string,
	publicKeyPath string,
	keyID string,
	verifyKeyFiles map[string]string,
	ttl time.Duration,
	revoker TokenRevoker,
	opts JWTOptions,
) (*JWTSessionStore, error) {
	privateKey, err := loadRSAPrivateKeyFromPEMFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load jwt private key: %w", err)
	}
	if strings.TrimSpace(keyID) == "" {
		keyID = "jwt-active"
	}

	verifiers := make(map[string]*rsa.PublicKey)
	activePub := &privateKey.PublicKey
	if strings.TrimSpace(publicKeyPath) != "" {
		activePub, err = loadRSAPublicKeyFromPEMFile(publicKeyPath)
		if err != nil {
			return nil, fmt.Errorf("load jwt public key: %w", err)
		}
	}
	verifiers[keyID] = activePub

	for kid, path := range verifyKeyFiles {
		kid = strings.TrimSpace(kid)
		path = strings.TrimSpace(path)
		if kid == "" || path == "" {
			continue
		}
		pub, err := loadRSAPublicKeyFromPEMFile(path)
		if err != nil {
			return nil, fmt.Errorf("load verify key %q: %w", kid, err)
		}
		verifiers[kid] = pub
	}

	opts = normalizeJWTOptions(opts)
	return &JWTSessionStore{
		ttl:          ttl,
		revoker:      revoker,
		rsaSigner:    privateKey,
		rsaSignerKid: keyID,
		rsaVerifiers: verifiers,
		issuer:       opts.Issuer,
		audience:     opts.Audience,
		leeway:       opts.Leeway,
	}, nil
}

// NewSession creates a signed JWT for the user ID.
func (s *JWTSessionStore) NewSession(userID string) (string, error) {
	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    s.issuer,
		Audience:  jwt.ClaimStrings{s.audience},
		ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ID:        randomHexID(12),
	}

	if s.rsaSigner == nil {
		return "", errors.New("jwt store not configured")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.rsaSignerKid
	return token.SignedString(s.rsaSigner)
}

// GetUserIDByToken validates a JWT and returns the subject.
func (s *JWTSessionStore) GetUserIDByToken(token string) (string, bool, error) {
	claims, err := s.parseAndVerify(token)
	if err != nil {
		return "", false, err
	}
	if s.revoker != nil {
		revoked, err := s.revoker.IsRevoked(claims.ID)
		if err != nil {
			return "", false, err
		}
		if revoked {
			return "", false, errors.New("token revoked")
		}
		if userRevoker, ok := s.revoker.(UserTokenRevoker); ok {
			cutoff, err := userRevoker.RevokedAfter(claims.Subject)
			if err != nil {
				return "", false, err
			}
			if !cutoff.IsZero() {
				if claims.IssuedAt == nil {
					return "", false, errors.New("token issued_at missing")
				}
				issuedAt := claims.IssuedAt.Time.UTC()
				if !issuedAt.After(cutoff) {
					return "", false, errors.New("token revoked for user")
				}
			}
		}
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return "", false, errors.New("token subject missing")
	}
	return claims.Subject, true, nil
}

// DeleteSession revokes the token until it expires.
func (s *JWTSessionStore) DeleteSession(token string) error {
	if s.revoker == nil {
		return nil
	}
	claims, err := s.parseAndVerify(token)
	if err != nil {
		return nil
	}
	if claims.ExpiresAt == nil {
		return nil
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	return s.revoker.Revoke(claims.ID, ttl)
}

// RevokeUserSessions revokes all sessions for a user issued before/at cutoff.
func (s *JWTSessionStore) RevokeUserSessions(userID string, since time.Time) error {
	if s.revoker == nil {
		return nil
	}
	userRevoker, ok := s.revoker.(UserTokenRevoker)
	if !ok {
		return errors.New("session revoker does not support user revocation")
	}
	return userRevoker.RevokeUser(userID, since)
}

// JWKS returns JSON Web Keys when RS256 mode is enabled.
func (s *JWTSessionStore) JWKS() []JWK {
	if len(s.rsaVerifiers) == 0 {
		return nil
	}
	kids := make([]string, 0, len(s.rsaVerifiers))
	for kid := range s.rsaVerifiers {
		kids = append(kids, kid)
	}
	sort.Strings(kids)
	out := make([]JWK, 0, len(kids))
	for _, kid := range kids {
		pub := s.rsaVerifiers[kid]
		out = append(out, JWK{
			Kty: "RSA",
			Use: "sig",
			Kid: kid,
			Alg: "RS256",
			N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		})
	}
	return out
}

func (s *JWTSessionStore) parseAndVerify(token string) (jwt.RegisteredClaims, error) {
	claims := jwt.RegisteredClaims{}
	token = strings.TrimSpace(token)
	if token == "" {
		return claims, errors.New("invalid token format")
	}

	if len(s.rsaVerifiers) == 0 {
		return claims, errors.New("jwt store not configured")
	}
	parserOptions := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(s.leeway),
	}
	if s.issuer != "" {
		parserOptions = append(parserOptions, jwt.WithIssuer(s.issuer))
	}
	if s.audience != "" {
		parserOptions = append(parserOptions, jwt.WithAudience(s.audience))
	}
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		kid = strings.TrimSpace(kid)
		if kid == "" {
			return nil, errors.New("token key id required")
		}
		pub, ok := s.rsaVerifiers[kid]
		if !ok {
			return nil, errors.New("unknown token key")
		}
		return pub, nil
	}, parserOptions...)
	if err != nil || !parsed.Valid {
		if err == nil {
			err = errors.New("invalid token")
		}
		return claims, err
	}
	if strings.TrimSpace(claims.ID) == "" {
		return claims, errors.New("token jti missing")
	}
	return claims, nil
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
	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		pub, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("certificate public key is not rsa")
		}
		return pub, nil
	}
	return nil, errors.New("failed to parse rsa public key")
}

func randomHexID(nBytes int) string {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", buf)
}

func normalizeJWTOptions(opts JWTOptions) JWTOptions {
	opts.Issuer = strings.TrimSpace(opts.Issuer)
	opts.Audience = strings.TrimSpace(opts.Audience)
	if opts.Issuer == "" {
		opts.Issuer = defaultJWTIssuer
	}
	if opts.Audience == "" {
		opts.Audience = defaultJWTAudience
	}
	if opts.Leeway <= 0 {
		opts.Leeway = defaultJWTLeeway
	}
	return opts
}
