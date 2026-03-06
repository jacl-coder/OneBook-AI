package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"onebookai/internal/util"
	"onebookai/services/gateway/internal/authclient"
)

func (s *Server) handleAdminEvalOverview(w http.ResponseWriter, r *http.Request, ctx authContext) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, r)
		return
	}
	overview, err := s.auth.AdminEvalOverview(util.RequestIDFromRequest(r), ctx.AccessToken)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (s *Server) handleAdminEvalDatasets(w http.ResponseWriter, r *http.Request, ctx authContext) {
	switch r.Method {
	case http.MethodGet:
		page, pageSize, err := parseAdminPageParams(r)
		if err != nil {
			writeErrorWithCode(w, r, http.StatusBadRequest, err.Error(), "ADMIN_PAGINATION_INVALID")
			return
		}
		resp, err := s.auth.AdminListEvalDatasets(util.RequestIDFromRequest(r), ctx.AccessToken, authclient.AdminListEvalDatasetsOptions{
			Query:      strings.TrimSpace(r.URL.Query().Get("query")),
			SourceType: strings.TrimSpace(r.URL.Query().Get("sourceType")),
			Status:     strings.TrimSpace(r.URL.Query().Get("status")),
			BookID:     strings.TrimSpace(r.URL.Query().Get("bookId")),
			Page:       page,
			PageSize:   pageSize,
		})
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	case http.MethodPost:
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			writeErrorWithCode(w, r, http.StatusBadRequest, "invalid multipart form", "ADMIN_EVAL_DATASET_INVALID")
			return
		}
		fields := map[string]string{
			"name":        strings.TrimSpace(r.FormValue("name")),
			"sourceType":  strings.TrimSpace(r.FormValue("sourceType")),
			"bookId":      strings.TrimSpace(r.FormValue("bookId")),
			"description": strings.TrimSpace(r.FormValue("description")),
			"version":     strings.TrimSpace(r.FormValue("version")),
		}
		files := map[string][]byte{}
		for _, key := range []string{"chunks", "queries", "qrels", "predictions", "metadata", "embeddings", "run"} {
			file, _, err := r.FormFile(key)
			if err != nil {
				continue
			}
			data, readErr := io.ReadAll(io.LimitReader(file, 32<<20))
			file.Close()
			if readErr != nil {
				writeErrorWithCode(w, r, http.StatusBadRequest, "failed to read upload", "ADMIN_EVAL_UPLOAD_READ_FAILED")
				return
			}
			files[key] = data
		}
		item, err := s.auth.AdminCreateEvalDatasetMultipart(util.RequestIDFromRequest(r), ctx.AccessToken, fields, files)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		_ = s.writeAdminAuditLog(r, ctx, authclient.AdminAuditLogCreateRequest{
			Action:     "admin.eval.dataset.create",
			TargetType: "eval_dataset",
			TargetID:   item.ID,
			After:      map[string]any{"name": item.Name, "sourceType": item.SourceType},
			RequestID:  util.RequestIDFromRequest(r),
		})
		writeJSON(w, http.StatusCreated, item)
	default:
		methodNotAllowed(w, r)
	}
}

func (s *Server) handleAdminEvalDatasetByID(w http.ResponseWriter, r *http.Request, ctx authContext) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/evals/datasets/")
	id := strings.Trim(path, "/")
	if id == "" || strings.Contains(id, "/") {
		writeErrorWithCode(w, r, http.StatusNotFound, "not found", "ADMIN_EVAL_DATASET_NOT_FOUND")
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := s.auth.AdminGetEvalDataset(util.RequestIDFromRequest(r), ctx.AccessToken, id)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodPatch:
		var req authclient.AdminEvalDatasetUpdateRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeErrorWithCode(w, r, http.StatusBadRequest, "invalid JSON body", "ADMIN_EVAL_DATASET_INVALID")
			return
		}
		item, err := s.auth.AdminUpdateEvalDataset(util.RequestIDFromRequest(r), ctx.AccessToken, id, req)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		_ = s.writeAdminAuditLog(r, ctx, authclient.AdminAuditLogCreateRequest{
			Action:     "admin.eval.dataset.update",
			TargetType: "eval_dataset",
			TargetID:   item.ID,
			After:      map[string]any{"name": item.Name, "status": item.Status},
			RequestID:  util.RequestIDFromRequest(r),
		})
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		if err := s.auth.AdminDeleteEvalDataset(util.RequestIDFromRequest(r), ctx.AccessToken, id); err != nil {
			writeAuthError(w, r, err)
			return
		}
		_ = s.writeAdminAuditLog(r, ctx, authclient.AdminAuditLogCreateRequest{
			Action:     "admin.eval.dataset.delete",
			TargetType: "eval_dataset",
			TargetID:   id,
			RequestID:  util.RequestIDFromRequest(r),
		})
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		methodNotAllowed(w, r)
	}
}

