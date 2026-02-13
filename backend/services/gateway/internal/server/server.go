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
	"strings"
	"time"

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

// Config wires required dependencies for the HTTP server.
type Config struct {
	Auth                       *authclient.Client
	Book                       *bookclient.Client
	Chat                       *chatclient.Client
	TokenVerifier              *usertoken.Verifier
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
		auth:              cfg.Auth,
		books:             cfg.Book,
		chat:              cfg.Chat,
		tokenVerifier:     cfg.TokenVerifier,
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

	// admin
	s.mux.Handle("/api/admin/users", s.adminOnly(s.handleAdminUsers))
	s.mux.Handle("/api/admin/users/", s.adminOnly(s.handleAdminUserByID))
	s.mux.Handle("/api/admin/books", s.adminOnly(s.handleAdminBooks))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// auth wrappers
type authHandler func(http.ResponseWriter, *http.Request, domain.User)

func (s *Server) authenticated(next authHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.authorize(r)
		if !ok {
			s.audit(r, "gateway.authorize", "fail")
			writeErrorWithCode(w, r, http.StatusUnauthorized, "unauthorized", "AUTH_INVALID_TOKEN")
			return
		}
		s.audit(r, "gateway.authorize", "success", "user_id", user.ID)
		next(w, r, user)
	})
}

func (s *Server) adminOnly(next authHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			s.audit(r, "gateway.admin.authorize", "fail", "reason", "missing_token")
			writeErrorWithCode(w, r, http.StatusUnauthorized, "unauthorized", "AUTH_INVALID_TOKEN")
			return
		}
		user, err := s.auth.Me(requestIDFromRequest(r), token)
		if err != nil {
			s.audit(r, "gateway.admin.authorize", "fail", "reason", "auth_me_failed")
			writeErrorWithCode(w, r, http.StatusUnauthorized, "unauthorized", "AUTH_INVALID_TOKEN")
			return
		}
		if user.Role != domain.RoleAdmin {
			s.audit(r, "gateway.admin.authorize", "fail", "user_id", user.ID, "reason", "forbidden")
			writeErrorWithCode(w, r, http.StatusForbidden, "forbidden", "ADMIN_FORBIDDEN")
			return
		}
		s.audit(r, "gateway.admin.authorize", "success", "user_id", user.ID)
		next(w, r, user)
	})
}

