package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"onebookai/internal/util"
	"onebookai/services/gateway/internal/authclient"
)

const (
	oauthProviderGoogle       = "google"
	googleAuthorizeURL        = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL            = "https://oauth2.googleapis.com/token"
	googleJWKSURL             = "https://www.googleapis.com/oauth2/v3/certs"
	defaultOAuthStatePrefix   = "onebook:auth:oauth"
	defaultOAuthStateTTL      = 10 * time.Minute
	defaultOAuthCallbackError = "OAuth login failed. Please try again."
)

type oauthConfig struct {
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	AppBaseURL         string
}

type oauthState struct {
	Provider     string    `json:"provider"`
	Nonce        string    `json:"nonce"`
	CodeVerifier string    `json:"codeVerifier"`
	ReturnTo     string    `json:"returnTo"`
	CreatedAt    time.Time `json:"createdAt"`
}

type oauthStateStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func newOAuthStateStore(addr, password, prefix string) *oauthStateStore {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = defaultOAuthStatePrefix
	}
	return &oauthStateStore{
		client: redis.NewClient(&redis.Options{Addr: strings.TrimSpace(addr), Password: password}),
		prefix: prefix,
		ttl:    defaultOAuthStateTTL,
	}
}

func (s *oauthStateStore) Save(ctx context.Context, state string, payload oauthState) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.key(state), raw, s.ttl).Err()
}

func (s *oauthStateStore) Consume(ctx context.Context, state string) (oauthState, error) {
	raw, err := s.client.GetDel(ctx, s.key(state)).Bytes()
	if errors.Is(err, redis.Nil) {
		return oauthState{}, errOAuthStateInvalid
	}
	if err != nil {
		return oauthState{}, err
	}
	var payload oauthState
	if err := json.Unmarshal(raw, &payload); err != nil {
		return oauthState{}, err
	}
	return payload, nil
}

func (s *oauthStateStore) key(state string) string {
	return fmt.Sprintf("%s:state:%s", s.prefix, state)
}

var errOAuthStateInvalid = errors.New("oauth state is invalid")

func (s *Server) handleGoogleOAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if !s.oauth.googleConfigured() {
		writeErrorWithCode(w, r, http.StatusServiceUnavailable, "google login is not configured", "AUTH_OAUTH_NOT_CONFIGURED")
		return
	}
	if !s.allowRate(w, r, s.loginLimiter, "too many oauth login attempts", "AUTH_LOGIN_RATE_LIMITED") {
		s.audit(r, "gateway.oauth.start", "rate_limited")
		return
	}

	state, err := randomURLToken(32)
	if err != nil {
		util.LoggerFromContext(r.Context()).Error("gateway.oauth.state_error", "err", err)
		writeErrorWithCode(w, r, http.StatusInternalServerError, "internal error", "AUTH_INTERNAL_ERROR")
		return
	}
	nonce, err := randomURLToken(32)
	if err != nil {
		util.LoggerFromContext(r.Context()).Error("gateway.oauth.nonce_error", "err", err)
		writeErrorWithCode(w, r, http.StatusInternalServerError, "internal error", "AUTH_INTERNAL_ERROR")
		return
	}
	codeVerifier, err := randomURLToken(64)
	if err != nil {
		util.LoggerFromContext(r.Context()).Error("gateway.oauth.pkce_error", "err", err)
		writeErrorWithCode(w, r, http.StatusInternalServerError, "internal error", "AUTH_INTERNAL_ERROR")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.oauthStates.Save(ctx, state, oauthState{
		Provider:     oauthProviderGoogle,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		ReturnTo:     s.oauthReturnTo(r, "/chat"),
		CreatedAt:    time.Now().UTC(),
	}); err != nil {
		util.LoggerFromContext(r.Context()).Error("gateway.oauth.state_store_error", "err", err)
		writeErrorWithCode(w, r, http.StatusInternalServerError, "internal error", "AUTH_INTERNAL_ERROR")
		return
	}

	authURL, err := url.Parse(googleAuthorizeURL)
	if err != nil {
		writeErrorWithCode(w, r, http.StatusInternalServerError, "internal error", "AUTH_INTERNAL_ERROR")
		return
	}
	query := authURL.Query()
	query.Set("client_id", s.oauth.GoogleClientID)
	query.Set("redirect_uri", s.oauth.GoogleRedirectURL)
	query.Set("response_type", "code")
	query.Set("scope", "openid email profile")
	query.Set("state", state)
	query.Set("nonce", nonce)
	query.Set("code_challenge", pkceChallenge(codeVerifier))
	query.Set("code_challenge_method", "S256")
	query.Set("prompt", "select_account")
	authURL.RawQuery = query.Encode()

	s.audit(r, "gateway.oauth.start", "success", "provider", oauthProviderGoogle)
	http.Redirect(w, r, authURL.String(), http.StatusFound)
}

