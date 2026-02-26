package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
	"onebookai/internal/ratelimit"
	"onebookai/internal/usertoken"
	"onebookai/internal/util"
	"onebookai/pkg/domain"
	"onebookai/services/gateway/internal/authclient"
	"onebookai/services/gateway/internal/bookclient"
	"onebookai/services/gateway/internal/chatclient"
)

type contextKey string

const requestIDContextKey contextKey = "request_id"

const (
	defaultAccessCookieName    = "onebook_access"
	defaultAccessCookiePath    = "/"
	defaultAccessCookieMaxAge  = 15 * time.Minute
	defaultRefreshCookieName   = "onebook_refresh"
	defaultRefreshCookiePath   = "/api/auth"
	defaultRefreshCookieMaxAge = 30 * 24 * time.Hour
)

var errSessionUnauthorized = errors.New("session unauthorized")

// Config wires required dependencies for the HTTP server.
type Config struct {
	Auth                       *authclient.Client
	Book                       *bookclient.Client
	Chat                       *chatclient.Client
	TokenVerifier              *usertoken.Verifier
	AccessCookieName           string
	AccessCookieDomain         string
	AccessCookiePath           string
	AccessCookieSecure         bool
	AccessCookieSameSite       string
	AccessCookieMaxAge         time.Duration
	RefreshCookieName          string
	RefreshCookieDomain        string
	RefreshCookiePath          string
	RefreshCookieSecure        bool
	RefreshCookieSameSite      string
	RefreshCookieMaxAge        time.Duration
	RedisAddr                  string
	RedisPassword              string
	TrustedProxyCIDRs          []string
	SignupRateLimitPerMinute   int
	LoginRateLimitPerMinute    int
	RefreshRateLimitPerMinute  int
	PasswordRateLimitPerMinute int
	MaxUploadBytes             int64
	AllowedExtensions          []string
}

// Server exposes HTTP endpoints for the backend.
type Server struct {
	auth              *authclient.Client
	books             *bookclient.Client
	chat              *chatclient.Client
	tokenVerifier     *usertoken.Verifier
	accessCookieName  string
	accessCookieCfg   http.Cookie
	refreshCookieName string
	refreshCookieCfg  http.Cookie
	refreshSingle     singleflight.Group
	mux               *http.ServeMux
	maxUploadBytes    int64
	allowedExtensions map[string]struct{}
	signupLimiter     *ratelimit.FixedWindowLimiter
	loginLimiter      *ratelimit.FixedWindowLimiter
	refreshLimiter    *ratelimit.FixedWindowLimiter
	passwordLimiter   *ratelimit.FixedWindowLimiter
	trustedProxies    *util.TrustedProxies
}

// New constructs the server with routes configured.
func New(cfg Config) (*Server, error) {
	signupLimit := cfg.SignupRateLimitPerMinute
	if signupLimit <= 0 {
		signupLimit = 5
	}
	loginLimit := cfg.LoginRateLimitPerMinute
	if loginLimit <= 0 {
		loginLimit = 10
	}
	refreshLimit := cfg.RefreshRateLimitPerMinute
	if refreshLimit <= 0 {
		refreshLimit = 20
	}
	passwordLimit := cfg.PasswordRateLimitPerMinute
	if passwordLimit <= 0 {
		passwordLimit = 10
	}
	rateWindow := time.Minute
	newLimiter := func(name string, limit int) (*ratelimit.FixedWindowLimiter, error) {
		prefix := "onebook:gateway:ratelimit:" + name
		limiter, err := ratelimit.NewRedisFixedWindowLimiter(cfg.RedisAddr, cfg.RedisPassword, prefix, limit, rateWindow)
		if err != nil {
			return nil, fmt.Errorf("init %s limiter: %w", name, err)
		}
		return limiter, nil
	}
	signupLimiter, err := newLimiter("signup", signupLimit)
	if err != nil {
		return nil, err
	}
	loginLimiter, err := newLimiter("login", loginLimit)
	if err != nil {
		return nil, err
	}
	refreshLimiter, err := newLimiter("refresh", refreshLimit)
	if err != nil {
		return nil, err
	}
	passwordLimiter, err := newLimiter("password", passwordLimit)
	if err != nil {
		return nil, err
	}
	trustedProxies, err := util.NewTrustedProxies(cfg.TrustedProxyCIDRs)
	if err != nil {
		return nil, fmt.Errorf("parse trustedProxyCIDRs: %w", err)
	}
	s := &Server{
		auth:             cfg.Auth,
		books:            cfg.Book,
		chat:             cfg.Chat,
		tokenVerifier:    cfg.TokenVerifier,
		accessCookieName: normalizeAccessCookieName(cfg.AccessCookieName),
		accessCookieCfg: http.Cookie{
			Name:     normalizeAccessCookieName(cfg.AccessCookieName),
			Path:     normalizeAccessCookiePath(cfg.AccessCookiePath),
			Domain:   strings.TrimSpace(cfg.AccessCookieDomain),
			MaxAge:   int(normalizeAccessCookieMaxAge(cfg.AccessCookieMaxAge).Seconds()),
			HttpOnly: true,
			Secure:   cfg.AccessCookieSecure,
			SameSite: parseSameSiteMode(cfg.AccessCookieSameSite),
		},
		refreshCookieName: normalizeRefreshCookieName(cfg.RefreshCookieName),
		refreshCookieCfg: http.Cookie{
			Name:     normalizeRefreshCookieName(cfg.RefreshCookieName),
			Path:     normalizeRefreshCookiePath(cfg.RefreshCookiePath),
			Domain:   strings.TrimSpace(cfg.RefreshCookieDomain),
			MaxAge:   int(normalizeRefreshCookieMaxAge(cfg.RefreshCookieMaxAge).Seconds()),
			HttpOnly: true,
			Secure:   cfg.RefreshCookieSecure,
			SameSite: parseSameSiteMode(cfg.RefreshCookieSameSite),
		},
		mux:               http.NewServeMux(),
		maxUploadBytes:    normalizeMaxBytes(cfg.MaxUploadBytes),
		allowedExtensions: normalizeExtensions(cfg.AllowedExtensions),
		signupLimiter:     signupLimiter,
		loginLimiter:      loginLimiter,
		refreshLimiter:    refreshLimiter,
		passwordLimiter:   passwordLimiter,
		trustedProxies:    trustedProxies,
	}
	s.routes()
	return s, nil
}

