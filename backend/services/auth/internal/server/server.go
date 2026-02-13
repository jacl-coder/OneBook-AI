package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"onebookai/internal/ratelimit"
	"onebookai/internal/util"
	"onebookai/pkg/domain"
	"onebookai/pkg/store"
	"onebookai/services/auth/internal/app"
	"onebookai/services/auth/internal/security"
)

// Config wires required dependencies for the HTTP server.
type Config struct {
	App                        *app.App
	RedisAddr                  string
	RedisPassword              string
	TrustedProxyCIDRs          []string
	SignupRateLimitPerMinute   int
	LoginRateLimitPerMinute    int
	RefreshRateLimitPerMinute  int
	PasswordRateLimitPerMinute int
}

// Server exposes HTTP endpoints for the auth service.
type Server struct {
	app             *app.App
	mux             *http.ServeMux
	signupLimiter   *ratelimit.FixedWindowLimiter
	loginLimiter    *ratelimit.FixedWindowLimiter
	refreshLimiter  *ratelimit.FixedWindowLimiter
	passwordLimiter *ratelimit.FixedWindowLimiter
	trustedProxies  *util.TrustedProxies
	alerter         *security.AuditAlerter
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
		prefix := "onebook:auth:ratelimit:" + name
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
		app:             cfg.App,
		mux:             http.NewServeMux(),
		signupLimiter:   signupLimiter,
		loginLimiter:    loginLimiter,
		refreshLimiter:  refreshLimiter,
		passwordLimiter: passwordLimiter,
		trustedProxies:  trustedProxies,
		alerter:         security.NewAuditAlerter(cfg.RedisAddr, cfg.RedisPassword, "onebook:auth:alerts"),
	}
	s.routes()
	return s, nil
}

// Router returns the configured handler.
func (s *Server) Router() http.Handler {
	return util.WithSecurityHeaders(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)

	// auth
	s.mux.HandleFunc("/auth/signup", s.handleSignup)
	s.mux.HandleFunc("/auth/login", s.handleLogin)
	s.mux.HandleFunc("/auth/refresh", s.handleRefresh)
	s.mux.HandleFunc("/auth/logout", s.handleLogout)
	s.mux.HandleFunc("/auth/jwks", s.handleJWKS)
	s.mux.HandleFunc("/.well-known/jwks.json", s.handleJWKS)
	s.mux.Handle("/auth/me", s.authenticated(s.handleMe))
	s.mux.Handle("/auth/me/password", s.authenticated(s.handleChangePassword))

	// admin
	s.mux.Handle("/auth/admin/users", s.adminOnly(s.handleAdminUsers))
	s.mux.Handle("/auth/admin/users/", s.adminOnly(s.handleAdminUserByID))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func isPasswordPolicyError(err error) bool {
	if err == nil {
		return false
	}
	// backend/pkg/auth.ValidatePassword returns errors starting with this prefix.
	return strings.HasPrefix(err.Error(), "password must")
}

// auth wrappers
type authHandler func(http.ResponseWriter, *http.Request, domain.User)

func (s *Server) authenticated(next authHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.authorize(r)
		if !ok {
			s.audit(r, "auth.authorize", "fail")
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		s.audit(r, "auth.authorize", "success", "user_id", user.ID)
		next(w, r, user)
	})
}

