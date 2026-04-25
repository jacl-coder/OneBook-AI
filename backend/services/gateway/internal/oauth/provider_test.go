package oauth

import (
	"encoding/base64"
	"math/big"
	"testing"
)

func TestGoogleJWKPublicKeyFromModulusExponent(t *testing.T) {
	modulus := new(big.Int).SetUint64(65537 * 65539)
	key := googleJWK{
		KID: "test-key",
		KTY: "RSA",
		N:   base64.RawURLEncoding.EncodeToString(modulus.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}),
	}
	pub, err := key.publicKey()
	if err != nil {
		t.Fatalf("publicKey() error = %v", err)
	}
	if pub.E != 65537 {
		t.Fatalf("public exponent = %d, want 65537", pub.E)
	}
	if pub.N.Cmp(modulus) != 0 {
		t.Fatalf("public modulus = %s, want %s", pub.N.String(), modulus.String())
	}
}
