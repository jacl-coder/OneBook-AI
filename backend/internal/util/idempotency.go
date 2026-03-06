package util

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

const (
	HeaderIdempotencyKey      = "Idempotency-Key"
	HeaderIdempotencyReplayed = "Idempotency-Replayed"
)

func IdempotencyKeyFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.Header.Get(HeaderIdempotencyKey))
}

func HashStrings(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		if _, err := h.Write([]byte(strings.TrimSpace(part))); err != nil {
			continue
		}
		if _, err := h.Write([]byte{0}); err != nil {
			continue
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func HashJSON(value any) string {
	if value == nil {
		return HashStrings("{}")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return HashStrings("invalid-json")
	}
	return HashStrings(string(data))
}