// Router returns the configured handler.
func (s *Server) Router() http.Handler {
	handler := util.WithSecurityHeaders(util.WithCORS(s.mux))
	return withRequestID(handler)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)

	// auth
	s.mux.HandleFunc("/api/auth/signup", s.handleSignup)
	s.mux.HandleFunc("/api/auth/login", s.handleLogin)
	s.mux.HandleFunc("/api/auth/login/methods", s.handleLoginMethods)
	s.mux.HandleFunc("/api/auth/otp/send", s.handleOTPSend)
	s.mux.HandleFunc("/api/auth/otp/verify", s.handleOTPVerify)
	s.mux.HandleFunc("/api/auth/password/reset/verify", s.handlePasswordResetVerify)
	s.mux.HandleFunc("/api/auth/password/reset/complete", s.handlePasswordResetComplete)
	s.mux.HandleFunc("/api/auth/refresh", s.handleRefresh)
	s.mux.HandleFunc("/api/auth/logout", s.handleLogout)
	s.mux.HandleFunc("/api/auth/jwks", s.handleJWKS)
	s.mux.HandleFunc("/.well-known/jwks.json", s.handleJWKS)
	s.mux.Handle("/api/users/me", s.authenticated(s.handleMe))
	s.mux.Handle("/api/users/me/password", s.authenticated(s.handleChangePassword))

	// books & chats (auth required)
	s.mux.Handle("/api/books", s.authenticated(s.handleBooks))
	s.mux.Handle("/api/books/", s.authenticated(s.handleBookByID))
	s.mux.Handle("/api/chats", s.authenticated(s.handleChats))
	s.mux.Handle("/api/conversations", s.authenticated(s.handleConversations))
	s.mux.Handle("/api/conversations/", s.authenticated(s.handleConversationByID))

	// admin
	s.mux.Handle("/api/admin/users", s.adminOnly(s.handleAdminUsers))
	s.mux.Handle("/api/admin/users/", s.adminOnly(s.handleAdminUserByID))
	s.mux.Handle("/api/admin/books", s.adminOnly(s.handleAdminBooks))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// auth wrappers
type authContext struct {
	User         domain.User
	AccessToken  string
	RefreshToken string
}

type authHandler func(http.ResponseWriter, *http.Request, authContext)

func (s *Server) authenticated(next authHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.resolveSession(w, r)
		if err != nil {
			s.audit(r, "gateway.authorize", "fail")
			if errors.Is(err, errSessionUnauthorized) {
				s.clearAuthCookies(w)
				writeErrorWithCode(w, r, http.StatusUnauthorized, "authentication required", "AUTH_INVALID_TOKEN")
				return
			}
			writeAuthError(w, r, err)
			return
		}
		s.audit(r, "gateway.authorize", "success", "user_id", ctx.User.ID)
		next(w, r, ctx)
	})
}

func (s *Server) adminOnly(next authHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := s.resolveSession(w, r)
		if err != nil {
			s.audit(r, "gateway.admin.authorize", "fail", "reason", err.Error())
			if errors.Is(err, errSessionUnauthorized) {
				s.clearAuthCookies(w)
				writeErrorWithCode(w, r, http.StatusUnauthorized, "authentication required", "AUTH_INVALID_TOKEN")
				return
			}
			writeAuthError(w, r, err)
			return
		}
		if ctx.User.Role != domain.RoleAdmin {
			s.audit(r, "gateway.admin.authorize", "fail", "user_id", ctx.User.ID, "reason", "forbidden")
			writeErrorWithCode(w, r, http.StatusForbidden, "you do not have permission", "ADMIN_FORBIDDEN")
			return
		}
		s.audit(r, "gateway.admin.authorize", "success", "user_id", ctx.User.ID)
		next(w, r, ctx)
	})
}

