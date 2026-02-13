package bookclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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

func (c *Client) UploadBook(token, filename string, r io.Reader) (domain.Book, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return domain.Book{}, err
	}
	if _, err := io.Copy(part, r); err != nil {
		return domain.Book{}, err
	}
	if err := writer.Close(); err != nil {
		return domain.Book{}, err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/books", body)
	if err != nil {
		return domain.Book{}, err
	}
	addAuthHeader(req, token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var book domain.Book
	if err := c.do(req, &book); err != nil {
		return domain.Book{}, err
	}
	return book, nil
}

func (c *Client) ListBooks(token string) ([]domain.Book, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/books", nil)
	if err != nil {
		return nil, err
	}
	addAuthHeader(req, token)

	var resp listBooksResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) GetBook(token, id string) (domain.Book, error) {
	path := fmt.Sprintf("%s/books/%s", c.baseURL, id)
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return domain.Book{}, err
	}
	addAuthHeader(req, token)

	var book domain.Book
	if err := c.do(req, &book); err != nil {
		return domain.Book{}, err
	}
	return book, nil
}

func (c *Client) DeleteBook(token, id string) error {
	path := fmt.Sprintf("%s/books/%s", c.baseURL, id)
	req, err := http.NewRequest(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	addAuthHeader(req, token)
	return c.do(req, nil)
}

// DownloadResponse contains pre-signed URL and filename for download.
type DownloadResponse struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

// GetDownloadURL returns a pre-signed download URL for the book file.
func (c *Client) GetDownloadURL(token, id string) (DownloadResponse, error) {
	path := fmt.Sprintf("%s/books/%s/download", c.baseURL, id)
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return DownloadResponse{}, err
	}
	addAuthHeader(req, token)

	var resp DownloadResponse
	if err := c.do(req, &resp); err != nil {
		return DownloadResponse{}, err
	}
	return resp, nil
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

type listBooksResponse struct {
	Items []domain.Book `json:"items"`
	Count int           `json:"count"`
}
