package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ingestClient interface {
	Enqueue(bookID string) error
}

type httpIngestClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func newIngestClient(baseURL, token string) *httpIngestClient {
	return &httpIngestClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *httpIngestClient) Enqueue(bookID string) error {
	payload, err := json.Marshal(map[string]string{"bookId": bookID})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/ingest/jobs", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", c.token)

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
		return fmt.Errorf("ingest error: %s", msg)
	}
	return nil
}
