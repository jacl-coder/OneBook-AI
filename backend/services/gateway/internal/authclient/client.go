package authclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func (c *Client) SignUp(email, password string) (domain.User, string, string, error) {
	payload := map[string]string{"email": email, "password": password}
	var resp authResponse
	if err := c.doJSON(http.MethodPost, "/auth/signup", "", payload, &resp); err != nil {
		return domain.User{}, "", "", err
	}
	return resp.User, resp.Token, resp.RefreshToken, nil
}

func (c *Client) Login(email, password string) (domain.User, string, string, error) {
	payload := map[string]string{"email": email, "password": password}
	var resp authResponse
	if err := c.doJSON(http.MethodPost, "/auth/login", "", payload, &resp); err != nil {
		return domain.User{}, "", "", err
	}
	return resp.User, resp.Token, resp.RefreshToken, nil
}

func (c *Client) Refresh(refreshToken string) (domain.User, string, string, error) {
	payload := map[string]string{"refreshToken": refreshToken}
	var resp authResponse
	if err := c.doJSON(http.MethodPost, "/auth/refresh", "", payload, &resp); err != nil {
		return domain.User{}, "", "", err
	}
	return resp.User, resp.Token, resp.RefreshToken, nil
}

func (c *Client) Logout(token, refreshToken string) error {
	var payload any
	if strings.TrimSpace(refreshToken) != "" {
		payload = map[string]string{"refreshToken": refreshToken}
	}
	return c.doJSON(http.MethodPost, "/auth/logout", token, payload, nil)
}

func (c *Client) Me(token string) (domain.User, error) {
	var user domain.User
	if err := c.doJSON(http.MethodGet, "/auth/me", token, nil, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (c *Client) UpdateMe(token, email string) (domain.User, error) {
	payload := map[string]string{"email": email}
	var user domain.User
	if err := c.doJSON(http.MethodPatch, "/auth/me", token, payload, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (c *Client) ChangePassword(token, currentPassword, newPassword string) error {
	payload := map[string]string{
		"currentPassword": currentPassword,
		"newPassword":     newPassword,
	}
	return c.doJSON(http.MethodPost, "/auth/me/password", token, payload, nil)
}

func (c *Client) JWKS() ([]store.JWK, error) {
	var resp jwksResponse
	if err := c.doJSON(http.MethodGet, "/auth/jwks", "", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Keys, nil
}

func (c *Client) AdminListUsers(token string) ([]domain.User, error) {
	var resp listUsersResponse
	if err := c.doJSON(http.MethodGet, "/auth/admin/users", token, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) AdminUpdateUser(token, userID string, role *domain.UserRole, status *domain.UserStatus) (domain.User, error) {
	payload := map[string]string{}
	if role != nil {
		payload["role"] = string(*role)
	}
	if status != nil {
		payload["status"] = string(*status)
	}
	var user domain.User
	path := fmt.Sprintf("/auth/admin/users/%s", userID)
	if err := c.doJSON(http.MethodPatch, path, token, payload, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (c *Client) doJSON(method, path, token string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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

type authResponse struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refreshToken"`
	User         domain.User `json:"user"`
}

type listUsersResponse struct {
	Items []domain.User `json:"items"`
	Count int           `json:"count"`
}

type jwksResponse struct {
	Keys []store.JWK `json:"keys"`
}