func (s *Server) handleGoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	if !s.oauth.googleConfigured() {
		s.redirectOAuthFailure(w, r, oauthState{})
		return
	}
	if providerErr := strings.TrimSpace(r.URL.Query().Get("error")); providerErr != "" {
		s.audit(r, "gateway.oauth.callback", "fail", "reason", providerErr)
		s.redirectOAuthFailure(w, r, oauthState{})
		return
	}
	stateID := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if stateID == "" || code == "" {
		s.audit(r, "gateway.oauth.callback", "fail", "reason", "missing_state_or_code")
		s.redirectOAuthFailure(w, r, oauthState{})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	state, err := s.oauthStates.Consume(ctx, stateID)
	if err != nil || state.Provider != oauthProviderGoogle {
		s.audit(r, "gateway.oauth.callback", "fail", "reason", "invalid_state")
		s.redirectOAuthFailure(w, r, state)
		return
	}

	tokenResp, err := s.exchangeGoogleCode(ctx, code, state.CodeVerifier)
	if err != nil {
		util.LoggerFromContext(r.Context()).Error("gateway.oauth.token_error", "err", err)
		s.audit(r, "gateway.oauth.callback", "fail", "reason", "token_exchange")
		s.redirectOAuthFailure(w, r, state)
		return
	}
	claims, err := s.verifyGoogleIDToken(ctx, tokenResp.IDToken, state.Nonce)
	if err != nil {
		util.LoggerFromContext(r.Context()).Error("gateway.oauth.id_token_error", "err", err)
		s.audit(r, "gateway.oauth.callback", "fail", "reason", "id_token")
		s.redirectOAuthFailure(w, r, state)
		return
	}

	user, accessToken, refreshToken, err := s.auth.OAuthComplete(util.RequestIDFromRequest(r), authclient.OAuthCompleteRequest{
		Provider:      oauthProviderGoogle,
		Subject:       claims.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		DisplayName:   claims.Name,
		AvatarURL:     claims.Picture,
	})
	if err != nil {
		s.audit(r, "gateway.oauth.callback", "fail", "reason", err.Error())
		s.redirectOAuthFailure(w, r, state)
		return
	}
	s.setAccessCookie(w, accessToken)
	s.setRefreshCookie(w, refreshToken)
	s.audit(r, "gateway.oauth.callback", "success", "provider", oauthProviderGoogle, "user_id", user.ID)
	http.Redirect(w, r, safeRedirectTarget(state.ReturnTo, "/chat"), http.StatusFound)
}

type googleTokenResponse struct {
	IDToken string `json:"id_token"`
}

func (s *Server) exchangeGoogleCode(ctx context.Context, code, codeVerifier string) (googleTokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", s.oauth.GoogleClientID)
	form.Set("client_secret", s.oauth.GoogleClientSecret)
	form.Set("code", code)
	form.Set("code_verifier", codeVerifier)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", s.oauth.GoogleRedirectURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return googleTokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return googleTokenResponse{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return googleTokenResponse{}, fmt.Errorf("google token exchange failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tokenResp googleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return googleTokenResponse{}, err
	}
	if strings.TrimSpace(tokenResp.IDToken) == "" {
		return googleTokenResponse{}, errors.New("google token response missing id_token")
	}
	return tokenResp, nil
}

type googleIDClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

func (s *Server) verifyGoogleIDToken(ctx context.Context, rawToken, nonce string) (googleIDClaims, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("unexpected google signing method %s", token.Method.Alg())
		}
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("google token missing kid")
		}
		return googlePublicKey(ctx, kid)
	})
	if err != nil {
		return googleIDClaims{}, err
	}
	if !token.Valid {
		return googleIDClaims{}, errors.New("google id token invalid")
	}
	if !validGoogleIssuer(claimString(claims, "iss")) {
		return googleIDClaims{}, errors.New("google id token issuer invalid")
	}
	if !claimAudienceContains(claims, s.oauth.GoogleClientID) {
		return googleIDClaims{}, errors.New("google id token audience invalid")
	}
	if exp, err := claims.GetExpirationTime(); err != nil || exp == nil || time.Now().After(exp.Time) {
		return googleIDClaims{}, errors.New("google id token expired")
	}
	if tokenNonce := claimString(claims, "nonce"); tokenNonce == "" || tokenNonce != nonce {
		return googleIDClaims{}, errors.New("google id token nonce invalid")
	}
	subject := claimString(claims, "sub")
	if subject == "" {
		return googleIDClaims{}, errors.New("google id token subject missing")
	}
	return googleIDClaims{
		Subject:       subject,
		Email:         claimString(claims, "email"),
		EmailVerified: claimBool(claims, "email_verified"),
		Name:          claimString(claims, "name"),
		Picture:       claimString(claims, "picture"),
	}, nil
}

