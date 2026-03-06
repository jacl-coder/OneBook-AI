package bookclient

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
)

// Client calls the book service over HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

type UploadBookRequest struct {
	Filename        string
	PrimaryCategory string
	Tags            []string
	Reader          io.Reader
}

type ListBooksParams struct {
	Query           string
	OwnerID         string
	Status          string
	PrimaryCategory string
	Tag             string
	Format          string
	Language        string
	SortBy          string
	SortOrder       string
}

type UpdateBookRequest struct {
	Title           string   `json:"title"`
	PrimaryCategory string   `json:"primaryCategory"`
	Tags            []string `json:"tags"`
}

// APIError represents a book service error response.
type APIError struct {
	Status  int
	Message string
	Code    string
}

func (e *APIError) Error() string {
	return e.Message
}

// NewClient constructs a book service client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) UploadBook(requestID, token, idempotencyKey string, payload UploadBookRequest) (domain.Book, bool, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", payload.Filename)
	if err != nil {
		return domain.Book{}, false, err
	}
	if _, err := io.Copy(part, payload.Reader); err != nil {
		return domain.Book{}, false, err
	}
	if strings.TrimSpace(payload.PrimaryCategory) != "" {
		if err := writer.WriteField("primaryCategory", strings.TrimSpace(payload.PrimaryCategory)); err != nil {
			return domain.Book{}, false, err
		}
	}
	for _, tag := range payload.Tags {
		if err := writer.WriteField("tags[]", strings.TrimSpace(tag)); err != nil {
			return domain.Book{}, false, err
		}
	}
	if err := writer.Close(); err != nil {
		return domain.Book{}, false, err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/books", body)
	if err != nil {
		return domain.Book{}, false, err
	}
	addAuthHeader(req, token)
	addRequestIDHeader(req, requestID)
	addIdempotencyKeyHeader(req, idempotencyKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var book domain.Book
	replayed, err := c.do(req, &book)
	if err != nil {
		return domain.Book{}, false, err
	}
	return book, replayed, nil
}

func (c *Client) ListBooks(requestID, token string, params ListBooksParams) ([]domain.Book, error) {
	reqURL, err := url.Parse(c.baseURL + "/books")
	if err != nil {
		return nil, err
	}
	query := reqURL.Query()
	setQueryValue(query, "query", params.Query)
	setQueryValue(query, "ownerId", params.OwnerID)
	setQueryValue(query, "status", params.Status)
	setQueryValue(query, "primaryCategory", params.PrimaryCategory)
	setQueryValue(query, "tag", params.Tag)
	setQueryValue(query, "format", params.Format)
	setQueryValue(query, "language", params.Language)
	setQueryValue(query, "sortBy", params.SortBy)
	setQueryValue(query, "sortOrder", params.SortOrder)
	reqURL.RawQuery = query.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, err
	}
	addAuthHeader(req, token)
	addRequestIDHeader(req, requestID)

	var resp listBooksResponse
	if _, err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) UpdateBook(requestID, token, id string, payload UpdateBookRequest) (domain.Book, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.Book{}, err
	}
	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/books/%s", c.baseURL, id), bytes.NewReader(body))
	if err != nil {
		return domain.Book{}, err
	}
	addAuthHeader(req, token)
	addRequestIDHeader(req, requestID)
	req.Header.Set("Content-Type", "application/json")
	var book domain.Book
	if _, err := c.do(req, &book); err != nil {
		return domain.Book{}, err
	}
	return book, nil
}

func (c *Client) GetBook(requestID, token, id string) (domain.Book, error) {
	path := fmt.Sprintf("%s/books/%s", c.baseURL, id)
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return domain.Book{}, err
	}
	addAuthHeader(req, token)
	addRequestIDHeader(req, requestID)

	var book domain.Book
	if _, err := c.do(req, &book); err != nil {
		return domain.Book{}, err
	}
	return book, nil
}

func (c *Client) DeleteBook(requestID, token, id string) error {
	path := fmt.Sprintf("%s/books/%s", c.baseURL, id)
	req, err := http.NewRequest(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	addAuthHeader(req, token)
	addRequestIDHeader(req, requestID)
	_, err = c.do(req, nil)
	return err
}

func (c *Client) ReprocessBook(requestID, token, idempotencyKey, id string) (domain.Book, bool, error) {
	path := fmt.Sprintf("%s/books/%s/reprocess", c.baseURL, id)
	req, err := http.NewRequest(http.MethodPost, path, bytes.NewReader([]byte("{}")))
	if err != nil {
		return domain.Book{}, false, err
	}
	addAuthHeader(req, token)
	addRequestIDHeader(req, requestID)
	addIdempotencyKeyHeader(req, idempotencyKey)
	req.Header.Set("Content-Type", "application/json")
	var book domain.Book
	replayed, err := c.do(req, &book)
	if err != nil {
		return domain.Book{}, false, err
	}
	return book, replayed, nil
}

// DownloadResponse contains pre-signed URL and filename for download.
type DownloadResponse struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

// GetDownloadURL returns a pre-signed download URL for the book file.
func (c *Client) GetDownloadURL(requestID, token, id string) (DownloadResponse, error) {
	path := fmt.Sprintf("%s/books/%s/download", c.baseURL, id)
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return DownloadResponse{}, err
	}
	addAuthHeader(req, token)
	addRequestIDHeader(req, requestID)

	var resp DownloadResponse
	if _, err := c.do(req, &resp); err != nil {
		return DownloadResponse{}, err
	}
	return resp, nil
}

func (c *Client) do(req *http.Request, out any) (bool, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	replayed := strings.EqualFold(strings.TrimSpace(resp.Header.Get("Idempotency-Replayed")), "true")
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
		return replayed, &APIError{Status: resp.StatusCode, Message: msg, Code: strings.TrimSpace(errResp.Code)}
	}
	if out == nil {
		return replayed, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return replayed, err
	}
	return replayed, nil
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

func addIdempotencyKeyHeader(req *http.Request, key string) {
	if strings.TrimSpace(key) == "" {
		return
	}
	req.Header.Set("Idempotency-Key", key)
}

func setQueryValue(query url.Values, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	query.Set(key, value)
}

type listBooksResponse struct {
	Items []domain.Book `json:"items"`
	Count int           `json:"count"`
}
