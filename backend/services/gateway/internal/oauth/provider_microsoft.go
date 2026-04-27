package oauth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type microsoftProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	tenant       string
	httpClient   *http.Client
}

func newMicrosoftProvider(cfg Config) Provider {
	return microsoftProvider{
		clientID:     strings.TrimSpace(cfg.MicrosoftClientID),
		clientSecret: strings.TrimSpace(cfg.MicrosoftClientSecret),
		redirectURL:  strings.TrimSpace(cfg.MicrosoftRedirectURL),
		tenant:       normalizeMicrosoftTenant(cfg.MicrosoftTenant),
		httpClient:   http.DefaultClient,
	}
}

func (p microsoftProvider) Name() string {
	return ProviderMicrosoft
}

func (p microsoftProvider) Configured() bool {
	return p.clientID != "" && p.clientSecret != "" && p.redirectURL != "" && p.tenant != ""
}

func (p microsoftProvider) client() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return http.DefaultClient
}

func (p microsoftProvider) AuthCodeURL(state, nonce, codeVerifier string) (string, error) {
	authURL, err := url.Parse(p.authorizeURL())
	if err != nil {
		return "", err
	}
	query := authURL.Query()
	query.Set("client_id", p.clientID)
	query.Set("redirect_uri", p.redirectURL)
	query.Set("response_type", "code")
	query.Set("scope", "openid profile email")
	query.Set("state", state)
	query.Set("nonce", nonce)
	query.Set("code_challenge", pkceChallenge(codeVerifier))
	query.Set("code_challenge_method", "S256")
	query.Set("prompt", "select_account")
	authURL.RawQuery = query.Encode()
	return authURL.String(), nil
}

func (p microsoftProvider) ExchangeCode(ctx context.Context, code, codeVerifier string) (TokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)
	form.Set("code", code)
	form.Set("code_verifier", codeVerifier)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", p.redirectURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL(), strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client().Do(req)
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TokenResponse{}, fmt.Errorf("microsoft token exchange failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return TokenResponse{}, err
	}
	if strings.TrimSpace(tokenResp.IDToken) == "" {
		return TokenResponse{}, errors.New("microsoft token response missing id_token")
	}
	return tokenResp, nil
}

func (p microsoftProvider) VerifyIDToken(ctx context.Context, rawToken, nonce string) (Identity, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("unexpected microsoft signing method %s", token.Method.Alg())
		}
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("microsoft token missing kid")
		}
		return p.publicKey(ctx, kid)
	})
	if err != nil {
		return Identity{}, err
	}
	if !token.Valid {
		return Identity{}, errors.New("microsoft id token invalid")
	}
	if !claimAudienceContains(claims, p.clientID) {
		return Identity{}, errors.New("microsoft id token audience invalid")
	}
	tid := claimString(claims, "tid")
	if !validMicrosoftIssuer(claimString(claims, "iss"), tid, p.tenant) {
		return Identity{}, errors.New("microsoft id token issuer invalid")
	}
	now := time.Now()
	if exp, err := claims.GetExpirationTime(); err != nil || exp == nil || now.After(exp.Time) {
		return Identity{}, errors.New("microsoft id token expired")
	}
	if nbf, err := claims.GetNotBefore(); err == nil && nbf != nil && now.Before(nbf.Time) {
		return Identity{}, errors.New("microsoft id token not active")
	}
	if tokenNonce := claimString(claims, "nonce"); tokenNonce == "" || tokenNonce != nonce {
		return Identity{}, errors.New("microsoft id token nonce invalid")
	}
	subject := claimString(claims, "sub")
	if subject == "" {
		return Identity{}, errors.New("microsoft id token subject missing")
	}
	return Identity{
		Subject:       subject,
		Email:         claimString(claims, "email"),
		EmailVerified: claimBool(claims, "email_verified"),
		Name:          claimString(claims, "name"),
	}, nil
}

func (p microsoftProvider) publicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	return fetchRSAPublicKey(ctx, p.client(), p.jwksURL(), kid, ProviderMicrosoft)
}

func (p microsoftProvider) authorizeURL() string {
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", url.PathEscape(p.tenant))
}

func (p microsoftProvider) tokenURL() string {
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", url.PathEscape(p.tenant))
}

func (p microsoftProvider) jwksURL() string {
	return fmt.Sprintf("https://login.microsoftonline.com/%s/discovery/v2.0/keys", url.PathEscape(p.tenant))
}

func normalizeMicrosoftTenant(tenant string) string {
	tenant = strings.TrimSpace(strings.ToLower(tenant))
	if tenant == "" {
		return defaultMicrosoftTenant
	}
	return tenant
}

func validMicrosoftIssuer(issuer, tenantID, configuredTenant string) bool {
	issuer = strings.TrimSpace(issuer)
	tenantID = strings.TrimSpace(strings.ToLower(tenantID))
	configuredTenant = normalizeMicrosoftTenant(configuredTenant)
	if issuer == "" || tenantID == "" {
		return false
	}
	if issuer != fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", tenantID) {
		return false
	}
	if isMicrosoftTenantAlias(configuredTenant) || !looksLikeGUID(configuredTenant) {
		return true
	}
	return strings.EqualFold(tenantID, configuredTenant)
}

func isMicrosoftTenantAlias(tenant string) bool {
	switch normalizeMicrosoftTenant(tenant) {
	case "common", "organizations", "consumers":
		return true
	default:
		return false
	}
}

func looksLikeGUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
				return false
			}
		}
	}
	return true
}
