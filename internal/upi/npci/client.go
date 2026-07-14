// Package npci is a minimal HTTP client for the NPCI UPI Collect / Intent
// APIs: collect request, status, refund, and dispute endpoints.
package npci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the NPCI UPI client.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// New constructs an NPCI client.
func New(baseURL, apiKey string) *Client {
	return &Client{BaseURL: baseURL, APIKey: apiKey, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// CollectResponse is the response from initiating a UPI Collect request.
type CollectResponse struct {
	CollectID    string `json:"collect_id"`
	Status       string `json:"status"`
	ResponseCode string `json:"response_code,omitempty"`
}

// InitiateCollect starts a UPI Collect request.
func (c *Client) InitiateCollect(ctx context.Context, idemKey, payerVPA, payeeVPA string, amount float64, currency, remark string) (*CollectResponse, error) {
	body := map[string]any{
		"payer_vpa": payerVPA,
		"payee_vpa": payeeVPA,
		"amount":    amount,
		"currency":  currency,
		"remark":    remark,
	}
	resp, err := c.doPost(ctx, "/v1/upi/collect", idemKey, body)
	if err != nil {
		return nil, err
	}
	var cr CollectResponse
	if err := json.Unmarshal(resp, &cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

// GetCollectStatus retrieves the status of a UPI Collect request.
func (c *Client) GetCollectStatus(ctx context.Context, collectID string) (*CollectResponse, error) {
	url := fmt.Sprintf("%s/v1/upi/collect/%s", c.BaseURL, collectID)
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
		return nil, &APIError{Status: resp.StatusCode, Code: "COLLECT_NOT_FOUND", Msg: string(data)}
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{Status: resp.StatusCode, Code: parseCode(data), Msg: string(data)}
	}
	var cr CollectResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

// RefundResponse is the response from a UPI refund.
type RefundResponse struct {
	RefundID     string `json:"refund_id"`
	Status       string `json:"status"`
	ResponseCode string `json:"response_code,omitempty"`
}

// Refund initiates a UPI refund.
func (c *Client) Refund(ctx context.Context, idemKey, originalCollectID string, amount float64, currency string) (*RefundResponse, error) {
	body := map[string]any{
		"original_collect_id": originalCollectID,
		"amount":              amount,
		"currency":            currency,
	}
	resp, err := c.doPost(ctx, "/v1/upi/refunds", idemKey, body)
	if err != nil {
		return nil, err
	}
	var rr RefundResponse
	if err := json.Unmarshal(resp, &rr); err != nil {
		return nil, err
	}
	return &rr, nil
}

// DisputeResponse is the response from recording a dispute.
type DisputeResponse struct {
	DisputeID  string `json:"dispute_id"`
	Status     string `json:"status"`
	ReasonCode string `json:"reason_code,omitempty"`
}

// RecordDispute records a chargeback / dispute.
func (c *Client) RecordDispute(ctx context.Context, idemKey, originalCollectID, reasonCode string, amount float64, currency string) (*DisputeResponse, error) {
	body := map[string]any{
		"original_collect_id": originalCollectID,
		"reason_code":         reasonCode,
		"amount":              amount,
		"currency":            currency,
	}
	resp, err := c.doPost(ctx, "/v1/upi/disputes", idemKey, body)
	if err != nil {
		return nil, err
	}
	var dr DisputeResponse
	if err := json.Unmarshal(resp, &dr); err != nil {
		return nil, err
	}
	return &dr, nil
}

func (c *Client) doPost(ctx context.Context, path, idemKey string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.APIKey)
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 500 {
		return nil, &APIError{Status: resp.StatusCode, Msg: string(out)}
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{Status: resp.StatusCode, Code: parseCode(out), Msg: string(out)}
	}
	return out, nil
}

// APIError is the error returned for non-2xx responses.
type APIError struct {
	Status int
	Code   string
	Msg    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("npci error: status=%d code=%s msg=%s", e.Status, e.Code, e.Msg)
}

// DeclineCode returns the NPCI response code from an error if present.
func DeclineCode(err error) string {
	if ae, ok := err.(*APIError); ok {
		return ae.Code
	}
	return ""
}

func parseCode(data []byte) string {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err == nil {
		if c, ok := m["response_code"]; ok {
			return fmt.Sprintf("%v", c)
		}
		if c, ok := m["code"]; ok {
			return fmt.Sprintf("%v", c)
		}
	}
	return ""
}
