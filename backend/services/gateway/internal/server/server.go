package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"onebookai/internal/util"
	"onebookai/pkg/domain"
	"onebookai/services/gateway/internal/authclient"
	"onebookai/services/gateway/internal/bookclient"
	"onebookai/services/gateway/internal/chatclient"
)

// Config wires required dependencies for the HTTP server.
type Config struct {
	Auth *authclient.Client
	Book *bookclient.Client
	Chat *chatclient.Client
}

// Server exposes HTTP endpoints for the backend.
type Server struct {
	auth  *authclient.Client
	books *bookclient.Client
	chat  *chatclient.Client
	mux   *http.ServeMux
}

// New constructs the server with routes configured.
func New(cfg Config) *Server {
	s := &Server{
		auth:  cfg.Auth,
		books: cfg.Book,
		chat:  cfg.Chat,
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s
}

// Router returns the configured handler.
func (s *Server) Router() http.Handler {
	return util.WithCORS(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)

	// auth
	s.mux.HandleFunc("/api/auth/signup", s.handleSignup)
	s.mux.HandleFunc("/api/auth/login", s.handleLogin)
	s.mux.HandleFunc("/api/auth/logout", s.handleLogout)
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
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r, user)
	})
}

func (s *Server) adminOnly(next authHandler) http.Handler {
	return s.authenticated(func(w http.ResponseWriter, r *http.Request, user domain.User) {
		if user.Role != domain.RoleAdmin {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r, user)
	})
}

func (s *Server) authorize(r *http.Request) (domain.User, bool) {
	token, ok := bearerToken(r)
	if !ok {
		return domain.User{}, false
	}
	user, err := s.auth.Me(token)
	if err != nil {
		return domain.User{}, false
	}
	return user, true
}

// auth handlers
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req authRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, token, err := s.auth.SignUp(req.Email, req.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{Token: token, User: user})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req authRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, token, err := s.auth.Login(req.Email, req.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.auth.Logout(token); err != nil {
		writeAuthError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	switch r.Method {
	case http.MethodPost:
		s.handleUploadBook(w, r, user)
	case http.MethodGet:
		s.handleListBooks(w, user)
	default:
		methodNotAllowed(w)
	}
}

// /api/books/{id} or /api/books/{id}/download
func (s *Server) handleBookByID(w http.ResponseWriter, r *http.Request, user domain.User) {
	path := strings.TrimPrefix(r.URL.Path, "/api/books/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	if id == "" {
		http.NotFound(w, r)
		return
	}

	// Handle /api/books/{id}/download
	if len(parts) == 2 && parts[1] == "download" {
		s.handleDownloadBook(w, r, user, id)
		return
	}
	if len(parts) == 2 {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		book, err := s.books.GetBook(user, id)
		if err != nil {
			writeBookError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, book)
	case http.MethodDelete:
		if err := s.books.DeleteBook(user, id); err != nil {
			writeBookError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		methodNotAllowed(w)
	}
}

// handleDownloadBook returns a pre-signed download URL for the book file.
func (s *Server) handleDownloadBook(w http.ResponseWriter, r *http.Request, user domain.User, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	resp, err := s.books.GetDownloadURL(user, id)
	if err != nil {
		writeBookError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUploadBook(w http.ResponseWriter, r *http.Request, user domain.User) {
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
	book, err := s.books.UploadBook(user, header.Filename, file)
	if err != nil {
		writeBookError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, book)
}

func (s *Server) handleListBooks(w http.ResponseWriter, user domain.User) {
	books, err := s.books.ListBooks(user)
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
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
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
	book, err := s.books.GetBook(user, req.BookID)
	if err != nil {
		writeBookError(w, err)
		return
	}
	ans, err := s.chat.AskQuestion(user, book, req.Question)
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
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	books, err := s.books.ListBooks(user)
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
	Token string      `json:"token"`
	User  domain.User `json:"user"`
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
