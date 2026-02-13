package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"onebookai/internal/servicetoken"
	"onebookai/internal/usertoken"
	"onebookai/internal/util"
	"onebookai/pkg/domain"
	"onebookai/services/book/internal/app"
	"onebookai/services/book/internal/authclient"
)

// Config wires required dependencies for the HTTP server.
type Config struct {
	App                         *app.App
	Auth                        *authclient.Client
	TokenVerifier               *usertoken.Verifier
	InternalJWTKeyID            string
	InternalJWTPublicKeyPath    string
	InternalJWTVerifyPublicKeys map[string]string
	MaxUploadBytes              int64
}

// Server exposes HTTP endpoints for the book service.
type Server struct {
	app            *app.App
	auth           *authclient.Client
	tokenVerifier  *usertoken.Verifier
	internalVerify *servicetoken.Verifier
	mux            *http.ServeMux
	maxUploadBytes int64
}

// New constructs the server with routes configured.
func New(cfg Config) (*Server, error) {
	maxUploadBytes := cfg.MaxUploadBytes
	if maxUploadBytes <= 0 {
		maxUploadBytes = 50 * 1024 * 1024
	}
	s := &Server{
		app:            cfg.App,
		auth:           cfg.Auth,
		tokenVerifier:  cfg.TokenVerifier,
		mux:            http.NewServeMux(),
		maxUploadBytes: maxUploadBytes,
	}
	verifier, err := servicetoken.NewVerifierWithOptions(servicetoken.VerifierOptions{
		PublicKeyPath:      strings.TrimSpace(cfg.InternalJWTPublicKeyPath),
		VerifyPublicKeyMap: cfg.InternalJWTVerifyPublicKeys,
		DefaultKeyID:       cfg.InternalJWTKeyID,
		Audience:           "book",
		AllowedIssuers:     []string{"ingest-service", "indexer-service"},
		Leeway:             servicetoken.DefaultLeeway,
	})
	if err != nil {
		return nil, err
	}
	s.internalVerify = verifier
	s.routes()
	return s, nil
}

// Router returns the configured handler.
func (s *Server) Router() http.Handler {
	return util.WithRequestID(util.WithRequestLog("book", util.WithSecurityHeaders(util.WithCORS(s.mux))))
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
		next(w, r, user)
	})
}

func (s *Server) withInternal(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.internalVerify == nil {
			writeError(w, http.StatusInternalServerError, "internal auth not configured")
			return
		}
		token, ok := servicetoken.BearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if _, err := s.internalVerify.Verify(token); err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	})
}

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

// /books/{id} or /books/{id}/download
func (s *Server) handleBookByID(w http.ResponseWriter, r *http.Request, user domain.User) {
	path := strings.TrimPrefix(r.URL.Path, "/books/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	if id == "" {
		notFound(w, "not found")
		return
	}

	// Check if this is a download request
	if len(parts) == 2 && parts[1] == "download" {
		s.handleDownloadBook(w, r, user, id)
		return
	}
	if len(parts) == 2 {
		notFound(w, "not found")
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

// handleDownloadBook returns a pre-signed download URL for the book file.
func (s *Server) handleDownloadBook(w http.ResponseWriter, r *http.Request, user domain.User, id string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
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
	url, filename, err := s.app.GetDownloadURL(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate download URL")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"url":      url,
		"filename": filename,
	})
}

// /internal/books/{id}/file or /internal/books/{id}/status
func (s *Server) handleInternalBook(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/internal/books/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		notFound(w, "not found")
		return
	}
	id := parts[0]
	action := parts[1]
	if id == "" || action == "" {
		notFound(w, "not found")
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
		notFound(w, "not found")
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

type errorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"requestId,omitempty"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{
		Error:     msg,
		Code:      errorCodeForBook(status, msg),
		RequestID: strings.TrimSpace(w.Header().Get("X-Request-Id")),
	})
}

func errorCodeForBook(status int, msg string) string {
	message := strings.ToLower(strings.TrimSpace(msg))
	switch {
	case message == "auth client not configured", message == "internal auth not configured":
		return "SYSTEM_INTERNAL_ERROR"
	case message == "unauthorized":
		return "AUTH_INVALID_TOKEN"
	case message == "forbidden":
		return "BOOK_FORBIDDEN"
	case message == "book not found":
		return "BOOK_NOT_FOUND"
	case message == "file too large":
		return "BOOK_FILE_TOO_LARGE"
	case message == "filename required", strings.Contains(message, "file is required"):
		return "BOOK_FILE_REQUIRED"
	case strings.Contains(message, "unsupported file type"):
		return "BOOK_UNSUPPORTED_FILE_TYPE"
	case message == "invalid form data":
		return "BOOK_INVALID_UPLOAD_FORM"
	case message == "invalid json body":
		return "BOOK_INVALID_REQUEST"
	case message == "invalid status":
		return "BOOK_INVALID_STATUS"
	case message == "failed to generate download url":
		return "BOOK_DOWNLOAD_URL_FAILED"
	case message == "method not allowed":
		return "SYSTEM_METHOD_NOT_ALLOWED"
	case message == "not found":
		return "SYSTEM_NOT_FOUND"
	}

	switch status {
	case http.StatusBadRequest:
		return "BOOK_INVALID_REQUEST"
	case http.StatusUnauthorized:
		return "AUTH_INVALID_TOKEN"
	case http.StatusForbidden:
		return "BOOK_FORBIDDEN"
	case http.StatusNotFound:
		return "BOOK_NOT_FOUND"
	case http.StatusMethodNotAllowed:
		return "SYSTEM_METHOD_NOT_ALLOWED"
	default:
		if status >= http.StatusInternalServerError {
			return "SYSTEM_INTERNAL_ERROR"
		}
		return "REQUEST_ERROR"
	}
}

type statusRequest struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage"`
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
