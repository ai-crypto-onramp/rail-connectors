// Package bankapi is a minimal HTTP client for submitting NACHA files to a
// bank partner API and querying batch status. The BaseURL is configurable
// (RAIL_ACH_PARTNER_URL) so tests can point at an httptest.Server.
package bankapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the bank partner API client.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// New constructs a bankapi Client.
func New(baseURL, apiKey string) *Client {
	return &Client{BaseURL: baseURL, APIKey: apiKey, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// SubmitResponse is the response from submitting a NACHA file.
type SubmitResponse struct {
	BatchID   string `json:"batch_id"`
	Status    string `json:"status"`
	Accepted  bool   `json:"accepted"`
	Rejected  bool   `json:"rejected"`
	RejectMsg string `json:"reject_message,omitempty"`
}

// SubmitNACHA submits a NACHA file body under the given idempotency key.
func (c *Client) SubmitNACHA(ctx context.Context, fileBody []byte, idempotencyKey string) (*SubmitResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/ach/batches", bytes.NewReader(fileBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-API-Key", c.APIKey)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 500 {
		return nil, &APIError{Status: resp.StatusCode, Msg: string(data)}
	}
	if resp.StatusCode >= 400 {
		var sr SubmitResponse
		_ = json.Unmarshal(data, &sr)
		sr.Rejected = true
		if sr.RejectMsg == "" {
			sr.RejectMsg = string(data)
		}
		return &sr, nil
	}
	var sr SubmitResponse
	if err := json.Unmarshal(data, &sr); err != nil {
		return nil, err
	}
	sr.Accepted = true
	return &sr, nil
}

// BatchStatusResponse is the response from querying batch status.
type BatchStatusResponse struct {
	BatchID    string `json:"batch_id"`
	Status     string `json:"status"`
	ReturnCode string `json:"return_code,omitempty"`
}

// GetBatchStatus queries the status of a submitted batch.
func (c *Client) GetBatchStatus(ctx context.Context, batchID string) (*BatchStatusResponse, error) {
	url := fmt.Sprintf("%s/v1/ach/batches/%s", c.BaseURL, batchID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 500 {
		return nil, &APIError{Status: resp.StatusCode, Msg: string(data)}
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, &APIError{Status: resp.StatusCode, Code: "BATCH_NOT_FOUND", Msg: string(data)}
	}
	var bs BatchStatusResponse
	if err := json.Unmarshal(data, &bs); err != nil {
		return nil, err
	}
	return &bs, nil
}

// APIError is the error returned for 5xx / non-JSON responses.
type APIError struct {
	Status int
	Code   string
	Msg    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bankapi error: status=%d code=%s msg=%s", e.Status, e.Code, e.Msg)
}
