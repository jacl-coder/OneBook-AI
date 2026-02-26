package chatclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	Code    string
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

func (c *Client) AskQuestion(requestID, token, conversationID, bookID, question string) (domain.Answer, error) {
	payload := chatRequest{ConversationID: strings.TrimSpace(conversationID), BookID: bookID, Question: question}
	data, err := json.Marshal(payload)
	if err != nil {
		return domain.Answer{}, err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/chats", bytes.NewReader(data))
	if err != nil {
		return domain.Answer{}, err
	}
	addAuthHeader(req, token)
	addRequestIDHeader(req, requestID)
	req.Header.Set("Content-Type", "application/json")

	var ans domain.Answer
	if err := c.do(req, &ans); err != nil {
		return domain.Answer{}, err
	}
	return ans, nil
}

func (c *Client) ListConversations(requestID, token string, limit int) ([]domain.Conversation, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", limit))
	}
	endpoint := c.baseURL + "/conversations"
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	addAuthHeader(req, token)
	addRequestIDHeader(req, requestID)

	var resp struct {
		Items []domain.Conversation `json:"items"`
	}
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) ListConversationMessages(requestID, token, conversationID string, limit int) ([]domain.Message, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", limit))
	}
	endpoint := fmt.Sprintf("%s/conversations/%s/messages", c.baseURL, url.PathEscape(strings.TrimSpace(conversationID)))
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	addAuthHeader(req, token)
	addRequestIDHeader(req, requestID)

	var resp struct {
		Items []domain.Message `json:"items"`
	}
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
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

func addAuthHeader(req *http.Request, token string) {
	if strings.TrimSpace(token) == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
}

func addRequestIDHeader(req *http.Request, requestID string) {
	if strings.TrimSpace(requestID) == "" {
		return
	}
	req.Header.Set("X-Request-Id", requestID)
}

type chatRequest struct {
	ConversationID string `json:"conversationId,omitempty"`
	BookID         string `json:"bookId"`
	Question       string `json:"question"`
}
