package server

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"onebookai/pkg/domain"
	"onebookai/pkg/store"
	"onebookai/services/auth/internal/app"
)

func (s *Server) handleAdminEvalOverview(w http.ResponseWriter, r *http.Request, _ domain.User) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	windowStart := time.Now().UTC().Add(-24 * time.Hour)
	if raw := strings.TrimSpace(r.URL.Query().Get("windowHours")); raw != "" {
		if hours, err := strconv.Atoi(raw); err == nil && hours > 0 {
			windowStart = time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
		}
	}
	overview, err := s.app.AdminGetEvalOverview(windowStart)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (s *Server) handleAdminEvalDatasets(w http.ResponseWriter, r *http.Request, user domain.User) {
	switch r.Method {
	case http.MethodGet:
		page, pageSize, err := parsePageParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		items, total, err := s.app.AdminListEvalDatasets(store.EvalDatasetListOptions{
			Query:      strings.TrimSpace(r.URL.Query().Get("query")),
			SourceType: strings.TrimSpace(r.URL.Query().Get("sourceType")),
			Status:     strings.TrimSpace(r.URL.Query().Get("status")),
			BookID:     strings.TrimSpace(r.URL.Query().Get("bookId")),
			Page:       page,
			PageSize:   pageSize,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		totalPages := 0
		if pageSize > 0 {
			totalPages = (total + pageSize - 1) / pageSize
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items":      items,
			"count":      len(items),
			"page":       page,
			"pageSize":   pageSize,
			"total":      total,
			"totalPages": totalPages,
		})
	case http.MethodPost:
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "invalid multipart form")
			return
		}
		version := 1
		if raw := strings.TrimSpace(r.FormValue("version")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				version = parsed
			}
		}
		input := app.EvalDatasetCreateInput{
			Name:        r.FormValue("name"),
			SourceType:  domain.EvalDatasetSourceType(strings.TrimSpace(r.FormValue("sourceType"))),
			BookID:      r.FormValue("bookId"),
			Version:     version,
			Description: r.FormValue("description"),
			Files:       map[string]app.EvalUploadedFile{},
		}
		for _, key := range []string{"chunks", "queries", "qrels", "predictions", "metadata", "embeddings", "run"} {
			file, header, err := r.FormFile(key)
			if err != nil {
				continue
			}
			data, readErr := io.ReadAll(io.LimitReader(file, 32<<20))
			file.Close()
			if readErr != nil {
				writeError(w, http.StatusBadRequest, "failed to read upload")
				return
			}
			input.Files[key] = app.EvalUploadedFile{
				Filename:    header.Filename,
				ContentType: header.Header.Get("Content-Type"),
				Data:        data,
			}
		}
		item, err := s.app.AdminCreateEvalDataset(user, input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAdminEvalDatasetByID(w http.ResponseWriter, r *http.Request, _ domain.User) {
	path := strings.TrimPrefix(r.URL.Path, "/auth/admin/evals/datasets/")
	id := strings.Trim(path, "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := s.app.AdminGetEvalDataset(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPatch:
		var req adminEvalDatasetUpdateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		var status *domain.EvalDatasetStatus
		if strings.TrimSpace(req.Status) != "" {
			parsed := domain.EvalDatasetStatus(strings.TrimSpace(req.Status))
			status = &parsed
		}
		item, err := s.app.AdminUpdateEvalDataset(id, app.EvalDatasetUpdateInput{
			Name:        req.Name,
			Description: req.Description,
			Status:      status,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := s.app.AdminDeleteEvalDataset(id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAdminEvalRuns(w http.ResponseWriter, r *http.Request, user domain.User) {
	switch r.Method {
	case http.MethodGet:
		page, pageSize, err := parsePageParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		items, total, err := s.app.AdminListEvalRuns(store.EvalRunListOptions{
			DatasetID:     strings.TrimSpace(r.URL.Query().Get("datasetId")),
			Status:        strings.TrimSpace(r.URL.Query().Get("status")),
			Mode:          strings.TrimSpace(r.URL.Query().Get("mode")),
			RetrievalMode: strings.TrimSpace(r.URL.Query().Get("retrievalMode")),
			Page:          page,
			PageSize:      pageSize,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		totalPages := 0
		if pageSize > 0 {
			totalPages = (total + pageSize - 1) / pageSize
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items":      items,
			"count":      len(items),
			"page":       page,
			"pageSize":   pageSize,
			"total":      total,
			"totalPages": totalPages,
		})
	case http.MethodPost:
		var req adminEvalRunCreateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		run, err := s.app.AdminCreateEvalRun(user, app.EvalRunCreateInput{
			DatasetID:     req.DatasetID,
			Mode:          domain.EvalRunMode(strings.TrimSpace(req.Mode)),
			RetrievalMode: domain.EvalRetrievalMode(strings.TrimSpace(req.RetrievalMode)),
			GateMode:      strings.TrimSpace(req.GateMode),
			Params:        req.Params,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, run)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAdminEvalRunByID(w http.ResponseWriter, r *http.Request, _ domain.User) {
	path := strings.TrimPrefix(r.URL.Path, "/auth/admin/evals/runs/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	parts := strings.Split(path, "/")
	id := strings.TrimSpace(parts[0])
	if id == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		item, err := s.app.AdminGetEvalRun(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 2 && parts[1] == "cancel" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		item, err := s.app.AdminCancelEvalRun(id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
		return
	}
	if len(parts) == 2 && parts[1] == "per-query" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		rows, err := s.app.AdminGetEvalPerQuery(id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": rows, "count": len(rows)})
		return
	}
	if len(parts) == 3 && parts[1] == "artifacts" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		path, artifact, err := s.app.AdminGetEvalArtifactPath(id, parts[2])
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if artifact.ContentType != "" {
			w.Header().Set("Content-Type", artifact.ContentType)
		}
		filename := artifact.Name
		if ext := filepath.Ext(filename); ext != "" && artifact.ContentType == "" {
			if guessed := mime.TypeByExtension(ext); guessed != "" {
				w.Header().Set("Content-Type", guessed)
			}
		}
		w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(filename))
		http.ServeFile(w, r, path)
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

type adminEvalDatasetUpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      string  `json:"status,omitempty"`
}

type adminEvalRunCreateRequest struct {
	DatasetID     string         `json:"datasetId"`
	Mode          string         `json:"mode"`
	RetrievalMode string         `json:"retrievalMode"`
	GateMode      string         `json:"gateMode"`
	Params        map[string]any `json:"params,omitempty"`
}
