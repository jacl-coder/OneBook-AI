package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"onebookai/internal/util"
	"onebookai/services/gateway/internal/authclient"
	"onebookai/services/gateway/internal/oauth"
)

const (
	oauthProviderGoogle       = oauth.ProviderGoogle
	oauthProviderMicrosoft    = oauth.ProviderMicrosoft
	defaultOAuthStatePrefix   = "onebook:auth:oauth"
	defaultOAuthStateTTL      = 10 * time.Minute
	defaultOAuthCallbackError = "OAuth login failed. Please try again."
)

type oauthConfig struct {
	AppBaseURL string
	providers  map[string]oauth.Provider
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
	s.handleOAuthStart(w, r, oauthProviderGoogle)
}

func (s *Server) handleMicrosoftOAuthStart(w http.ResponseWriter, r *http.Request) {
	s.handleOAuthStart(w, r, oauthProviderMicrosoft)
}

func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request, providerName string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	provider, ok := s.oauth.provider(providerName)
	if !ok || !provider.Configured() {
		writeErrorWithCode(w, r, http.StatusServiceUnavailable, providerName+" login is not configured", "AUTH_OAUTH_NOT_CONFIGURED")
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
		Provider:     provider.Name(),
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		ReturnTo:     s.oauthReturnTo(r, "/chat"),
		CreatedAt:    time.Now().UTC(),
	}); err != nil {
		util.LoggerFromContext(r.Context()).Error("gateway.oauth.state_store_error", "err", err)
		writeErrorWithCode(w, r, http.StatusInternalServerError, "internal error", "AUTH_INTERNAL_ERROR")
		return
	}

	authURL, err := provider.AuthCodeURL(state, nonce, codeVerifier)
	if err != nil {
		writeErrorWithCode(w, r, http.StatusInternalServerError, "internal error", "AUTH_INTERNAL_ERROR")
		return
	}

	s.audit(r, "gateway.oauth.start", "success", "provider", provider.Name())
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleGoogleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	s.handleOAuthCallback(w, r, oauthProviderGoogle)
}

func (s *Server) handleMicrosoftOAuthCallback(w http.ResponseWriter, r *http.Request) {
	s.handleOAuthCallback(w, r, oauthProviderMicrosoft)
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request, providerName string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	provider, ok := s.oauth.provider(providerName)
	if !ok || !provider.Configured() {
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
	if err != nil || state.Provider != provider.Name() {
		s.audit(r, "gateway.oauth.callback", "fail", "reason", "invalid_state")
		s.redirectOAuthFailure(w, r, state)
		return
	}

	tokenResp, err := provider.ExchangeCode(ctx, code, state.CodeVerifier)
	if err != nil {
		util.LoggerFromContext(r.Context()).Error("gateway.oauth.token_error", "err", err)
		s.audit(r, "gateway.oauth.callback", "fail", "reason", "token_exchange")
		s.redirectOAuthFailure(w, r, state)
		return
	}
	identity, err := provider.VerifyIDToken(ctx, tokenResp.IDToken, state.Nonce)
	if err != nil {
		util.LoggerFromContext(r.Context()).Error("gateway.oauth.id_token_error", "err", err)
		s.audit(r, "gateway.oauth.callback", "fail", "reason", "id_token")
		s.redirectOAuthFailure(w, r, state)
		return
	}

	user, accessToken, refreshToken, err := s.auth.OAuthComplete(util.RequestIDFromRequest(r), authclient.OAuthCompleteRequest{
		Provider:      provider.Name(),
		Subject:       identity.Subject,
		Email:         identity.Email,
		EmailVerified: identity.EmailVerified,
		DisplayName:   identity.Name,
		AvatarURL:     identity.Picture,
	})
	if err != nil {
		s.audit(r, "gateway.oauth.callback", "fail", "reason", err.Error())
		s.redirectOAuthFailure(w, r, state)
		return
	}
	s.setAccessCookie(w, accessToken)
	s.setRefreshCookie(w, refreshToken)
	s.audit(r, "gateway.oauth.callback", "success", "provider", provider.Name(), "user_id", user.ID)
	http.Redirect(w, r, safeRedirectTarget(state.ReturnTo, "/chat"), http.StatusFound)
}

func newOAuthConfig(appBaseURL string, providers map[string]oauth.Provider) oauthConfig {
	return oauthConfig{
		AppBaseURL: strings.TrimRight(strings.TrimSpace(appBaseURL), "/"),
		providers:  providers,
	}
}

func (c oauthConfig) provider(name string) (oauth.Provider, bool) {
	provider, ok := c.providers[strings.ToLower(strings.TrimSpace(name))]
	return provider, ok
}

func randomURLToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
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
