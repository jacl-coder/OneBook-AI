package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
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

// Config wires required dependencies for the HTTP server.
type Config struct {
	Auth                       *authclient.Client
	Book                       *bookclient.Client
	Chat                       *chatclient.Client
	TokenVerifier              *usertoken.Verifier
	RedisAddr                  string
	RedisPassword              string
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
	}
	s.routes()
	return s, nil
}

// Router returns the configured handler.
func (s *Server) Router() http.Handler {
	return util.WithSecurityHeaders(util.WithCORS(s.mux))
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
			writeError(w, http.StatusUnauthorized, "unauthorized")
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
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		user, err := s.auth.Me(token)
		if err != nil {
			s.audit(r, "gateway.admin.authorize", "fail", "reason", "auth_me_failed")
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if user.Role != domain.RoleAdmin {
			s.audit(r, "gateway.admin.authorize", "fail", "user_id", user.ID, "reason", "forbidden")
			writeError(w, http.StatusForbidden, "forbidden")
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
	user, err := s.auth.Me(token)
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
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.signupLimiter, "too many signup attempts") {
		s.audit(r, "gateway.signup", "rate_limited")
		return
	}
	var req authRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "gateway.signup", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, accessToken, refreshToken, err := s.auth.SignUp(req.Email, req.Password)
	if err != nil {
		s.audit(r, "gateway.signup", "fail", "reason", err.Error())
		writeAuthError(w, err)
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
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.loginLimiter, "too many login attempts") {
		s.audit(r, "gateway.login", "rate_limited")
		return
	}
	var req authRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "gateway.login", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, accessToken, refreshToken, err := s.auth.Login(req.Email, req.Password)
	if err != nil {
		s.audit(r, "gateway.login", "fail", "reason", err.Error())
		writeAuthError(w, err)
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
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.refreshLimiter, "too many refresh attempts") {
		s.audit(r, "gateway.refresh", "rate_limited")
		return
	}
	var req refreshRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		s.audit(r, "gateway.refresh", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		s.audit(r, "gateway.refresh", "fail", "reason", "missing_refresh_token")
		writeError(w, http.StatusBadRequest, "refreshToken is required")
		return
	}
	user, accessToken, refreshToken, err := s.auth.Refresh(req.RefreshToken)
	if err != nil {
		s.audit(r, "gateway.refresh", "fail", "reason", err.Error())
		writeAuthError(w, err)
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
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.refreshLimiter, "too many logout attempts") {
		s.audit(r, "gateway.logout", "rate_limited")
		return
	}
	var req logoutRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		s.audit(r, "gateway.logout", "fail", "reason", "invalid_json")
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		s.audit(r, "gateway.logout", "fail", "reason", "missing_token")
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.auth.Logout(token, req.RefreshToken); err != nil {
		s.audit(r, "gateway.logout", "fail", "reason", err.Error())
		writeAuthError(w, err)
		return
	}
	s.audit(r, "gateway.logout", "success")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	keys, err := s.auth.JWKS()
	if err != nil {
		writeAuthError(w, err)
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
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if req.Email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		updated, err := s.auth.UpdateMe(token, req.Email)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, user domain.User) {
	_ = user
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !s.allowRate(w, r, s.passwordLimiter, "too many password change attempts") {
		s.audit(r, "gateway.password.change", "rate_limited")
		return
	}
	var req changePasswordRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "currentPassword and newPassword are required")
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.auth.ChangePassword(token, req.CurrentPassword, req.NewPassword); err != nil {
		writeAuthError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// /api/books
func (s *Server) handleBooks(w http.ResponseWriter, r *http.Request, user domain.User) {
	_ = user
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	switch r.Method {
	case http.MethodPost:
		s.handleUploadBook(w, r, token)
	case http.MethodGet:
		s.handleListBooks(w, token)
	default:
		methodNotAllowed(w)
	}
}

// /api/books/{id} or /api/books/{id}/download
func (s *Server) handleBookByID(w http.ResponseWriter, r *http.Request, user domain.User) {
	_ = user
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/books/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	if id == "" {
		http.NotFound(w, r)
		return
	}

	// Handle /api/books/{id}/download
	if len(parts) == 2 && parts[1] == "download" {
		s.handleDownloadBook(w, r, token, id)
		return
	}
	if len(parts) == 2 {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		book, err := s.books.GetBook(token, id)
		if err != nil {
			writeBookError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, book)
	case http.MethodDelete:
		if err := s.books.DeleteBook(token, id); err != nil {
			writeBookError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		methodNotAllowed(w)
	}
}

// handleDownloadBook returns a pre-signed download URL for the book file.
func (s *Server) handleDownloadBook(w http.ResponseWriter, r *http.Request, token, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	resp, err := s.books.GetDownloadURL(token, id)
	if err != nil {
		writeBookError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUploadBook(w http.ResponseWriter, r *http.Request, token string) {
	if s.maxUploadBytes > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, s.maxUploadBytes)
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid form data")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required (field: file)")
		return
	}
	defer file.Close()
	if !s.isExtensionAllowed(header.Filename) {
		writeError(w, http.StatusBadRequest, "unsupported file type")
		return
	}
	book, err := s.books.UploadBook(token, header.Filename, file)
	if err != nil {
		writeBookError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, book)
}

func (s *Server) handleListBooks(w http.ResponseWriter, token string) {
	books, err := s.books.ListBooks(token)
	if err != nil {
		writeBookError(w, err)
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
		methodNotAllowed(w)
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req chatRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.BookID == "" {
		writeError(w, http.StatusBadRequest, "bookId is required")
		return
	}
	ans, err := s.chat.AskQuestion(token, req.BookID, req.Question)
	if err != nil {
		writeChatError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ans)
}

// admin handlers
func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request, user domain.User) {
	_ = user
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	users, err := s.auth.AdminListUsers(token)
	if err != nil {
		writeAuthError(w, err)
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
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	updated, err := s.auth.AdminUpdateUser(token, id, role, status)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleAdminBooks(w http.ResponseWriter, r *http.Request, user domain.User) {
	_ = user
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	books, err := s.books.ListBooks(token)
	if err != nil {
		writeBookError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": books,
		"count": len(books),
	})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
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

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
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
		"ip", clientIP(r),
	}
	logAttrs = append(logAttrs, attrs...)
	if outcome == "success" {
		slog.Info("security_event", logAttrs...)
		return
	}
	slog.Warn("security_event", logAttrs...)
}

func (s *Server) allowRate(w http.ResponseWriter, r *http.Request, limiter *ratelimit.FixedWindowLimiter, msg string) bool {
	key := r.URL.Path + "|" + clientIP(r)
	if limiter.Allow(key) {
		return true
	}
	w.Header().Set("Retry-After", "60")
	writeError(w, http.StatusTooManyRequests, msg)
	return false
}

func clientIP(r *http.Request) string {
	if xfwd := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xfwd != "" {
		parts := strings.Split(xfwd, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func (s *Server) isExtensionAllowed(filename string) bool {
	if len(s.allowedExtensions) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(filename))
	_, ok := s.allowedExtensions[ext]
	return ok
}

func writeAuthError(w http.ResponseWriter, err error) {
	if apiErr, ok := err.(*authclient.APIError); ok {
		writeError(w, apiErr.Status, apiErr.Message)
		return
	}
	writeError(w, http.StatusBadGateway, "auth service unavailable")
}

func writeBookError(w http.ResponseWriter, err error) {
	if apiErr, ok := err.(*bookclient.APIError); ok {
		writeError(w, apiErr.Status, apiErr.Message)
		return
	}
	writeError(w, http.StatusBadGateway, "book service unavailable")
}

func writeChatError(w http.ResponseWriter, err error) {
	if apiErr, ok := err.(*chatclient.APIError); ok {
		writeError(w, apiErr.Status, apiErr.Message)
		return
	}
	writeError(w, http.StatusBadGateway, "chat service unavailable")
}
