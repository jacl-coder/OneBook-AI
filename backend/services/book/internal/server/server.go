package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"onebookai/pkg/domain"
	"onebookai/services/book/internal/app"
)

// Config wires required dependencies for the HTTP server.
type Config struct {
	App           *app.App
	InternalToken string
}

// Server exposes HTTP endpoints for the book service.
type Server struct {
	app           *app.App
	internalToken string
	mux           *http.ServeMux
}

// New constructs the server with routes configured.
func New(cfg Config) *Server {
	s := &Server{
		app:           cfg.App,
		internalToken: strings.TrimSpace(cfg.InternalToken),
		mux:           http.NewServeMux(),
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
	s.mux.Handle("/internal/books/", s.withInternal(s.handleInternalBook))

	// books
	s.mux.Handle("/books", s.withUser(s.handleBooks))
	s.mux.Handle("/books/", s.withUser(s.handleBookByID))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type userHandler func(http.ResponseWriter, *http.Request, domain.User)

func (s *Server) withUser(next userHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromHeaders(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r, user)
	})
}

func (s *Server) withInternal(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.Header.Get("X-Internal-Token"))
		if token == "" || token != s.internalToken {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	})
}

func userFromHeaders(r *http.Request) (domain.User, bool) {
	userID := strings.TrimSpace(r.Header.Get("X-User-Id"))
	role := strings.TrimSpace(r.Header.Get("X-User-Role"))
	if userID == "" || role == "" {
		return domain.User{}, false
	}
	userRole, ok := parseUserRole(role)
	if !ok {
		return domain.User{}, false
	}
	return domain.User{ID: userID, Role: userRole}, true
}

// /books
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

// /books/{id}
func (s *Server) handleBookByID(w http.ResponseWriter, r *http.Request, user domain.User) {
	id := strings.TrimPrefix(r.URL.Path, "/books/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	book, ok, err := s.app.GetBook(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
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

// /internal/books/{id}/file or /internal/books/{id}/status
func (s *Server) handleInternalBook(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/internal/books/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	action := parts[1]
	if id == "" || action == "" {
		http.NotFound(w, r)
		return
	}
	switch action {
	case "file":
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleInternalFile(w, id)
	case "status":
		if r.Method != http.MethodPatch {
			methodNotAllowed(w)
			return
		}
		s.handleInternalStatus(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleInternalFile(w http.ResponseWriter, id string) {
	url, filename, err := s.app.GetDownloadURL(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "book not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"url":      url,
		"filename": filename,
	})
}

func (s *Server) handleInternalStatus(w http.ResponseWriter, r *http.Request, id string) {
	var req statusRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	status, ok := parseBookStatus(req.Status)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}
	if err := s.app.UpdateStatus(id, status, req.ErrorMessage); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
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

func (s *Server) handleListBooks(w http.ResponseWriter, user domain.User) {
	books, err := s.app.ListBooks(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
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

func notFound(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusNotFound, msg)
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

func parseBookStatus(status string) (domain.BookStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case string(domain.StatusQueued):
		return domain.StatusQueued, true
	case string(domain.StatusProcessing):
		return domain.StatusProcessing, true
	case string(domain.StatusReady):
		return domain.StatusReady, true
	case string(domain.StatusFailed):
		return domain.StatusFailed, true
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

type statusRequest struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
}
