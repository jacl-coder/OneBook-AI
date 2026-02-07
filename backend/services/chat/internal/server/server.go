package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"onebookai/internal/usertoken"
	"onebookai/internal/util"
	"onebookai/pkg/domain"
	"onebookai/services/chat/internal/app"
	"onebookai/services/chat/internal/authclient"
	"onebookai/services/chat/internal/bookclient"
)

// Config wires required dependencies for the HTTP server.
type Config struct {
	App           *app.App
	Auth          *authclient.Client
	Books         *bookclient.Client
	TokenVerifier *usertoken.Verifier
}

// Server exposes HTTP endpoints for the chat service.
type Server struct {
	app           *app.App
	auth          *authclient.Client
	books         *bookclient.Client
	tokenVerifier *usertoken.Verifier
	mux           *http.ServeMux
}

// New constructs the server with routes configured.
func New(cfg Config) *Server {
	s := &Server{
		app:           cfg.App,
		auth:          cfg.Auth,
		books:         cfg.Books,
		tokenVerifier: cfg.TokenVerifier,
		mux:           http.NewServeMux(),
	}
	s.routes()
	return s
}

// Router returns the configured handler.
func (s *Server) Router() http.Handler {
	return util.WithSecurityHeaders(util.WithCORS(s.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.Handle("/chats", s.withUser(s.handleChats))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type userHandler func(http.ResponseWriter, *http.Request, string, domain.User)

func (s *Server) withUser(next userHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			writeError(w, http.StatusInternalServerError, "auth client not configured")
			return
		}
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if s.tokenVerifier != nil {
			if _, err := s.tokenVerifier.VerifySubject(token); err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		}
		user, err := s.auth.Me(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r, token, user)
	})
}

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request, token string, user domain.User) {
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
	if req.BookID == "" {
		writeError(w, http.StatusBadRequest, "bookId is required")
		return
	}
	if s.books == nil {
		writeError(w, http.StatusInternalServerError, "book client not configured")
		return
	}
	book, err := s.books.GetBook(token, req.BookID)
	if err != nil {
		writeBookError(w, err)
		return
	}
	ans, err := s.app.AskQuestion(user, book, req.Question)
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
	BookID   string `json:"bookId"`
	Question string `json:"question"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeBookError(w http.ResponseWriter, err error) {
	if apiErr, ok := err.(*bookclient.APIError); ok {
		writeError(w, apiErr.Status, apiErr.Message)
		return
	}
	writeError(w, http.StatusBadGateway, "book service unavailable")
}

func bearerToken(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return "", false
	}
	return token, true
}