func (s *Server) resolveSession(w http.ResponseWriter, r *http.Request) (authContext, error) {
	accessToken := tokenFromCookie(r, s.accessCookieName)
	refreshToken := tokenFromCookie(r, s.refreshCookieName)
	if accessToken != "" {
		user, err := s.authorizeAccessToken(r, accessToken)
		if err == nil {
			return authContext{User: user, AccessToken: accessToken, RefreshToken: refreshToken}, nil
		}
		if !errors.Is(err, errSessionUnauthorized) {
			return authContext{}, err
		}
	}
	refreshed, err := s.refreshViaCookie(w, r, refreshToken)
	if err != nil {
		return authContext{}, err
	}
	return refreshed, nil
}

func (s *Server) authorizeAccessToken(r *http.Request, accessToken string) (domain.User, error) {
	token := strings.TrimSpace(accessToken)
	if token == "" {
		s.audit(r, "gateway.token.verify", "fail", "reason", "missing_access_cookie")
		return domain.User{}, errSessionUnauthorized
	}
	if s.tokenVerifier != nil {
		if _, err := s.tokenVerifier.VerifySubject(token); err != nil {
			s.audit(r, "gateway.token.verify", "fail", "reason", "invalid_signature_or_claims")
			return domain.User{}, errSessionUnauthorized
		}
	}
	user, err := s.auth.Me(requestIDFromRequest(r), token)
	if err != nil {
		if isUnauthorizedAuthError(err) {
			s.audit(r, "gateway.token.verify", "fail", "reason", "auth_me_unauthorized")
			return domain.User{}, errSessionUnauthorized
		}
		return domain.User{}, err
	}
	s.audit(r, "gateway.token.verify", "success", "user_id", user.ID)
	return user, nil
}

func (s *Server) refreshViaCookie(w http.ResponseWriter, r *http.Request, refreshToken string) (authContext, error) {
	token := strings.TrimSpace(refreshToken)
	if token == "" {
		s.audit(r, "gateway.refresh", "fail", "reason", "missing_refresh_cookie")
		return authContext{}, errSessionUnauthorized
	}
	result, err, _ := s.refreshSingle.Do(token, func() (any, error) {
		user, accessToken, newRefreshToken, err := s.auth.Refresh(requestIDFromRequest(r), token)
		if err != nil {
			return nil, err
		}
		return authContext{
			User:         user,
			AccessToken:  accessToken,
			RefreshToken: newRefreshToken,
		}, nil
	})
	if err != nil {
		if isUnauthorizedAuthError(err) {
			s.audit(r, "gateway.refresh", "fail", "reason", "invalid_refresh_cookie")
			return authContext{}, errSessionUnauthorized
		}
		return authContext{}, err
	}
	ctx := result.(authContext)
	s.setAccessCookie(w, ctx.AccessToken)
	s.setRefreshCookie(w, ctx.RefreshToken)
	return ctx, nil
}

// auth handlers
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if !s.allowRate(w, r, s.signupLimiter, "too many signup attempts", "AUTH_SIGNUP_RATE_LIMITED") {
		s.audit(r, "gateway.signup", "rate_limited")
		return
	}
	var req authRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "gateway.signup", "fail", "reason", "invalid_json")
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid request payload", "AUTH_INVALID_REQUEST")
		return
	}
	user, accessToken, refreshToken, err := s.auth.SignUp(requestIDFromRequest(r), req.Email, req.Password)
	if err != nil {
		s.audit(r, "gateway.signup", "fail", "reason", err.Error())
		writeAuthError(w, r, err)
		return
	}
	s.audit(r, "gateway.signup", "success", "user_id", user.ID)
	s.setAccessCookie(w, accessToken)
	s.setRefreshCookie(w, refreshToken)
	writeJSON(w, http.StatusCreated, authResponse{
		User: user,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if !s.allowRate(w, r, s.loginLimiter, "too many login attempts", "AUTH_LOGIN_RATE_LIMITED") {
		s.audit(r, "gateway.login", "rate_limited")
		return
	}
	var req authRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "gateway.login", "fail", "reason", "invalid_json")
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid request payload", "AUTH_INVALID_REQUEST")
		return
	}
	user, accessToken, refreshToken, err := s.auth.Login(requestIDFromRequest(r), req.Email, req.Password)
	if err != nil {
		s.audit(r, "gateway.login", "fail", "reason", err.Error())
		writeAuthError(w, r, err)
		return
	}
	s.audit(r, "gateway.login", "success", "user_id", user.ID)
	s.setAccessCookie(w, accessToken)
	s.setRefreshCookie(w, refreshToken)
	writeJSON(w, http.StatusOK, authResponse{
		User: user,
	})
}

