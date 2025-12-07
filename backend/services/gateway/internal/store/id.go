package store

import (
	"crypto/rand"
	"encoding/hex"
)

// NewID returns a random hex string suitable as an identifier.
func NewID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "id-unknown"
	}
	return hex.EncodeToString(b[:])
}
