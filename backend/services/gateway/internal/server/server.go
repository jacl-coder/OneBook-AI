package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"onebookai/pkg/domain"
	"onebookai/services/gateway/internal/app"
)

// Config wires required dependencies for the HTTP server.
type Config struct {
	App *app.App
}

// Server exposes HTTP endpoints for the backend.
type Server struct {
	app *app.App
	mux *http.ServeMux
}

// New constructs the server with routes configured.
func New(cfg Config) *Server {
	s := &Server{
		app: cfg.App,
		mux: http.NewServeMux(),
	}
	s.routes()
	return s
}

// Router returns the configured handler.
func (s *Server) Router() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)

	// auth
	s.mux.HandleFunc("/api/auth/signup", s.handleSignup)
	s.mux.HandleFunc("/api/auth/login", s.handleLogin)
	s.mux.Handle("/api/users/me", s.authenticated(s.handleMe))

	// books & chats (auth required)
	s.mux.Handle("/api/books", s.authenticated(s.handleBooks))
	s.mux.Handle("/api/books/", s.authenticated(s.handleBookByID))
	s.mux.Handle("/api/chats", s.authenticated(s.handleChats))

	// admin
	s.mux.Handle("/api/admin/users", s.adminOnly(s.handleAdminUsers))
	s.mux.Handle("/api/admin/books", s.adminOnly(s.handleAdminBooks))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
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
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return domain.User{}, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
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
	var req authRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, token, err := s.app.SignUp(req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
	user, token, err := s.app.Login(req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// /api/books
func (s *Server) handleBooks(w http.ResponseWriter, r *http.Request, user domain.User) {
	switch r.Method {
	case http.MethodPost:
		s.handleUploadBook(w, r, user)
	case http.MethodGet:
		s.handleListBooks(w, r, user)
	default:
		methodNotAllowed(w)
	}
}

// /api/books/{id}
func (s *Server) handleBookByID(w http.ResponseWriter, r *http.Request, user domain.User) {
	id := strings.TrimPrefix(r.URL.Path, "/api/books/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	book, ok := s.app.GetBook(id)
	if !ok {
		notFound(w, "book not found")
		return
	}
	if book.OwnerID != user.ID && user.Role != domain.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, book)
	case http.MethodDelete:
		if err := s.app.DeleteBook(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		methodNotAllowed(w)
	}
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
	book, err := s.app.UploadBook(user, header.Filename, file, header.Size)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, book)
}

func (s *Server) handleListBooks(w http.ResponseWriter, r *http.Request, user domain.User) {
	books := s.app.ListBooks(user)
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
	ans, err := s.app.AskQuestion(user, req.BookID, req.Question)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, app.ErrBookNotReady) {
			status = http.StatusConflict
		} else if strings.Contains(err.Error(), "forbidden") {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ans)
}

// admin handlers
func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	users := s.app.ListUsers()
	writeJSON(w, http.StatusOK, map[string]any{
		"items": users,
		"count": len(users),
	})
}

func (s *Server) handleAdminBooks(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	books := s.app.ListBooks(user)
	writeJSON(w, http.StatusOK, map[string]any{
		"items": books,
		"count": len(books),
	})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func notFound(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusNotFound, msg)
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