func (s *Server) handleLoginMethods(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if !s.allowRate(w, r, s.loginLimiter, "too many login method checks", "AUTH_LOGIN_METHOD_RATE_LIMITED") {
		s.audit(r, "gateway.login.methods", "rate_limited")
		return
	}
	var req loginMethodsRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "gateway.login.methods", "fail", "reason", "invalid_json")
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid request payload", "AUTH_INVALID_REQUEST")
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		s.audit(r, "gateway.login.methods", "fail", "reason", "missing_email")
		writeErrorWithCode(w, r, http.StatusBadRequest, "email is required", "AUTH_EMAIL_REQUIRED")
		return
	}
	passwordLogin, err := s.auth.LoginMethods(requestIDFromRequest(r), req.Email)
	if err != nil {
		s.audit(r, "gateway.login.methods", "fail", "reason", err.Error())
		writeAuthError(w, r, err)
		return
	}
	s.audit(r, "gateway.login.methods", "success")
	writeJSON(w, http.StatusOK, loginMethodsResponse{PasswordLogin: passwordLogin})
}

func (s *Server) handleOTPSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if !s.allowRate(w, r, s.signupLimiter, "too many verification code requests", "AUTH_OTP_SEND_RATE_LIMITED") {
		s.audit(r, "gateway.otp.send", "rate_limited")
		return
	}
	var req otpSendRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "gateway.otp.send", "fail", "reason", "invalid_json")
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid request payload", "AUTH_INVALID_REQUEST")
		return
	}
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Purpose) == "" {
		s.audit(r, "gateway.otp.send", "fail", "reason", "missing_fields")
		writeErrorWithCode(w, r, http.StatusBadRequest, "email and verification purpose are required", "AUTH_INVALID_REQUEST")
		return
	}
	resp, err := s.auth.OTPSend(requestIDFromRequest(r), req.Email, req.Purpose)
	if err != nil {
		s.audit(r, "gateway.otp.send", "fail", "reason", err.Error())
		writeAuthError(w, r, err)
		return
	}
	s.audit(r, "gateway.otp.send", "success")
	writeJSON(w, http.StatusAccepted, resp)
}

func (s *Server) handleOTPVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if !s.allowRate(w, r, s.loginLimiter, "too many verification attempts", "AUTH_OTP_VERIFY_RATE_LIMITED") {
		s.audit(r, "gateway.otp.verify", "rate_limited")
		return
	}
	var req otpVerifyRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "gateway.otp.verify", "fail", "reason", "invalid_json")
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid request payload", "AUTH_INVALID_REQUEST")
		return
	}
	if strings.TrimSpace(req.ChallengeID) == "" || strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Purpose) == "" || strings.TrimSpace(req.Code) == "" {
		s.audit(r, "gateway.otp.verify", "fail", "reason", "missing_fields")
		writeErrorWithCode(w, r, http.StatusBadRequest, "verification request is incomplete", "AUTH_INVALID_REQUEST")
		return
	}
	user, accessToken, refreshToken, err := s.auth.OTPVerify(
		requestIDFromRequest(r),
		req.ChallengeID,
		req.Email,
		req.Purpose,
		req.Code,
		req.Password,
	)
	if err != nil {
		s.audit(r, "gateway.otp.verify", "fail", "reason", err.Error())
		writeAuthError(w, r, err)
		return
	}
	s.audit(r, "gateway.otp.verify", "success", "user_id", user.ID, "purpose", req.Purpose)
	s.setAccessCookie(w, accessToken)
	s.setRefreshCookie(w, refreshToken)
	writeJSON(w, http.StatusOK, authResponse{
		User: user,
	})
}

