package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"onebookai/internal/servicetoken"
	"onebookai/internal/util"
	"onebookai/services/ingest/internal/app"
)

// Config wires required dependencies for the HTTP server.
type Config struct {
	App                         *app.App
	InternalJWTKeyID            string
	InternalJWTPublicKeyPath    string
	InternalJWTVerifyPublicKeys map[string]string
}

// Server exposes HTTP endpoints for the ingest service.
type Server struct {
	app          *app.App
	internalAuth *servicetoken.Verifier
	mux          *http.ServeMux
}

// New constructs the server with routes configured.
func New(cfg Config) (*Server, error) {
	s := &Server{
		app: cfg.App,
		mux: http.NewServeMux(),
	}
	verifier, err := servicetoken.NewVerifierWithOptions(servicetoken.VerifierOptions{
		PublicKeyPath:      strings.TrimSpace(cfg.InternalJWTPublicKeyPath),
		VerifyPublicKeyMap: cfg.InternalJWTVerifyPublicKeys,
		DefaultKeyID:       cfg.InternalJWTKeyID,
		Audience:           "ingest",
		AllowedIssuers:     []string{"book-service"},
		Leeway:             servicetoken.DefaultLeeway,
	})
	if err != nil {
		return nil, err
	}
	s.internalAuth = verifier
	s.routes()
	return s, nil
}

// Router returns the configured handler.
func (s *Server) Router() http.Handler {
	return util.WithRequestID(util.WithRequestLog("ingest", util.WithSecurityHeaders(util.WithCORS(s.mux))))
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.Handle("/ingest/jobs", s.withInternal(s.handleJobs))
	s.mux.Handle("/ingest/jobs/", s.withInternal(s.handleJobByID))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) withInternal(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.internalAuth == nil {
			writeError(w, http.StatusInternalServerError, "internal auth not configured")
			return
		}
		token, ok := servicetoken.BearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if _, err := s.internalAuth.Verify(token); err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	})
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req ingestRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	job, err := s.app.Enqueue(req.BookID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) handleJobByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/ingest/jobs/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	job, ok := s.app.GetJob(id)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

type ingestRequest struct {
	BookID string `json:"bookId"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type errorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"requestId,omitempty"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{
		Error:     msg,
		Code:      errorCodeForStatus(status),
		RequestID: strings.TrimSpace(w.Header().Get("X-Request-Id")),
	})
}

func errorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "REQUEST_ERROR"
	case http.StatusUnauthorized:
		return "AUTH_INVALID_TOKEN"
	case http.StatusNotFound:
		return "SYSTEM_NOT_FOUND"
	case http.StatusMethodNotAllowed:
		return "SYSTEM_METHOD_NOT_ALLOWED"
	default:
		if status >= http.StatusInternalServerError {
			return "SYSTEM_INTERNAL_ERROR"
		}
		return "REQUEST_ERROR"
	}
}