func (s *Server) handleAdminEvalRuns(w http.ResponseWriter, r *http.Request, ctx authContext) {
	switch r.Method {
	case http.MethodGet:
		page, pageSize, err := parseAdminPageParams(r)
		if err != nil {
			writeErrorWithCode(w, r, http.StatusBadRequest, err.Error(), "ADMIN_PAGINATION_INVALID")
			return
		}
		resp, err := s.auth.AdminListEvalRuns(util.RequestIDFromRequest(r), ctx.AccessToken, authclient.AdminListEvalRunsOptions{
			DatasetID:     strings.TrimSpace(r.URL.Query().Get("datasetId")),
			Status:        strings.TrimSpace(r.URL.Query().Get("status")),
			Mode:          strings.TrimSpace(r.URL.Query().Get("mode")),
			RetrievalMode: strings.TrimSpace(r.URL.Query().Get("retrievalMode")),
			Page:          page,
			PageSize:      pageSize,
		})
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	case http.MethodPost:
		var req authclient.AdminCreateEvalRunRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			writeErrorWithCode(w, r, http.StatusBadRequest, "invalid JSON body", "ADMIN_EVAL_RUN_INVALID")
			return
		}
		run, err := s.auth.AdminCreateEvalRun(util.RequestIDFromRequest(r), ctx.AccessToken, req)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		_ = s.writeAdminAuditLog(r, ctx, authclient.AdminAuditLogCreateRequest{
			Action:     "admin.eval.run.create",
			TargetType: "eval_run",
			TargetID:   run.ID,
			After:      map[string]any{"datasetId": run.DatasetID, "mode": run.Mode},
			RequestID:  util.RequestIDFromRequest(r),
		})
		writeJSON(w, http.StatusCreated, run)
	default:
		methodNotAllowed(w, r)
	}
}

func (s *Server) handleAdminEvalRunByID(w http.ResponseWriter, r *http.Request, ctx authContext) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/evals/runs/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeErrorWithCode(w, r, http.StatusNotFound, "not found", "ADMIN_EVAL_RUN_NOT_FOUND")
		return
	}
	parts := strings.Split(path, "/")
	id := strings.TrimSpace(parts[0])
	if id == "" {
		writeErrorWithCode(w, r, http.StatusNotFound, "not found", "ADMIN_EVAL_RUN_NOT_FOUND")
		return
	}
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, r)
			return
		}
		run, err := s.auth.AdminGetEvalRun(util.RequestIDFromRequest(r), ctx.AccessToken, id)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, run)
		return
	}
	switch {
	case len(parts) == 2 && parts[1] == "cancel":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, r)
			return
		}
		run, err := s.auth.AdminCancelEvalRun(util.RequestIDFromRequest(r), ctx.AccessToken, id)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		_ = s.writeAdminAuditLog(r, ctx, authclient.AdminAuditLogCreateRequest{
			Action:     "admin.eval.run.cancel",
			TargetType: "eval_run",
			TargetID:   run.ID,
			RequestID:  util.RequestIDFromRequest(r),
		})
		writeJSON(w, http.StatusOK, run)
	case len(parts) == 2 && parts[1] == "per-query":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, r)
			return
		}
		resp, err := s.auth.AdminGetEvalPerQuery(util.RequestIDFromRequest(r), ctx.AccessToken, id)
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	case len(parts) == 3 && parts[1] == "artifacts":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, r)
			return
		}
		data, contentType, err := s.auth.AdminDownloadEvalArtifact(util.RequestIDFromRequest(r), ctx.AccessToken, id, parts[2])
		if err != nil {
			writeAuthError(w, r, err)
			return
		}
		if strings.TrimSpace(contentType) != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(parts[2]))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		_ = s.writeAdminAuditLog(r, ctx, authclient.AdminAuditLogCreateRequest{
			Action:     "admin.eval.artifact.download",
			TargetType: "eval_run",
			TargetID:   id,
			After:      map[string]any{"artifact": parts[2], "downloadedAt": time.Now().UTC()},
			RequestID:  util.RequestIDFromRequest(r),
		})
	default:
		writeErrorWithCode(w, r, http.StatusNotFound, "not found", "ADMIN_EVAL_RUN_NOT_FOUND")
	}
}
