package chatclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"onebookai/pkg/domain"
)

// Client calls the chat service over HTTP.
type Client struct {
	baseURL          string
	httpClient       *http.Client
	streamHTTPClient *http.Client
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
		baseURL:          strings.TrimRight(baseURL, "/"),
		httpClient:       &http.Client{Timeout: 120 * time.Second},
		streamHTTPClient: &http.Client{},
	}
}

func (c *Client) AskQuestion(requestID, token, idempotencyKey, conversationID, bookID, question string, debug bool) (domain.Answer, bool, error) {
	payload := chatRequest{ConversationID: strings.TrimSpace(conversationID), BookID: bookID, Question: question, Debug: debug}
	data, err := json.Marshal(payload)
	if err != nil {
		return domain.Answer{}, false, err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/chats", bytes.NewReader(data))
	if err != nil {
		return domain.Answer{}, false, err
	}
	addAuthHeader(req, token)
	addRequestIDHeader(req, requestID)
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(idempotencyKey) != "" {
		req.Header.Set("Idempotency-Key", strings.TrimSpace(idempotencyKey))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Answer{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		msg := strings.TrimSpace(errResp.Error)
		if msg == "" {
			msg = resp.Status
		}
		return domain.Answer{}, false, &APIError{Status: resp.StatusCode, Message: msg, Code: strings.TrimSpace(errResp.Code)}
	}
	var ans domain.Answer
	if err := json.NewDecoder(resp.Body).Decode(&ans); err != nil {
		return domain.Answer{}, false, err
	}
	replayed := strings.EqualFold(strings.TrimSpace(resp.Header.Get("Idempotency-Replayed")), "true")
	return ans, replayed, nil
}

type StreamResponse struct {
	Body     io.ReadCloser
	Replayed bool
}

func (c *Client) StreamQuestion(
	ctx context.Context,
	requestID string,
	token string,
	idempotencyKey string,
	conversationID string,
	bookID string,
	question string,
	debug bool,
) (*StreamResponse, error) {
	payload := chatRequest{ConversationID: strings.TrimSpace(conversationID), BookID: bookID, Question: question, Debug: debug}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chats", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	addAuthHeader(req, token)
	addRequestIDHeader(req, requestID)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if strings.TrimSpace(idempotencyKey) != "" {
		req.Header.Set("Idempotency-Key", strings.TrimSpace(idempotencyKey))
	}
	resp, err := c.streamHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		var errResp struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		msg := strings.TrimSpace(errResp.Error)
		if msg == "" {
			msg = resp.Status
		}
		return nil, &APIError{Status: resp.StatusCode, Message: msg, Code: strings.TrimSpace(errResp.Code)}
	}
	return &StreamResponse{
		Body:     resp.Body,
		Replayed: strings.EqualFold(strings.TrimSpace(resp.Header.Get("Idempotency-Replayed")), "true"),
	}, nil
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
	Debug          bool   `json:"debug,omitempty"`
}
