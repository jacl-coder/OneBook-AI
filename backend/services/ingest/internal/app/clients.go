package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"onebookai/internal/servicetoken"
	"onebookai/pkg/domain"
)

type bookClient struct {
	baseURL    string
	signer     *servicetoken.Signer
	httpClient *http.Client
}

type bookFile struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
}

func newBookClient(baseURL string, signer *servicetoken.Signer) *bookClient {
	return &bookClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		signer:     signer,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *bookClient) FetchFile(ctx context.Context, bookID string) (bookFile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/internal/books/%s/file", c.baseURL, bookID), nil)
	if err != nil {
		return bookFile{}, err
	}
	token, err := c.signer.Sign("book")
	if err != nil {
		return bookFile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	var resp bookFile
	if err := c.do(req, &resp); err != nil {
		return bookFile{}, err
	}
	if resp.URL == "" {
		return bookFile{}, fmt.Errorf("empty download url")
	}
	return resp, nil
}

func (c *bookClient) UpdateStatus(ctx context.Context, bookID string, status domain.BookStatus, errMsg string) error {
	payload, err := json.Marshal(map[string]string{
		"status":       string(status),
		"errorMessage": errMsg,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, fmt.Sprintf("%s/internal/books/%s/status", c.baseURL, bookID), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	token, err := c.signer.Sign("book")
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return c.do(req, nil)
}

func (c *bookClient) do(req *http.Request, out any) error {
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
		return fmt.Errorf("book service error: %s", msg)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

type indexerClient struct {
	baseURL    string
	signer     *servicetoken.Signer
	httpClient *http.Client
}

func newIndexerClient(baseURL string, signer *servicetoken.Signer) *indexerClient {
	return &indexerClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		signer:     signer,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *indexerClient) Enqueue(ctx context.Context, bookID string) error {
	payload, err := json.Marshal(map[string]string{"bookId": bookID})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/indexer/jobs", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	token, err := c.signer.Sign("indexer")
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
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
		return fmt.Errorf("indexer service error: %s", msg)
	}
	return nil
}
