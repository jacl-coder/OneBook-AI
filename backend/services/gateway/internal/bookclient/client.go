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

func (c *Client) UploadBook(user domain.User, filename string, r io.Reader) (domain.Book, error) {
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
	addUserHeaders(req, user)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var book domain.Book
	if err := c.do(req, &book); err != nil {
		return domain.Book{}, err
	}
	return book, nil
}

func (c *Client) ListBooks(user domain.User) ([]domain.Book, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/books", nil)
	if err != nil {
		return nil, err
	}
	addUserHeaders(req, user)

	var resp listBooksResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) GetBook(user domain.User, id string) (domain.Book, error) {
	path := fmt.Sprintf("%s/books/%s", c.baseURL, id)
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return domain.Book{}, err
	}
	addUserHeaders(req, user)

	var book domain.Book
	if err := c.do(req, &book); err != nil {
		return domain.Book{}, err
	}
	return book, nil
}

func (c *Client) DeleteBook(user domain.User, id string) error {
	path := fmt.Sprintf("%s/books/%s", c.baseURL, id)
	req, err := http.NewRequest(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	addUserHeaders(req, user)
	return c.do(req, nil)
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

type listBooksResponse struct {
	Items []domain.Book `json:"items"`
	Count int           `json:"count"`
}
