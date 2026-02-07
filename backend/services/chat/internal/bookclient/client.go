package bookclient

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"onebookai/pkg/domain"
)

// Client calls the book service over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// APIError represents a book service error response.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}

// NewClient constructs a book service client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// GetBook fetches a book by id using caller bearer token.
func (c *Client) GetBook(token, id string) (domain.Book, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/books/"+id, nil)
	if err != nil {
		return domain.Book{}, err
	}
	addAuthHeader(req, token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Book{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		msg := errResp.Error
		if msg == "" {
			msg = resp.Status
		}
		return domain.Book{}, &APIError{Status: resp.StatusCode, Message: msg}
	}
	var book domain.Book
	if err := json.NewDecoder(resp.Body).Decode(&book); err != nil {
		return domain.Book{}, err
	}
	return book, nil
}

func addAuthHeader(req *http.Request, token string) {
	if strings.TrimSpace(token) == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
}
