package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"onebookai/pkg/domain"
	"onebookai/services/chat/internal/app"
)

// Config wires required dependencies for the HTTP server.
type Config struct {
	App *app.App
}

// Server exposes HTTP endpoints for the chat service.
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
	s.mux.Handle("/chats", s.withUser(s.handleChats))
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
	if req.Question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}
	if req.Book.ID == "" || req.Book.OwnerID == "" {
		writeError(w, http.StatusBadRequest, "book is required")
		return
	}
	ans, err := s.app.AskQuestion(user, req.Book, req.Question)
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

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

type chatRequest struct {
	Book     domain.Book `json:"book"`
	Question string      `json:"question"`
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