func (s *Server) handlePasswordResetVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if !s.allowRate(w, r, s.loginLimiter, "too many password reset verification attempts", "AUTH_PASSWORD_RESET_VERIFY_RATE_LIMITED") {
		s.audit(r, "gateway.password.reset.verify", "rate_limited")
		return
	}
	var req passwordResetVerifyRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "gateway.password.reset.verify", "fail", "reason", "invalid_json")
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid request payload", "AUTH_INVALID_REQUEST")
		return
	}
	if strings.TrimSpace(req.ChallengeID) == "" || strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Code) == "" {
		s.audit(r, "gateway.password.reset.verify", "fail", "reason", "missing_fields")
		writeErrorWithCode(w, r, http.StatusBadRequest, "verification request is incomplete", "AUTH_INVALID_REQUEST")
		return
	}
	resp, err := s.auth.PasswordResetVerify(requestIDFromRequest(r), req.ChallengeID, req.Email, req.Code)
	if err != nil {
		s.audit(r, "gateway.password.reset.verify", "fail", "reason", err.Error())
		writeAuthError(w, r, err)
		return
	}
	s.audit(r, "gateway.password.reset.verify", "success")
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePasswordResetComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if !s.allowRate(w, r, s.passwordLimiter, "too many password reset attempts", "AUTH_PASSWORD_RESET_RATE_LIMITED") {
		s.audit(r, "gateway.password.reset.complete", "rate_limited")
		return
	}
	var req passwordResetCompleteRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "gateway.password.reset.complete", "fail", "reason", "invalid_json")
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid request payload", "AUTH_INVALID_REQUEST")
		return
	}
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.ResetToken) == "" || strings.TrimSpace(req.NewPassword) == "" {
		s.audit(r, "gateway.password.reset.complete", "fail", "reason", "missing_fields")
		writeErrorWithCode(w, r, http.StatusBadRequest, "email, reset token, and new password are required", "AUTH_INVALID_REQUEST")
		return
	}
	if err := s.auth.PasswordResetComplete(requestIDFromRequest(r), req.Email, req.ResetToken, req.NewPassword); err != nil {
		s.audit(r, "gateway.password.reset.complete", "fail", "reason", err.Error())
		writeAuthError(w, r, err)
		return
	}
	s.audit(r, "gateway.password.reset.complete", "success")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if !s.allowRate(w, r, s.refreshLimiter, "too many refresh attempts", "AUTH_REFRESH_RATE_LIMITED") {
		s.audit(r, "gateway.refresh", "rate_limited")
		return
	}
	ctx, err := s.refreshViaCookie(w, r, tokenFromCookie(r, s.refreshCookieName))
	if err != nil {
		if errors.Is(err, errSessionUnauthorized) {
			s.clearAuthCookies(w)
			writeErrorWithCode(w, r, http.StatusUnauthorized, "authentication required", "AUTH_INVALID_REFRESH_TOKEN")
			return
		}
		writeAuthError(w, r, err)
		return
	}
	s.audit(r, "gateway.refresh", "success", "user_id", ctx.User.ID)
	writeJSON(w, http.StatusOK, authResponse{
		User: ctx.User,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if !s.allowRate(w, r, s.refreshLimiter, "too many logout attempts", "AUTH_REFRESH_RATE_LIMITED") {
		s.audit(r, "gateway.logout", "rate_limited")
		return
	}
	defer s.clearAuthCookies(w)
	accessToken := tokenFromCookie(r, s.accessCookieName)
	refreshToken := tokenFromCookie(r, s.refreshCookieName)
	if strings.TrimSpace(accessToken) == "" && strings.TrimSpace(refreshToken) == "" {
		s.audit(r, "gateway.logout", "success", "reason", "no_active_session")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := s.auth.Logout(requestIDFromRequest(r), accessToken, refreshToken); err != nil {
		s.audit(r, "gateway.logout", "fail", "reason", err.Error())
		writeAuthError(w, r, err)
		return
	}
	s.audit(r, "gateway.logout", "success")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	keys, err := s.auth.JWKS(requestIDFromRequest(r))
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, ctx authContext) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, ctx.User)
	case http.MethodPatch:
		var req updateMeRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeErrorWithCode(w, r, http.StatusBadRequest, "invalid request payload", "AUTH_INVALID_REQUEST")
			return
		}
		if req.Email == "" {
			writeErrorWithCode(w, r, http.StatusBadRequest, "email is required", "AUTH_EMAIL_REQUIRED")
			return
		}
		updated, err := s.auth.UpdateMe(requestIDFromRequest(r), ctx.AccessToken, req.Email)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		methodNotAllowed(w, r)
	}
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, ctx authContext) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	if !s.allowRate(w, r, s.passwordLimiter, "too many password change attempts", "AUTH_PASSWORD_CHANGE_RATE_LIMITED") {
		s.audit(r, "gateway.password.change", "rate_limited")
		return
	}
	var req changePasswordRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid request payload", "AUTH_INVALID_REQUEST")
		return
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		writeErrorWithCode(w, r, http.StatusBadRequest, "new password is required", "AUTH_NEW_PASSWORD_REQUIRED")
		return
	}
	if err := s.auth.ChangePassword(requestIDFromRequest(r), ctx.AccessToken, req.CurrentPassword, req.NewPassword); err != nil {
		writeAuthError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// /api/books
func (s *Server) handleBooks(w http.ResponseWriter, r *http.Request, ctx authContext) {
	switch r.Method {
	case http.MethodPost:
		s.handleUploadBook(w, r, ctx.AccessToken)
	case http.MethodGet:
		s.handleListBooks(w, r, ctx.AccessToken)
	default:
		methodNotAllowed(w, r)
	}
}

// /api/books/{id} or /api/books/{id}/download
func (s *Server) handleBookByID(w http.ResponseWriter, r *http.Request, ctx authContext) {
	path := strings.TrimPrefix(r.URL.Path, "/api/books/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	if id == "" {
		writeErrorWithCode(w, r, http.StatusNotFound, "not found", "BOOK_NOT_FOUND")
		return
	}

	// Handle /api/books/{id}/download
	if len(parts) == 2 && parts[1] == "download" {
		s.handleDownloadBook(w, r, ctx.AccessToken, id)
		return
	}
	if len(parts) == 2 {
		writeErrorWithCode(w, r, http.StatusNotFound, "not found", "BOOK_NOT_FOUND")
		return
	}

	switch r.Method {
	case http.MethodGet:
		book, err := s.books.GetBook(requestIDFromRequest(r), ctx.AccessToken, id)
		if err != nil {
			writeBookError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, book)
	case http.MethodDelete:
		if err := s.books.DeleteBook(requestIDFromRequest(r), ctx.AccessToken, id); err != nil {
			writeBookError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		methodNotAllowed(w, r)
	}
}

// handleDownloadBook returns a pre-signed download URL for the book file.
func (s *Server) handleDownloadBook(w http.ResponseWriter, r *http.Request, token, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	resp, err := s.books.GetDownloadURL(requestIDFromRequest(r), token, id)
	if err != nil {
		writeBookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUploadBook(w http.ResponseWriter, r *http.Request, token string) {
	if s.maxUploadBytes > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, s.maxUploadBytes)
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid form data", "BOOK_INVALID_UPLOAD_FORM")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeErrorWithCode(w, r, http.StatusBadRequest, "file is required (field: file)", "BOOK_FILE_REQUIRED")
		return
	}
	defer file.Close()
	if !s.isExtensionAllowed(header.Filename) {
		writeErrorWithCode(w, r, http.StatusBadRequest, "unsupported file type", "BOOK_UNSUPPORTED_FILE_TYPE")
		return
	}
	book, err := s.books.UploadBook(requestIDFromRequest(r), token, header.Filename, file)
	if err != nil {
		writeBookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, book)
}

func (s *Server) handleListBooks(w http.ResponseWriter, r *http.Request, token string) {
	books, err := s.books.ListBooks(requestIDFromRequest(r), token)
	if err != nil {
		writeBookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": books,
		"count": len(books),
	})
}

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request, ctx authContext) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	var req chatRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid request payload", "CHAT_INVALID_REQUEST")
		return
	}
	if req.BookID == "" {
		writeErrorWithCode(w, r, http.StatusBadRequest, "book ID is required", "CHAT_BOOK_ID_REQUIRED")
		return
	}
	ans, err := s.chat.AskQuestion(
		requestIDFromRequest(r),
		ctx.AccessToken,
		req.ConversationID,
		req.BookID,
		req.Question,
	)
	if err != nil {
		writeChatError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ans)
}

