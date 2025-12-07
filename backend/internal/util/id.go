package util

import (
	"crypto/rand"
	"encoding/hex"
)

// NewID returns a URL-safe hex string ID.
func NewID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
