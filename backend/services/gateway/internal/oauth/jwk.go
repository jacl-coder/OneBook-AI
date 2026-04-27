package oauth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
)

type rsaJWKS struct {
	Keys []rsaJWK `json:"keys"`
}

type rsaJWK struct {
	KID string   `json:"kid"`
	KTY string   `json:"kty"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5C []string `json:"x5c"`
}

func fetchRSAPublicKey(ctx context.Context, client *http.Client, jwksURL, kid, provider string) (*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s jwks fetch failed: status=%d", provider, resp.StatusCode)
	}
	var jwks rsaJWKS
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, err
	}
	for _, key := range jwks.Keys {
		if key.KID != kid {
			continue
		}
		return key.publicKey(provider)
	}
	return nil, fmt.Errorf("%s jwks kid not found", provider)
}

func (key rsaJWK) publicKey(provider string) (*rsa.PublicKey, error) {
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
			return nil, fmt.Errorf("%s cert public key is not rsa", provider)
		}
		return pub, nil
	}
	if strings.TrimSpace(key.KTY) != "" && !strings.EqualFold(key.KTY, "RSA") {
		return nil, fmt.Errorf("%s jwk key type %q is not rsa", provider, key.KTY)
	}
	if strings.TrimSpace(key.N) == "" || strings.TrimSpace(key.E) == "" {
		return nil, fmt.Errorf("%s jwk missing rsa key material", provider)
	}
	modulus, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("decode %s jwk modulus: %w", provider, err)
	}
	exponent, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("decode %s jwk exponent: %w", provider, err)
	}
	e := new(big.Int).SetBytes(exponent)
	if !e.IsInt64() {
		return nil, fmt.Errorf("%s jwk exponent is too large", provider)
	}
	eInt := int(e.Int64())
	if eInt <= 1 {
		return nil, fmt.Errorf("%s jwk exponent is invalid", provider)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulus),
		E: eInt,
	}, nil
}
