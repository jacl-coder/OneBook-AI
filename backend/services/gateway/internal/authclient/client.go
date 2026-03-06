package authclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"onebookai/pkg/domain"
	"onebookai/pkg/store"
)

// Client calls the auth service over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// APIError represents an auth service error response.
type APIError struct {
	Status  int
	Message string
	Code    string
}

func (e *APIError) Error() string {
	return e.Message
}

// NewClient constructs an auth service client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) SignUp(requestID, email, password string) (domain.User, string, string, error) {
	payload := map[string]string{"email": email, "password": password}
	var resp authResponse
	if err := c.doJSON(http.MethodPost, "/auth/signup", requestID, "", payload, &resp); err != nil {
		return domain.User{}, "", "", err
	}
	return resp.User, resp.Token, resp.RefreshToken, nil
}

func (c *Client) Login(requestID, email, password string) (domain.User, string, string, error) {
	payload := map[string]string{"email": email, "password": password}
	var resp authResponse
	if err := c.doJSON(http.MethodPost, "/auth/login", requestID, "", payload, &resp); err != nil {
		return domain.User{}, "", "", err
	}
	return resp.User, resp.Token, resp.RefreshToken, nil
}

func (c *Client) LoginMethods(requestID, email string) (bool, error) {
	payload := map[string]string{"email": email}
	var resp LoginMethodsResponse
	if err := c.doJSON(http.MethodPost, "/auth/login/methods", requestID, "", payload, &resp); err != nil {
		return false, err
	}
	return resp.PasswordLogin, nil
}

func (c *Client) OTPSend(requestID, email, purpose string) (OTPSendResponse, error) {
	payload := map[string]string{"email": email, "purpose": purpose}
	var resp OTPSendResponse
	if err := c.doJSON(http.MethodPost, "/auth/otp/send", requestID, "", payload, &resp); err != nil {
		return OTPSendResponse{}, err
	}
	return resp, nil
}

func (c *Client) OTPVerify(requestID, challengeID, email, purpose, code, password string) (domain.User, string, string, error) {
	payload := map[string]string{
		"challengeId": challengeID,
		"email":       email,
		"purpose":     purpose,
		"code":        code,
	}
	if strings.TrimSpace(password) != "" {
		payload["password"] = password
	}
	var resp authResponse
	if err := c.doJSON(http.MethodPost, "/auth/otp/verify", requestID, "", payload, &resp); err != nil {
		return domain.User{}, "", "", err
	}
	return resp.User, resp.Token, resp.RefreshToken, nil
}

func (c *Client) PasswordResetVerify(requestID, challengeID, email, code string) (PasswordResetVerifyResponse, error) {
	payload := map[string]string{
		"challengeId": challengeID,
		"email":       email,
		"code":        code,
	}
	var resp PasswordResetVerifyResponse
	if err := c.doJSON(http.MethodPost, "/auth/password/reset/verify", requestID, "", payload, &resp); err != nil {
		return PasswordResetVerifyResponse{}, err
	}
	return resp, nil
}

func (c *Client) PasswordResetComplete(requestID, email, resetToken, newPassword string) error {
	payload := map[string]string{
		"email":       email,
		"resetToken":  resetToken,
		"newPassword": newPassword,
	}
	return c.doJSON(http.MethodPost, "/auth/password/reset/complete", requestID, "", payload, nil)
}

func (c *Client) Refresh(requestID, refreshToken string) (domain.User, string, string, error) {
	payload := map[string]string{"refreshToken": refreshToken}
	var resp authResponse
	if err := c.doJSON(http.MethodPost, "/auth/refresh", requestID, "", payload, &resp); err != nil {
		return domain.User{}, "", "", err
	}
	return resp.User, resp.Token, resp.RefreshToken, nil
}

func (c *Client) Logout(requestID, token, refreshToken string) error {
	var payload any
	if strings.TrimSpace(refreshToken) != "" {
		payload = map[string]string{"refreshToken": refreshToken}
	}
	return c.doJSON(http.MethodPost, "/auth/logout", requestID, token, payload, nil)
}