func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request, ctx authContext) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	limit := parsePositiveIntWithMax(r.URL.Query().Get("limit"), 30, 100)
	items, err := s.chat.ListConversations(requestIDFromRequest(r), ctx.AccessToken, limit)
	if err != nil {
		writeChatError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"count": len(items),
	})
}

func (s *Server) handleConversationByID(w http.ResponseWriter, r *http.Request, ctx authContext) {
	path := strings.TrimPrefix(r.URL.Path, "/api/conversations/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeErrorWithCode(w, r, http.StatusNotFound, "not found", "CHAT_CONVERSATION_NOT_FOUND")
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "messages" {
		writeErrorWithCode(w, r, http.StatusNotFound, "not found", "CHAT_CONVERSATION_NOT_FOUND")
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	conversationID := strings.TrimSpace(parts[0])
	if conversationID == "" {
		writeErrorWithCode(w, r, http.StatusBadRequest, "conversation ID is required", "CHAT_CONVERSATION_ID_REQUIRED")
		return
	}
	limit := parsePositiveIntWithMax(r.URL.Query().Get("limit"), 200, 500)
	items, err := s.chat.ListConversationMessages(requestIDFromRequest(r), ctx.AccessToken, conversationID, limit)
	if err != nil {
		writeChatError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"count": len(items),
	})
}

// admin handlers
func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request, ctx authContext) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	users, err := s.auth.AdminListUsers(requestIDFromRequest(r), ctx.AccessToken)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": users,
		"count": len(users),
	})
}

func (s *Server) handleAdminUserByID(w http.ResponseWriter, r *http.Request, ctx authContext) {
	id := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	if id == "" || strings.Contains(id, "/") {
		writeErrorWithCode(w, r, http.StatusNotFound, "not found", "ADMIN_NOT_FOUND")
		return
	}
	if r.Method != http.MethodPatch {
		methodNotAllowed(w, r)
		return
	}
	var req adminUserUpdateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid request payload", "ADMIN_INVALID_REQUEST")
		return
	}
	var role *domain.UserRole
	if req.Role != "" {
		parsed, ok := parseUserRole(req.Role)
		if !ok {
			writeErrorWithCode(w, r, http.StatusBadRequest, "invalid role", "ADMIN_INVALID_ROLE")
			return
		}
		role = &parsed
	}
	var status *domain.UserStatus
	if req.Status != "" {
		parsed, ok := parseUserStatus(req.Status)
		if !ok {
			writeErrorWithCode(w, r, http.StatusBadRequest, "invalid status", "ADMIN_INVALID_STATUS")
			return
		}
		status = &parsed
	}
	if role == nil && status == nil {
		writeErrorWithCode(w, r, http.StatusBadRequest, "role or status is required", "ADMIN_UPDATE_FIELDS_REQUIRED")
		return
	}
	updated, err := s.auth.AdminUpdateUser(requestIDFromRequest(r), ctx.AccessToken, id, role, status)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleAdminBooks(w http.ResponseWriter, r *http.Request, ctx authContext) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	books, err := s.books.ListBooks(requestIDFromRequest(r), ctx.AccessToken)
	if err != nil {
		writeBookError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": books,
		"count": len(books),
	})
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeErrorWithCode(w, r, http.StatusMethodNotAllowed, "method not allowed", "SYSTEM_METHOD_NOT_ALLOWED")
}

