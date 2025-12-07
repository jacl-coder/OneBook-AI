package store

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// JWTSessionStore issues and validates HMAC-SHA256 JWT tokens.
type JWTSessionStore struct {
	secret []byte
	ttl    time.Duration
}

type jwtClaims struct {
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
	Iat int64  `json:"iat"`
}

// NewJWTSessionStore builds a stateless JWT session store.
func NewJWTSessionStore(secret string, ttl time.Duration) *JWTSessionStore {
	return &JWTSessionStore{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

// NewSession creates a signed JWT for the user ID.
func (s *JWTSessionStore) NewSession(userID string) (string, error) {
	now := time.Now().UTC()
	exp := now.Add(s.ttl)
	claims := jwtClaims{
		Sub: userID,
		Exp: exp.Unix(),
		Iat: now.Unix(),
	}
	return signJWT(claims, s.secret)
}

// GetUserIDByToken validates a JWT and returns the subject.
func (s *JWTSessionStore) GetUserIDByToken(token string) (string, bool, error) {
	claims, err := parseAndVerify(token, s.secret)
	if err != nil {
		return "", false, err
	}
	if time.Unix(claims.Exp, 0).Before(time.Now().UTC()) {
		return "", false, errors.New("token expired")
	}
	return claims.Sub, true, nil
}

// DeleteSession is a no-op for stateless JWT; provided for interface parity.
func (s *JWTSessionStore) DeleteSession(_ string) error {
	return nil
}

const headerSegment = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9" // {"alg":"HS256","typ":"JWT"}

func signJWT(claims jwtClaims, secret []byte) (string, error) {
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	unsigned := headerSegment + "." + payload
	sig := signHS256(unsigned, secret)
	return unsigned + "." + sig, nil
}

func parseAndVerify(token string, secret []byte) (jwtClaims, error) {
	var claims jwtClaims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, errors.New("invalid token format")
	}
	unsigned := parts[0] + "." + parts[1]
	expectedSig := signHS256(unsigned, secret)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return claims, errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, errors.New("invalid payload encoding")
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return claims, errors.New("invalid payload")
	}
	return claims, nil
}

func signHS256(data string, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
