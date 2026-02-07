package authclient

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"onebookai/pkg/domain"
)

// Client calls the auth service over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient constructs an auth service client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// Me validates bearer token and returns current user.
func (c *Client) Me(token string) (domain.User, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/auth/me", nil)
	if err != nil {
		return domain.User{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.User{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return domain.User{}, &APIError{Status: resp.StatusCode, Message: resp.Status}
	}
	var user domain.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

// APIError represents an auth service error response.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}
