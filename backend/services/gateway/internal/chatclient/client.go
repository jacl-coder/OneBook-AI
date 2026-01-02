package chatclient

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"onebookai/pkg/domain"
)

// Client calls the chat service over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// APIError represents a chat service error response.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}

// NewClient constructs a chat service client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) AskQuestion(user domain.User, book domain.Book, question string) (domain.Answer, error) {
	payload := chatRequest{Book: book, Question: question}
	data, err := json.Marshal(payload)
	if err != nil {
		return domain.Answer{}, err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/chats", bytes.NewReader(data))
	if err != nil {
		return domain.Answer{}, err
	}
	addUserHeaders(req, user)
	req.Header.Set("Content-Type", "application/json")

	var ans domain.Answer
	if err := c.do(req, &ans); err != nil {
		return domain.Answer{}, err
	}
	return ans, nil
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
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
		return &APIError{Status: resp.StatusCode, Message: msg}
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

func addUserHeaders(req *http.Request, user domain.User) {
	req.Header.Set("X-User-Id", user.ID)
	req.Header.Set("X-User-Role", string(user.Role))
}

type chatRequest struct {
	Book     domain.Book `json:"book"`
	Question string      `json:"question"`
}