func (c *Client) Me(requestID, token string) (domain.User, error) {
	var user domain.User
	if err := c.doJSON(http.MethodGet, "/auth/me", requestID, token, nil, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (c *Client) UpdateMe(requestID, token, email string) (domain.User, error) {
	payload := map[string]string{"email": email}
	var user domain.User
	if err := c.doJSON(http.MethodPatch, "/auth/me", requestID, token, payload, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (c *Client) ChangePassword(requestID, token, currentPassword, newPassword string) error {
	payload := map[string]string{"newPassword": newPassword}
	if strings.TrimSpace(currentPassword) != "" {
		payload["currentPassword"] = currentPassword
	}
	return c.doJSON(http.MethodPost, "/auth/me/password", requestID, token, payload, nil)
}

func (c *Client) JWKS(requestID string) ([]store.JWK, error) {
	var resp jwksResponse
	if err := c.doJSON(http.MethodGet, "/auth/jwks", requestID, "", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Keys, nil
}

func (c *Client) AdminListUsers(requestID, token string) ([]domain.User, error) {
	resp, err := c.AdminListUsersWithOptions(requestID, token, AdminListUsersOptions{})
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}

type AdminListUsersOptions struct {
	Query     string
	Role      string
	Status    string
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

type PagedUsersResponse struct {
	Items      []domain.User `json:"items"`
	Count      int           `json:"count"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
	Total      int           `json:"total"`
	TotalPages int           `json:"totalPages"`
}

func (c *Client) AdminListUsersWithOptions(requestID, token string, opts AdminListUsersOptions) (PagedUsersResponse, error) {
	query := url.Values{}
	if v := strings.TrimSpace(opts.Query); v != "" {
		query.Set("query", v)
	}
	if v := strings.TrimSpace(opts.Role); v != "" {
		query.Set("role", v)
	}
	if v := strings.TrimSpace(opts.Status); v != "" {
		query.Set("status", v)
	}
	if opts.Page > 0 {
		query.Set("page", fmt.Sprintf("%d", opts.Page))
	}
	if opts.PageSize > 0 {
		query.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	}
	if v := strings.TrimSpace(opts.SortBy); v != "" {
		query.Set("sortBy", v)
	}
	if v := strings.TrimSpace(opts.SortOrder); v != "" {
		query.Set("sortOrder", v)
	}
	path := "/auth/admin/users"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var resp listUsersResponse
	if err := c.doJSON(http.MethodGet, path, requestID, token, nil, &resp); err != nil {
		return PagedUsersResponse{}, err
	}
	return PagedUsersResponse{
		Items:      resp.Items,
		Count:      resp.Count,
		Page:       resp.Page,
		PageSize:   resp.PageSize,
		Total:      resp.Total,
		TotalPages: resp.TotalPages,
	}, nil
}

func (c *Client) AdminGetUser(requestID, token, userID string) (domain.User, error) {
	var user domain.User
	path := fmt.Sprintf("/auth/admin/users/%s", userID)
	if err := c.doJSON(http.MethodGet, path, requestID, token, nil, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (c *Client) AdminUpdateUser(requestID, token, userID string, role *domain.UserRole, status *domain.UserStatus) (domain.User, error) {
	payload := map[string]string{}
	if role != nil {
		payload["role"] = string(*role)
	}
	if status != nil {
		payload["status"] = string(*status)
	}
	var user domain.User
	path := fmt.Sprintf("/auth/admin/users/%s", userID)
	if err := c.doJSON(http.MethodPatch, path, requestID, token, payload, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (c *Client) AdminDisableUser(requestID, token, userID string) (domain.User, error) {
	var user domain.User
	path := fmt.Sprintf("/auth/admin/users/%s/disable", userID)
	if err := c.doJSON(http.MethodPost, path, requestID, token, map[string]any{}, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (c *Client) AdminEnableUser(requestID, token, userID string) (domain.User, error) {
	var user domain.User
	path := fmt.Sprintf("/auth/admin/users/%s/enable", userID)
	if err := c.doJSON(http.MethodPost, path, requestID, token, map[string]any{}, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

type AdminAuditLogCreateRequest struct {
	Action     string         `json:"action"`
	TargetType string         `json:"targetType"`
	TargetID   string         `json:"targetId"`
	Before     map[string]any `json:"before,omitempty"`
	After      map[string]any `json:"after,omitempty"`
	RequestID  string         `json:"requestId,omitempty"`
	IP         string         `json:"ip,omitempty"`
	UserAgent  string         `json:"userAgent,omitempty"`
}

func (c *Client) AdminCreateAuditLog(requestID, token string, req AdminAuditLogCreateRequest) (domain.AdminAuditLog, error) {
	var out domain.AdminAuditLog
	if err := c.doJSON(http.MethodPost, "/auth/admin/audit-logs", requestID, token, req, &out); err != nil {
		return domain.AdminAuditLog{}, err
	}
	return out, nil
}

type AdminListAuditLogsOptions struct {
	ActorID    string
	Action     string
	TargetType string
	TargetID   string
	From       string
	To         string
	Page       int
	PageSize   int
}

type PagedAuditLogsResponse struct {
	Items      []domain.AdminAuditLog `json:"items"`
	Count      int                    `json:"count"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"pageSize"`
	Total      int                    `json:"total"`
	TotalPages int                    `json:"totalPages"`
}

func (c *Client) AdminListAuditLogs(requestID, token string, opts AdminListAuditLogsOptions) (PagedAuditLogsResponse, error) {
	query := url.Values{}
	if v := strings.TrimSpace(opts.ActorID); v != "" {
		query.Set("actorId", v)
	}
	if v := strings.TrimSpace(opts.Action); v != "" {
		query.Set("action", v)
	}
	if v := strings.TrimSpace(opts.TargetType); v != "" {
		query.Set("targetType", v)
	}
	if v := strings.TrimSpace(opts.TargetID); v != "" {
		query.Set("targetId", v)
	}
	if v := strings.TrimSpace(opts.From); v != "" {
		query.Set("from", v)
	}
	if v := strings.TrimSpace(opts.To); v != "" {
		query.Set("to", v)
	}
	if opts.Page > 0 {
		query.Set("page", fmt.Sprintf("%d", opts.Page))
	}
	if opts.PageSize > 0 {
		query.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	}
	path := "/auth/admin/audit-logs"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var resp PagedAuditLogsResponse
	if err := c.doJSON(http.MethodGet, path, requestID, token, nil, &resp); err != nil {
		return PagedAuditLogsResponse{}, err
	}
	return resp, nil
}

func (c *Client) AdminOverview(requestID, token string) (domain.AdminOverview, error) {
	var overview domain.AdminOverview
	if err := c.doJSON(http.MethodGet, "/auth/admin/overview", requestID, token, nil, &overview); err != nil {
		return domain.AdminOverview{}, err
	}
	return overview, nil
}

type AdminEvalOverview = domain.AdminEvalOverview

type PagedEvalDatasetsResponse struct {
	Items      []domain.EvalDataset `json:"items"`
	Count      int                  `json:"count"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"pageSize"`
	Total      int                  `json:"total"`
	TotalPages int                  `json:"totalPages"`
}

type AdminListEvalDatasetsOptions struct {
	Query      string
	SourceType string
	Status     string
	BookID     string
	Page       int
	PageSize   int
}

type AdminEvalDatasetUpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      string  `json:"status,omitempty"`
}

type PagedEvalRunsResponse struct {
	Items      []domain.EvalRun `json:"items"`
	Count      int              `json:"count"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	Total      int              `json:"total"`
	TotalPages int              `json:"totalPages"`
}

type AdminListEvalRunsOptions struct {
	DatasetID     string
	Status        string
	Mode          string
	RetrievalMode string
	Page          int
	PageSize      int
}

type AdminCreateEvalRunRequest struct {
	DatasetID     string         `json:"datasetId"`
	Mode          string         `json:"mode"`
	RetrievalMode string         `json:"retrievalMode"`
	GateMode      string         `json:"gateMode"`
	Params        map[string]any `json:"params,omitempty"`
}

func (c *Client) AdminEvalOverview(requestID, token string) (AdminEvalOverview, error) {
	var overview AdminEvalOverview
	if err := c.doJSON(http.MethodGet, "/auth/admin/evals/overview", requestID, token, nil, &overview); err != nil {
		return AdminEvalOverview{}, err
	}
	return overview, nil
}

func (c *Client) AdminListEvalDatasets(requestID, token string, opts AdminListEvalDatasetsOptions) (PagedEvalDatasetsResponse, error) {
	query := url.Values{}
	if v := strings.TrimSpace(opts.Query); v != "" {
		query.Set("query", v)
	}
	if v := strings.TrimSpace(opts.SourceType); v != "" {
		query.Set("sourceType", v)
	}
	if v := strings.TrimSpace(opts.Status); v != "" {
		query.Set("status", v)
	}
	if v := strings.TrimSpace(opts.BookID); v != "" {
		query.Set("bookId", v)
	}
	if opts.Page > 0 {
		query.Set("page", fmt.Sprintf("%d", opts.Page))
	}
	if opts.PageSize > 0 {
		query.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	}
	path := "/auth/admin/evals/datasets"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var resp PagedEvalDatasetsResponse
	if err := c.doJSON(http.MethodGet, path, requestID, token, nil, &resp); err != nil {
		return PagedEvalDatasetsResponse{}, err
	}
	return resp, nil
}

func (c *Client) AdminCreateEvalDatasetMultipart(requestID, token string, fields map[string]string, files map[string][]byte) (domain.EvalDataset, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if err := writer.WriteField(key, value); err != nil {
			return domain.EvalDataset{}, err
		}
	}
	for key, data := range files {
		if len(data) == 0 {
			continue
		}
		part, err := writer.CreateFormFile(key, key)
		if err != nil {
			return domain.EvalDataset{}, err
		}
		if _, err := part.Write(data); err != nil {
			return domain.EvalDataset{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return domain.EvalDataset{}, err
	}
	var out domain.EvalDataset
	if err := c.doBody(http.MethodPost, "/auth/admin/evals/datasets", requestID, token, &body, writer.FormDataContentType(), &out); err != nil {
		return domain.EvalDataset{}, err
	}
	return out, nil
}

func (c *Client) AdminGetEvalDataset(requestID, token, id string) (domain.EvalDataset, error) {
	var out domain.EvalDataset
	if err := c.doJSON(http.MethodGet, "/auth/admin/evals/datasets/"+id, requestID, token, nil, &out); err != nil {
		return domain.EvalDataset{}, err
	}
	return out, nil
}

func (c *Client) AdminUpdateEvalDataset(requestID, token, id string, req AdminEvalDatasetUpdateRequest) (domain.EvalDataset, error) {
	var out domain.EvalDataset
	if err := c.doJSON(http.MethodPatch, "/auth/admin/evals/datasets/"+id, requestID, token, req, &out); err != nil {
		return domain.EvalDataset{}, err
	}
	return out, nil
}

func (c *Client) AdminDeleteEvalDataset(requestID, token, id string) error {
	return c.doJSON(http.MethodDelete, "/auth/admin/evals/datasets/"+id, requestID, token, nil, nil)
}

func (c *Client) AdminListEvalRuns(requestID, token string, opts AdminListEvalRunsOptions) (PagedEvalRunsResponse, error) {
	query := url.Values{}
	if v := strings.TrimSpace(opts.DatasetID); v != "" {
		query.Set("datasetId", v)
	}
	if v := strings.TrimSpace(opts.Status); v != "" {
		query.Set("status", v)
	}
	if v := strings.TrimSpace(opts.Mode); v != "" {
		query.Set("mode", v)
	}
	if v := strings.TrimSpace(opts.RetrievalMode); v != "" {
		query.Set("retrievalMode", v)
	}
	if opts.Page > 0 {
		query.Set("page", fmt.Sprintf("%d", opts.Page))
	}
	if opts.PageSize > 0 {
		query.Set("pageSize", fmt.Sprintf("%d", opts.PageSize))
	}
	path := "/auth/admin/evals/runs"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var resp PagedEvalRunsResponse
	if err := c.doJSON(http.MethodGet, path, requestID, token, nil, &resp); err != nil {
		return PagedEvalRunsResponse{}, err
	}
	return resp, nil
}

func (c *Client) AdminCreateEvalRun(requestID, token string, req AdminCreateEvalRunRequest) (domain.EvalRun, error) {
	var out domain.EvalRun
	if err := c.doJSON(http.MethodPost, "/auth/admin/evals/runs", requestID, token, req, &out); err != nil {
		return domain.EvalRun{}, err
	}
	return out, nil
}

func (c *Client) AdminGetEvalRun(requestID, token, id string) (domain.EvalRun, error) {
	var out domain.EvalRun
	if err := c.doJSON(http.MethodGet, "/auth/admin/evals/runs/"+id, requestID, token, nil, &out); err != nil {
		return domain.EvalRun{}, err
	}
	return out, nil
}

func (c *Client) AdminCancelEvalRun(requestID, token, id string) (domain.EvalRun, error) {
	var out domain.EvalRun
	if err := c.doJSON(http.MethodPost, "/auth/admin/evals/runs/"+id+"/cancel", requestID, token, map[string]any{}, &out); err != nil {
		return domain.EvalRun{}, err
	}
	return out, nil
}

func (c *Client) AdminGetEvalPerQuery(requestID, token, id string) (map[string]any, error) {
	var out map[string]any
	if err := c.doJSON(http.MethodGet, "/auth/admin/evals/runs/"+id+"/per-query", requestID, token, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) AdminDownloadEvalArtifact(requestID, token, runID, name string) ([]byte, string, error) {
	path := "/auth/admin/evals/runs/" + runID + "/artifacts/" + name
	return c.doBytes(http.MethodGet, path, requestID, token)
}

func (c *Client) doJSON(method, path, requestID, token string, payload any, out any) error {
	var body io.Reader
	contentType := ""
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
		contentType = "application/json"
	}
	return c.doBody(method, path, requestID, token, body, contentType, out)
}

func (c *Client) doBody(method, path, requestID, token string, body io.Reader, contentType string, out any) error {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	if strings.TrimSpace(contentType) != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if strings.TrimSpace(requestID) != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		msg := errResp.Error
		if msg == "" {
			msg = resp.Status
		}
		return &APIError{Status: resp.StatusCode, Message: msg, Code: strings.TrimSpace(errResp.Code)}
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

func (c *Client) doBytes(method, path, requestID, token string) ([]byte, string, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if strings.TrimSpace(requestID) != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		msg := errResp.Error
		if msg == "" {
			msg = resp.Status
		}
		return nil, "", &APIError{Status: resp.StatusCode, Message: msg, Code: strings.TrimSpace(errResp.Code)}
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return data, resp.Header.Get("Content-Type"), nil
}

type authResponse struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refreshToken"`
	User         domain.User `json:"user"`
}

type OTPSendResponse struct {
	ChallengeID        string `json:"challengeId"`
	ExpiresInSeconds   int    `json:"expiresInSeconds"`
	ResendAfterSeconds int    `json:"resendAfterSeconds"`
	MaskedEmail        string `json:"maskedEmail,omitempty"`
}

type LoginMethodsResponse struct {
	PasswordLogin bool `json:"passwordLogin"`
}

type PasswordResetVerifyResponse struct {
	ResetToken       string `json:"resetToken"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
}

type listUsersResponse struct {
	Items      []domain.User `json:"items"`
	Count      int           `json:"count"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
	Total      int           `json:"total"`
	TotalPages int           `json:"totalPages"`
}

type jwksResponse struct {
	Keys []store.JWK `json:"keys"`
}