func parsePositiveIntWithMax(raw string, fallback int, max int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	if n > max {
		return max
	}
	return n
}

type chatRequest struct {
	ConversationID string `json:"conversationId,omitempty"`
	BookID         string `json:"bookId"`
	Question       string `json:"question"`
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginMethodsRequest struct {
	Email string `json:"email"`
}

type loginMethodsResponse struct {
	PasswordLogin bool `json:"passwordLogin"`
}

type otpSendRequest struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"`
}

type otpVerifyRequest struct {
	ChallengeID string `json:"challengeId"`
	Email       string `json:"email"`
	Purpose     string `json:"purpose"`
	Code        string `json:"code"`
	Password    string `json:"password,omitempty"`
}

type passwordResetVerifyRequest struct {
	ChallengeID string `json:"challengeId"`
	Email       string `json:"email"`
	Code        string `json:"code"`
}

type passwordResetCompleteRequest struct {
	Email       string `json:"email"`
	ResetToken  string `json:"resetToken"`
	NewPassword string `json:"newPassword"`
}

type authResponse struct {
	User domain.User `json:"user"`
}

type updateMeRequest struct {
	Email string `json:"email"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type adminUserUpdateRequest struct {
	Role   string `json:"role"`
	Status string `json:"status"`
}

func tokenFromCookie(r *http.Request, name string) string {
	if r == nil || strings.TrimSpace(name) == "" {
		return ""
	}
	cookie, err := r.Cookie(name)
	if err != nil || cookie == nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func (s *Server) setAccessCookie(w http.ResponseWriter, accessToken string) {
	token := strings.TrimSpace(accessToken)
	if token == "" {
		return
	}
	cookie := s.accessCookieCfg
	cookie.Value = token
	http.SetCookie(w, &cookie)
}

func (s *Server) setRefreshCookie(w http.ResponseWriter, refreshToken string) {
	token := strings.TrimSpace(refreshToken)
	if token == "" {
		return
	}
	cookie := s.refreshCookieCfg
	cookie.Value = token
	http.SetCookie(w, &cookie)
}

func (s *Server) clearAccessCookie(w http.ResponseWriter) {
	cookie := s.accessCookieCfg
	cookie.Value = ""
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(0, 0)
	http.SetCookie(w, &cookie)
}

func (s *Server) clearRefreshCookie(w http.ResponseWriter) {
	cookie := s.refreshCookieCfg
	cookie.Value = ""
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(0, 0)
	http.SetCookie(w, &cookie)
}

func (s *Server) clearAuthCookies(w http.ResponseWriter) {
	s.clearAccessCookie(w)
	s.clearRefreshCookie(w)
}

func parseUserRole(role string) (domain.UserRole, bool) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case string(domain.RoleUser):
		return domain.RoleUser, true
	case string(domain.RoleAdmin):
		return domain.RoleAdmin, true
	default:
		return "", false
	}
}

func parseUserStatus(status string) (domain.UserStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case string(domain.StatusActive):
		return domain.StatusActive, true
	case string(domain.StatusDisabled):
		return domain.StatusDisabled, true
	default:
		return "", false
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type errorDetail struct {
	Field  string `json:"field,omitempty"`
	Reason string `json:"reason"`
}

type errorResponse struct {
	Error     string        `json:"error"`
	Code      string        `json:"code"`
	RequestID string        `json:"requestId,omitempty"`
	Details   []errorDetail `json:"details,omitempty"`
}

func writeErrorWithCode(w http.ResponseWriter, r *http.Request, status int, msg, code string) {
	code = strings.TrimSpace(code)
	if code == "" {
		code = errorCodeForStatus(status)
	}
	if code == "" {
		code = "REQUEST_ERROR"
	}
	writeJSON(w, status, errorResponse{
		Error:     msg,
		Code:      code,
		RequestID: requestIDFromRequest(r),
	})
}

func errorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "REQUEST_ERROR"
	case http.StatusUnauthorized:
		return "AUTH_INVALID_TOKEN"
	case http.StatusForbidden:
		return "REQUEST_ERROR"
	case http.StatusNotFound:
		return "SYSTEM_NOT_FOUND"
	case http.StatusMethodNotAllowed:
		return "SYSTEM_METHOD_NOT_ALLOWED"
	case http.StatusTooManyRequests:
		return "SYSTEM_RATE_LIMITED"
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		return "SYSTEM_UPSTREAM_UNAVAILABLE"
	case http.StatusConflict:
		return "REQUEST_ERROR"
	default:
		if status >= http.StatusInternalServerError {
			return "SYSTEM_INTERNAL_ERROR"
		}
		return "REQUEST_ERROR"
	}
}

func normalizeMaxBytes(value int64) int64 {
	if value <= 0 {
		return 50 * 1024 * 1024
	}
	return value
}

func normalizeAccessCookieName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return defaultAccessCookieName
	}
	return name
}

func normalizeAccessCookiePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return defaultAccessCookiePath
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func normalizeAccessCookieMaxAge(age time.Duration) time.Duration {
	if age <= 0 {
		return defaultAccessCookieMaxAge
	}
	return age
}

func normalizeRefreshCookieName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return defaultRefreshCookieName
	}
	return name
}

func normalizeRefreshCookiePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return defaultRefreshCookiePath
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func normalizeRefreshCookieMaxAge(age time.Duration) time.Duration {
	if age <= 0 {
		return defaultRefreshCookieMaxAge
	}
	return age
}

func parseSameSiteMode(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "default":
		return http.SameSiteDefaultMode
	case "lax", "":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteLaxMode
	}
}

func normalizeExtensions(exts []string) map[string]struct{} {
	if len(exts) == 0 {
		exts = []string{".pdf", ".epub", ".txt"}
	}
	out := make(map[string]struct{}, len(exts))
	for _, ext := range exts {
		ext = strings.ToLower(strings.TrimSpace(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		out[ext] = struct{}{}
	}
	return out
}

func (s *Server) audit(r *http.Request, event, outcome string, attrs ...any) {
	logAttrs := []any{
		"event", event,
		"outcome", outcome,
		"path", r.URL.Path,
		"method", r.Method,
		"ip", util.ClientIP(r, s.trustedProxies),
		"request_id", requestIDFromRequest(r),
	}
	logAttrs = append(logAttrs, attrs...)
	if outcome == "success" {
		slog.Info("security_event", logAttrs...)
		return
	}
	slog.Warn("security_event", logAttrs...)
}

func (s *Server) allowRate(w http.ResponseWriter, r *http.Request, limiter *ratelimit.FixedWindowLimiter, msg, code string) bool {
	key := r.URL.Path + "|" + util.ClientIP(r, s.trustedProxies)
	if limiter.Allow(key) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeErrorWithCode(w, r, http.StatusTooManyRequests, msg, code)
	return false
}

func (s *Server) isExtensionAllowed(filename string) bool {
	if len(s.allowedExtensions) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(filename))
	_, ok := s.allowedExtensions[ext]
	return ok
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	if apiErr, ok := err.(*authclient.APIError); ok {
		slog.Warn(
			"gateway_upstream_error",
			"upstream", "auth",
			"status", apiErr.Status,
			"code", apiErr.Code,
			"path", r.URL.Path,
			"method", r.Method,
			"request_id", requestIDFromRequest(r),
		)
		writeErrorWithCode(w, r, apiErr.Status, apiErr.Message, apiErr.Code)
		return
	}
	slog.Error(
		"gateway_upstream_unavailable",
		"upstream", "auth",
		"path", r.URL.Path,
		"method", r.Method,
		"request_id", requestIDFromRequest(r),
		"err", err,
	)
	writeErrorWithCode(w, r, http.StatusBadGateway, "auth service unavailable", "AUTH_SERVICE_UNAVAILABLE")
}

func isUnauthorizedAuthError(err error) bool {
	apiErr, ok := err.(*authclient.APIError)
	if !ok {
		return false
	}
	return apiErr.Status == http.StatusUnauthorized
}

func writeBookError(w http.ResponseWriter, r *http.Request, err error) {
	if apiErr, ok := err.(*bookclient.APIError); ok {
		slog.Warn(
			"gateway_upstream_error",
			"upstream", "book",
			"status", apiErr.Status,
			"code", apiErr.Code,
			"path", r.URL.Path,
			"method", r.Method,
			"request_id", requestIDFromRequest(r),
		)
		writeErrorWithCode(w, r, apiErr.Status, apiErr.Message, apiErr.Code)
		return
	}
	slog.Error(
		"gateway_upstream_unavailable",
		"upstream", "book",
		"path", r.URL.Path,
		"method", r.Method,
		"request_id", requestIDFromRequest(r),
		"err", err,
	)
	writeErrorWithCode(w, r, http.StatusBadGateway, "book service unavailable", "BOOK_SERVICE_UNAVAILABLE")
}

func writeChatError(w http.ResponseWriter, r *http.Request, err error) {
	if apiErr, ok := err.(*chatclient.APIError); ok {
		slog.Warn(
			"gateway_upstream_error",
			"upstream", "chat",
			"status", apiErr.Status,
			"code", apiErr.Code,
			"path", r.URL.Path,
			"method", r.Method,
			"request_id", requestIDFromRequest(r),
		)
		writeErrorWithCode(w, r, apiErr.Status, apiErr.Message, apiErr.Code)
		return
	}
	slog.Error(
		"gateway_upstream_unavailable",
		"upstream", "chat",
		"path", r.URL.Path,
		"method", r.Method,
		"request_id", requestIDFromRequest(r),
		"err", err,
	)
	writeErrorWithCode(w, r, http.StatusBadGateway, "chat service unavailable", "CHAT_SERVICE_UNAVAILABLE")
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if requestID == "" {
			requestID = util.NewID()
		}
		w.Header().Set("X-Request-Id", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	id, _ := r.Context().Value(requestIDContextKey).(string)
	return id
}