func (s *Server) adminOnly(next authHandler) http.Handler {
	return s.authenticated(func(w http.ResponseWriter, r *http.Request, user domain.User) {
		if user.Role != domain.RoleAdmin {
			s.audit(r, "auth.admin.authorize", "fail", "user_id", user.ID)
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		s.audit(r, "auth.admin.authorize", "success", "user_id", user.ID)
		next(w, r, user)
	})
}

func (s *Server) authorize(r *http.Request) (domain.User, bool) {
	token, ok := bearerToken(r)
	if !ok {
		return domain.User{}, false
	}
	return s.app.UserFromToken(token)
}

// auth handlers
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.signupLimiter, "too many signup attempts") {
		s.audit(r, "auth.signup", "rate_limited")
		return
	}
	var req authRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.signup", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, accessToken, refreshToken, err := s.app.SignUp(req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrEmailAndPasswordRequired):
			s.audit(r, "auth.signup", "fail", "reason", "missing_fields")
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, app.ErrEmailAlreadyExists):
			s.audit(r, "auth.signup", "fail", "reason", "email_exists")
			writeError(w, http.StatusBadRequest, err.Error())
		case isPasswordPolicyError(err):
			s.audit(r, "auth.signup", "fail", "reason", "invalid_password")
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			slog.Error("auth.signup_error", "err", err)
			s.audit(r, "auth.signup", "fail", "reason", "internal_error")
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	s.audit(r, "auth.signup", "success", "user_id", user.ID)
	writeJSON(w, http.StatusCreated, authResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.loginLimiter, "too many login attempts") {
		s.audit(r, "auth.login", "rate_limited")
		return
	}
	var req authRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.login", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, accessToken, refreshToken, err := s.app.Login(req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrUserDisabled):
			s.audit(r, "auth.login", "fail", "reason", "user_disabled")
			writeError(w, http.StatusUnauthorized, app.ErrInvalidCredentials.Error())
		case errors.Is(err, app.ErrInvalidCredentials):
			s.audit(r, "auth.login", "fail", "reason", "invalid_credentials")
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			slog.Error("auth.login_error", "err", err)
			s.audit(r, "auth.login", "fail", "reason", "internal_error")
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	s.audit(r, "auth.login", "success", "user_id", user.ID)
	writeJSON(w, http.StatusOK, authResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.refreshLimiter, "too many refresh attempts") {
		s.audit(r, "auth.refresh", "rate_limited")
		return
	}
	var req refreshRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.refresh", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, accessToken, refreshToken, err := s.app.Refresh(req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrRefreshTokenRequired):
			s.audit(r, "auth.refresh", "fail", "reason", "missing_refresh_token")
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, app.ErrInvalidRefreshToken):
			s.audit(r, "auth.refresh", "fail", "reason", "invalid_refresh_token")
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			slog.Error("auth.refresh_error", "err", err)
			s.audit(r, "auth.refresh", "fail", "reason", "internal_error")
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	s.audit(r, "auth.refresh", "success", "user_id", user.ID)
	writeJSON(w, http.StatusOK, authResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.refreshLimiter, "too many logout attempts") {
		s.audit(r, "auth.logout", "rate_limited")
		return
	}
	var req logoutRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		s.audit(r, "auth.logout", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		s.audit(r, "auth.logout", "fail", "reason", "missing_token")
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.app.Logout(token, req.RefreshToken); err != nil {
		s.audit(r, "auth.logout", "fail", "reason", err.Error())
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	s.audit(r, "auth.logout", "success")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	keys := s.app.JWKS()
	if len(keys) == 0 {
		writeError(w, http.StatusNotFound, "jwks not configured")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, http.StatusOK, jwksResponse{Keys: keys})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, user domain.User) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, user)
	case http.MethodPatch:
		var req updateMeRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.Email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}
		updated, err := s.app.UpdateMyEmail(user, req.Email)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.passwordLimiter, "too many password change attempts") {
		s.audit(r, "auth.password.change", "rate_limited", "user_id", user.ID)
		return
	}
	var req changePasswordRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "auth.password.change", "fail", "user_id", user.ID, "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		s.audit(r, "auth.password.change", "fail", "user_id", user.ID, "reason", "missing_fields")
		writeError(w, http.StatusBadRequest, "currentPassword and newPassword are required")
		return
	}
	if err := s.app.ChangePassword(user.ID, req.CurrentPassword, req.NewPassword); err != nil {
		s.audit(r, "auth.password.change", "fail", "user_id", user.ID, "reason", err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r, "auth.password.change", "success", "user_id", user.ID)
	w.WriteHeader(http.StatusNoContent)
}

// admin handlers
func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	users, err := s.app.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": users,
		"count": len(users),
	})
}

func (s *Server) handleAdminUserByID(w http.ResponseWriter, r *http.Request, user domain.User) {
	id := strings.TrimPrefix(r.URL.Path, "/auth/admin/users/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPatch {
		methodNotAllowed(w)
		return
	}
	var req adminUserUpdateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	var role *domain.UserRole
	if req.Role != "" {
		parsed, ok := parseUserRole(req.Role)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid role")
			return
		}
		role = &parsed
	}
	var status *domain.UserStatus
	if req.Status != "" {
		parsed, ok := parseUserStatus(req.Status)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid status")
			return
		}
		status = &parsed
	}
	if role == nil && status == nil {
		writeError(w, http.StatusBadRequest, "role or status is required")
		return
	}
	updated, err := s.app.AdminUpdateUser(user, id, role, status)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refreshToken,omitempty"`
	User         domain.User `json:"user"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken,omitempty"`
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

type jwksResponse struct {
	Keys []store.JWK `json:"keys"`
}

func bearerToken(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		slog.Warn("missing bearer prefix", "path", r.URL.Path)
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		slog.Warn("empty bearer token", "path", r.URL.Path)
		return "", false
	}
	return token, true
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

func (s *Server) allowRate(w http.ResponseWriter, r *http.Request, limiter *ratelimit.FixedWindowLimiter, msg string) bool {
	key := r.URL.Path + "|" + util.ClientIP(r, s.trustedProxies)
	if limiter.Allow(key) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeError(w, http.StatusTooManyRequests, msg)
	return false
}

func (s *Server) audit(r *http.Request, event, outcome string, attrs ...any) {
	ip := util.ClientIP(r, s.trustedProxies)
	logAttrs := []any{
		"event", event,
		"outcome", outcome,
		"path", r.URL.Path,
		"method", r.Method,
		"ip", ip,
	}
	logAttrs = append(logAttrs, attrs...)
	if s.alerter != nil && outcome != "success" {
		alert, err := s.alerter.Observe(event, outcome, ip)
		if err != nil {
			slog.Error("security_alert_error", "event", event, "outcome", outcome, "ip", ip, "err", err)
		} else if alert.Triggered {
			slog.Error(
				"security_alert",
				"event", event,
				"outcome", outcome,
				"ip", ip,
				"count", alert.Count,
				"threshold", alert.Threshold,
				"window", alert.Window.String(),
			)
		}
	}
	if outcome == "success" {
		slog.Info("security_event", logAttrs...)
		return
	}
	slog.Warn("security_event", logAttrs...)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
