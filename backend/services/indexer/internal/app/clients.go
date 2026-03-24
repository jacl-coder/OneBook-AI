package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"onebookai/internal/servicetoken"
	"onebookai/pkg/domain"
)

var ErrStaleBookGeneration = errors.New("stale book generation")

type bookClient struct {
	baseURL    string
	signer     *servicetoken.Signer
	httpClient *http.Client
}

func newBookClient(baseURL string, signer *servicetoken.Signer) *bookClient {
	return &bookClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		signer:     signer,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *bookClient) UpdateStatus(ctx context.Context, bookID string, generation int64, status domain.BookStatus, errMsg string) error {
	payload, err := json.Marshal(map[string]any{
		"status":       string(status),
		"errorMessage": errMsg,
		"generation":   generation,
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
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		var errResp struct {
			Code string `json:"code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		if strings.EqualFold(strings.TrimSpace(errResp.Code), "BOOK_STALE_GENERATION") {
			return ErrStaleBookGeneration
		}
	}
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
	return nil
}
