// Package bankapi submits NACHA files to the bank partner API and queries
// batch status. The partner base URL is configured via
// RAIL_ACH_PARTNER_URL.
package bankapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client submits NACHA batches to the bank partner and polls their status.
type Client struct {
	// BaseURL is the value of RAIL_ACH_PARTNER_URL.
	BaseURL string
	// HTTPClient allows injection for tests; defaults to http.DefaultClient.
	HTTPClient *http.Client
}

// SubmitResponse is the partner's response to a NACHA submission.
type SubmitResponse struct {
	BatchID    string `json:"batch_id"`
	Status     string `json:"status"`
	RailRef    string `json:"rail_ref"`
	StatusCode string `json:"status_code"`
}

// StatusResponse is the partner's batch status query response.
type StatusResponse struct {
	BatchID    string `json:"batch_id"`
	Status     string `json:"status"`
	StatusCode string `json:"status_code"`
	ReturnCode string `json:"return_code"`
}

// Submit posts a NACHA file to the partner's /batches endpoint with the
// supplied idempotency key.
func (c *Client) Submit(ctx context.Context, idempotencyKey, nacha string) (*SubmitResponse, error) {
	if c.BaseURL == "" {
		return nil, fmt.Errorf("bankapi: RAIL_ACH_PARTNER_URL is empty")
	}
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	url := strings.TrimRight(c.BaseURL, "/") + "/batches"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(nacha))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Idempotency-Key", idempotencyKey)

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("bankapi: submit failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("bankapi: submit rejected: status=%d body=%s", resp.StatusCode, string(body))
	}
	var out SubmitResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("bankapi: cannot decode submit response: %w", err)
	}
	if out.StatusCode == "" {
		out.StatusCode = out.Status
	}
	return &out, nil
}

// GetStatus queries the partner's /batches/{id} endpoint for batch status.
func (c *Client) GetStatus(ctx context.Context, batchID string) (*StatusResponse, error) {
	if c.BaseURL == "" {
		return nil, fmt.Errorf("bankapi: RAIL_ACH_PARTNER_URL is empty")
	}
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	url := strings.TrimRight(c.BaseURL, "/") + "/batches/" + batchID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("bankapi: status query failed: status=%d", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("bankapi: status query rejected: status=%d", resp.StatusCode)
	}
	var out StatusResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("bankapi: cannot decode status response: %w", err)
	}
	if out.StatusCode == "" {
		out.StatusCode = out.Status
	}
	return &out, nil
}

// New constructs a Client with a default timeout.
func New(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}