type googleJWKS struct {
	Keys []googleJWK `json:"keys"`
}

type googleJWK struct {
	KID string   `json:"kid"`
	KTY string   `json:"kty"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5C []string `json:"x5c"`
}

func googlePublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleJWKSURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google jwks fetch failed: status=%d", resp.StatusCode)
	}
	var jwks googleJWKS
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, err
	}
	for _, key := range jwks.Keys {
		if key.KID != kid {
			continue
		}
		return key.publicKey()
	}
	return nil, errors.New("google jwks kid not found")
}

func (key googleJWK) publicKey() (*rsa.PublicKey, error) {
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
			return nil, errors.New("google cert public key is not rsa")
		}
		return pub, nil
	}
	if strings.TrimSpace(key.KTY) != "" && !strings.EqualFold(key.KTY, "RSA") {
		return nil, fmt.Errorf("google jwk key type %q is not rsa", key.KTY)
	}
	if strings.TrimSpace(key.N) == "" || strings.TrimSpace(key.E) == "" {
		return nil, errors.New("google jwk missing rsa key material")
	}
	modulus, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("decode google jwk modulus: %w", err)
	}
	exponent, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("decode google jwk exponent: %w", err)
	}
	e := new(big.Int).SetBytes(exponent)
	if !e.IsInt64() {
		return nil, errors.New("google jwk exponent is too large")
	}
	eInt := int(e.Int64())
	if eInt <= 1 {
		return nil, errors.New("google jwk exponent is invalid")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulus),
		E: eInt,
	}, nil
}

func (c oauthConfig) googleConfigured() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != "" && c.GoogleRedirectURL != ""
}

func randomURLToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (s *Server) redirectOAuthFailure(w http.ResponseWriter, r *http.Request, state oauthState) {
	target := s.oauthFailureTarget(state)
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Server) oauthFailureTarget(state oauthState) string {
	base := s.oauth.absoluteAppURL("/log-in/error")
	if state.ReturnTo != "" {
		if parsed, err := url.Parse(state.ReturnTo); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			base = parsed.Scheme + "://" + parsed.Host + "/log-in/error"
		}
	}
	u, err := url.Parse(base)
	if err != nil {
		return "/log-in/error"
	}
	query := u.Query()
	query.Set("message", defaultOAuthCallbackError)
	u.RawQuery = query.Encode()
	return u.String()
}

func safeRedirectTarget(raw, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fallback
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fallback
		}
		return parsed.String()
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	return fallback
}

func (s *Server) oauthReturnTo(r *http.Request, fallbackPath string) string {
	queryReturn := strings.TrimSpace(r.URL.Query().Get("returnTo"))
	if queryReturn != "" && strings.HasPrefix(queryReturn, "/") && !strings.HasPrefix(queryReturn, "//") {
		return s.oauth.absoluteAppURL(queryReturn)
	}
	return s.oauth.absoluteAppURL(fallbackPath)
}

func (c oauthConfig) absoluteAppURL(path string) string {
	if strings.TrimSpace(path) == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if c.AppBaseURL == "" {
		return path
	}
	return c.AppBaseURL + path
}

func validGoogleIssuer(issuer string) bool {
	return issuer == "https://accounts.google.com" || issuer == "accounts.google.com"
}

func claimString(claims jwt.MapClaims, key string) string {
	value, _ := claims[key].(string)
	return strings.TrimSpace(value)
}

func claimBool(claims jwt.MapClaims, key string) bool {
	switch value := claims[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(value, "true")
	default:
		return false
	}
}

func claimAudienceContains(claims jwt.MapClaims, audience string) bool {
	switch value := claims["aud"].(type) {
	case string:
		return value == audience
	case []string:
		for _, item := range value {
			if item == audience {
				return true
			}
		}
	case []any:
		for _, item := range value {
			if str, ok := item.(string); ok && str == audience {
				return true
			}
		}
	}
	return false
}
