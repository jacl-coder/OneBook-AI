package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"onebookai/internal/servicetoken"
)

type ingestClient interface {
	Enqueue(bookID string) error
}

type httpIngestClient struct {
	baseURL    string
	signer     *servicetoken.Signer
	httpClient *http.Client
}

func newIngestClient(baseURL string, signer *servicetoken.Signer) (*httpIngestClient, error) {
	if signer == nil {
		return nil, fmt.Errorf("internal signer is required")
	}
	return &httpIngestClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		signer:     signer,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
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
	token, err := c.signer.Sign("ingest")
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
		return fmt.Errorf("ingest error: %s", msg)
	}
	return nil
}
