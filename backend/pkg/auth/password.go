package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HashPassword returns a salted hash encoded as "salt$hash".
func HashPassword(password string) string {
	salt := randomHex(8)
	h := sha256.Sum256([]byte(salt + password))
	return salt + "$" + hex.EncodeToString(h[:])
}

// CheckPassword validates a password against a salted hash.
func CheckPassword(password, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 2 {
		return false
	}
	salt := parts[0]
	h := sha256.Sum256([]byte(salt + password))
	return hex.EncodeToString(h[:]) == parts[1]
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "salt"
	}
	return hex.EncodeToString(buf)
}