func (s *Server) authorize(r *http.Request) (domain.User, bool) {
	token, ok := bearerToken(r)
	if !ok {
		s.audit(r, "gateway.token.verify", "fail", "reason", "missing_token")
		return domain.User{}, false
	}
	if s.tokenVerifier != nil {
		if _, err := s.tokenVerifier.VerifySubject(token); err != nil {
			s.audit(r, "gateway.token.verify", "fail", "reason", "invalid_signature_or_claims")
			return domain.User{}, false
		}
	}
	user, err := s.auth.Me(requestIDFromRequest(r), token)
	if err != nil {
		s.audit(r, "gateway.token.verify", "fail", "reason", "auth_me_failed")
		return domain.User{}, false
	}
	s.audit(r, "gateway.token.verify", "success", "user_id", user.ID)
	return user, true
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
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid JSON body", "AUTH_INVALID_REQUEST")
		return
	}
	user, accessToken, refreshToken, err := s.auth.SignUp(requestIDFromRequest(r), req.Email, req.Password)
	if err != nil {
		s.audit(r, "gateway.signup", "fail", "reason", err.Error())
		writeAuthError(w, r, err)
		return
	}
	s.audit(r, "gateway.signup", "success", "user_id", user.ID)
	writeJSON(w, http.StatusCreated, authResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
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
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid JSON body", "AUTH_INVALID_REQUEST")
		return
	}
	user, accessToken, refreshToken, err := s.auth.Login(requestIDFromRequest(r), req.Email, req.Password)
	if err != nil {
		s.audit(r, "gateway.login", "fail", "reason", err.Error())
		writeAuthError(w, r, err)
		return
	}
	s.audit(r, "gateway.login", "success", "user_id", user.ID)
	writeJSON(w, http.StatusOK, authResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
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
	var req refreshRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "gateway.refresh", "fail", "reason", "invalid_json")
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid JSON body", "AUTH_INVALID_REQUEST")
		return
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		s.audit(r, "gateway.refresh", "fail", "reason", "missing_refresh_token")
		writeErrorWithCode(w, r, http.StatusBadRequest, "refreshToken is required", "AUTH_REFRESH_TOKEN_REQUIRED")
		return
	}
	user, accessToken, refreshToken, err := s.auth.Refresh(requestIDFromRequest(r), req.RefreshToken)
	if err != nil {
		s.audit(r, "gateway.refresh", "fail", "reason", err.Error())
		writeAuthError(w, r, err)
		return
	}
	s.audit(r, "gateway.refresh", "success", "user_id", user.ID)
	writeJSON(w, http.StatusOK, authResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
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
	var req logoutRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		s.audit(r, "gateway.logout", "fail", "reason", "invalid_json")
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid JSON body", "AUTH_INVALID_REQUEST")
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		s.audit(r, "gateway.logout", "fail", "reason", "missing_token")
		writeErrorWithCode(w, r, http.StatusUnauthorized, "unauthorized", "AUTH_INVALID_TOKEN")
		return
	}
	if err := s.auth.Logout(requestIDFromRequest(r), token, req.RefreshToken); err != nil {
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

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, user domain.User) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, user)
	case http.MethodPatch:
		var req updateMeRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeErrorWithCode(w, r, http.StatusBadRequest, "invalid JSON body", "AUTH_INVALID_REQUEST")
			return
		}
		if req.Email == "" {
			writeErrorWithCode(w, r, http.StatusBadRequest, "email is required", "AUTH_EMAIL_REQUIRED")
			return
		}
		token, ok := bearerToken(r)
		if !ok {
			writeErrorWithCode(w, r, http.StatusUnauthorized, "unauthorized", "AUTH_INVALID_TOKEN")
			return
		}
		updated, err := s.auth.UpdateMe(requestIDFromRequest(r), token, req.Email)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		methodNotAllowed(w, r)
	}
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, user domain.User) {
	_ = user
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
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid JSON body", "AUTH_INVALID_REQUEST")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeErrorWithCode(w, r, http.StatusBadRequest, "currentPassword and newPassword are required", "AUTH_PASSWORD_FIELDS_REQUIRED")
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		writeErrorWithCode(w, r, http.StatusUnauthorized, "unauthorized", "AUTH_INVALID_TOKEN")
		return
	}
	if err := s.auth.ChangePassword(requestIDFromRequest(r), token, req.CurrentPassword, req.NewPassword); err != nil {
		writeAuthError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// /api/books
func (s *Server) handleBooks(w http.ResponseWriter, r *http.Request, user domain.User) {
	_ = user
	token, ok := bearerToken(r)
	if !ok {
		writeErrorWithCode(w, r, http.StatusUnauthorized, "unauthorized", "AUTH_INVALID_TOKEN")
		return
	}
	switch r.Method {
	case http.MethodPost:
		s.handleUploadBook(w, r, token)
	case http.MethodGet:
		s.handleListBooks(w, r, token)
	default:
		methodNotAllowed(w, r)
	}
}

// /api/books/{id} or /api/books/{id}/download
func (s *Server) handleBookByID(w http.ResponseWriter, r *http.Request, user domain.User) {
	_ = user
	token, ok := bearerToken(r)
	if !ok {
		writeErrorWithCode(w, r, http.StatusUnauthorized, "unauthorized", "AUTH_INVALID_TOKEN")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/books/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	if id == "" {
		writeErrorWithCode(w, r, http.StatusNotFound, "not found", "BOOK_NOT_FOUND")
		return
	}

	// Handle /api/books/{id}/download
	if len(parts) == 2 && parts[1] == "download" {
		s.handleDownloadBook(w, r, token, id)
		return
	}
	if len(parts) == 2 {
		writeErrorWithCode(w, r, http.StatusNotFound, "not found", "BOOK_NOT_FOUND")
		return
	}

	switch r.Method {
	case http.MethodGet:
		book, err := s.books.GetBook(requestIDFromRequest(r), token, id)
		if err != nil {
			writeBookError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, book)
	case http.MethodDelete:
		if err := s.books.DeleteBook(requestIDFromRequest(r), token, id); err != nil {
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

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request, user domain.User) {
	_ = user
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		writeErrorWithCode(w, r, http.StatusUnauthorized, "unauthorized", "AUTH_INVALID_TOKEN")
		return
	}
	var req chatRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid JSON body", "CHAT_INVALID_REQUEST")
		return
	}
	if req.BookID == "" {
		writeErrorWithCode(w, r, http.StatusBadRequest, "bookId is required", "CHAT_BOOK_ID_REQUIRED")
		return
	}
	ans, err := s.chat.AskQuestion(requestIDFromRequest(r), token, req.BookID, req.Question)
	if err != nil {
		writeChatError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ans)
}

// admin handlers
func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request, user domain.User) {
	_ = user
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		writeErrorWithCode(w, r, http.StatusUnauthorized, "unauthorized", "AUTH_INVALID_TOKEN")
		return
	}
	users, err := s.auth.AdminListUsers(requestIDFromRequest(r), token)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": users,
		"count": len(users),
	})
}

func (s *Server) handleAdminUserByID(w http.ResponseWriter, r *http.Request, user domain.User) {
	_ = user
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
		writeErrorWithCode(w, r, http.StatusBadRequest, "invalid JSON body", "ADMIN_INVALID_REQUEST")
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
	token, ok := bearerToken(r)
	if !ok {
		writeErrorWithCode(w, r, http.StatusUnauthorized, "unauthorized", "AUTH_INVALID_TOKEN")
		return
	}
	updated, err := s.auth.AdminUpdateUser(requestIDFromRequest(r), token, id, role, status)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleAdminBooks(w http.ResponseWriter, r *http.Request, user domain.User) {
	_ = user
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		writeErrorWithCode(w, r, http.StatusUnauthorized, "unauthorized", "AUTH_INVALID_TOKEN")
		return
	}
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

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeErrorWithCode(w, r, http.StatusMethodNotAllowed, "method not allowed", "SYSTEM_METHOD_NOT_ALLOWED")
}

type chatRequest struct {
	BookID   string `json:"bookId"`
	Question string `json:"question"`
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
		writeErrorWithCode(w, r, apiErr.Status, apiErr.Message, apiErr.Code)
		return
	}
	writeErrorWithCode(w, r, http.StatusBadGateway, "auth service unavailable", "AUTH_SERVICE_UNAVAILABLE")
}

func writeBookError(w http.ResponseWriter, r *http.Request, err error) {
	if apiErr, ok := err.(*bookclient.APIError); ok {
		writeErrorWithCode(w, r, apiErr.Status, apiErr.Message, apiErr.Code)
		return
	}
	writeErrorWithCode(w, r, http.StatusBadGateway, "book service unavailable", "BOOK_SERVICE_UNAVAILABLE")
}

func writeChatError(w http.ResponseWriter, r *http.Request, err error) {
	if apiErr, ok := err.(*chatclient.APIError); ok {
		writeErrorWithCode(w, r, apiErr.Status, apiErr.Message, apiErr.Code)
		return
	}
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
