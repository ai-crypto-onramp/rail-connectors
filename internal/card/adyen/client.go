// Package adyen is a minimal Adyen card processor client implementing the
// payments, captures, refunds, and payment retrieval endpoints.
package adyen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the subset of the Adyen API used by the card
// adapter. The BaseURL and APIKey are configurable so tests can point the
// client at an httptest.Server.
type Client struct {
	BaseURL   string
	APIKey   string
	Merchant string
	HTTP     *http.Client
}

// New constructs an Adyen client.
func New(baseURL, apiKey, merchant string) *Client {
	return &Client{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		Merchant:  merchant,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}
}

// PaymentResponse is the response from /payments.
type PaymentResponse struct {
	PSPReference    string `json:"pspReference"`
	ResultCode      string `json:"resultCode"`
	RefusalReason   string `json:"refusalReason"`
	RefusalReasonCode string `json:"refusalReasonCode"`
}

// CaptureResponse is the response from /payments/{id}/captures.
type CaptureResponse struct {
	PSPReference string `json:"pspReference"`
	Status       string `json:"status"`
}

// RefundResponse is the response from /payments/{id}/refunds.
type RefundResponse struct {
	PSPReference string `json:"pspReference"`
	Status       string `json:"status"`
}

// APIError is the error returned for non-2xx responses.
type APIError struct {
	Status int
	Code   string
	Msg    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("adyen api error: status=%d code=%s msg=%s", e.Status, e.Code, e.Msg)
}

// DeclineCode returns the refusal reason code from an error if present.
func DeclineCode(err error) string {
	if ae, ok := err.(*APIError); ok {
		return ae.Code
  }
	return ""
}

type amountReq struct {
	Value    int64  `json:"value"`
	Currency string `json:"currency"`
}

// GetPayment retrieves a payment by PSP reference (for status polling).
func (c *Client) GetPayment(ctx context.Context, pspRef string) (*PaymentResponse, error) {
	path := fmt.Sprintf("/payments/%s", pspRef)
	resp, err := c.do(ctx, http.MethodGet, path, "", nil)
	if err != nil {
		return nil, err
	}
	if err := checkError(resp); err != nil {
		return nil, err
	}
	var pr PaymentResponse
	if err := json.Unmarshal(resp.body, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

func (c *Client) postPayment(ctx context.Context, path, idempotencyKey string, body map[string]any) (*PaymentResponse, error) {
	resp, err := c.do(ctx, http.MethodPost, path, idempotencyKey, body)
	if err != nil {
		return nil, err
	}
	if err := checkError(resp); err != nil {
		return nil, err
	}
	var pr PaymentResponse
	if err := json.Unmarshal(resp.body, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

type rawResp struct {
	status int
	body   []byte
}

func (c *Client) do(ctx context.Context, method, path, idempotencyKey string, body any) (*rawResp, error) {
	url := c.BaseURL + path
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
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
	return &rawResp{status: resp.StatusCode, body: data}, nil
}

type errorEnvelope struct {
	Status    int    `json:"status"`
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}

func checkError(r *rawResp) error {
	if r.status >= 200 && r.status < 300 {
		return nil
	}
	var ee errorEnvelope
	_ = json.Unmarshal(r.body, &ee)
	return &APIError{Status: r.status, Code: ee.ErrorCode, Msg: ee.Message}
}

// Authorize implements card.Processor.
func (c *Client) Authorize(ctx context.Context, ref string, amountMinor int64, currency, idemKey string) (string, string, error) {
	pr, err := c.AuthorizePayment(ctx, ref, amountMinor, currency, idemKey)
	if err != nil {
		return "", "", err
	}
	return pr.PSPReference, pr.ResultCode, nil
}

// Capture implements card.Processor.
func (c *Client) Capture(ctx context.Context, railRef string, amountMinor int64, currency, idemKey string) (string, error) {
	cr, err := c.CapturePayment(ctx, railRef, amountMinor, currency, idemKey)
	if err != nil {
		return "", err
	}
	return cr.Status, nil
}

// Refund implements card.Processor.
func (c *Client) Refund(ctx context.Context, railRef string, amountMinor int64, currency, idemKey string) (string, error) {
	rr, err := c.RefundPayment(ctx, railRef, amountMinor, currency, idemKey)
	if err != nil {
		return "", err
	}
	return rr.Status, nil
}

// Status implements card.Processor.
func (c *Client) Status(ctx context.Context, railRef string) (string, error) {
	pr, err := c.GetPayment(ctx, railRef)
	if err != nil {
		return "", err
	}
	return pr.ResultCode, nil
}

// DeclineCode implements card.Processor.
func (c *Client) DeclineCode(err error) string {
	return DeclineCode(err)
}

// AuthorizePayment submits a payment to /payments.
func (c *Client) AuthorizePayment(ctx context.Context, reference string, amount int64, currency, idempotencyKey string) (*PaymentResponse, error) {
	body := map[string]any{
		"amount":          amountReq{Value: amount, Currency: currency},
		"merchantAccount": c.Merchant,
		"reference":       reference,
	}
	return c.postPayment(ctx, "/payments", idempotencyKey, body)
}

// CapturePayment submits a capture to /payments/{id}/captures.
func (c *Client) CapturePayment(ctx context.Context, pspRef string, amount int64, currency, idempotencyKey string) (*CaptureResponse, error) {
	path := fmt.Sprintf("/payments/%s/captures", pspRef)
	body := map[string]any{
		"amount":          amountReq{Value: amount, Currency: currency},
		"merchantAccount": c.Merchant,
	}
	resp, err := c.do(ctx, http.MethodPost, path, idempotencyKey, body)
	if err != nil {
		return nil, err
	}
	if err := checkError(resp); err != nil {
		return nil, err
	}
	var cr CaptureResponse
	if err := json.Unmarshal(resp.body, &cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

// RefundPayment submits a refund to /payments/{id}/refunds.
func (c *Client) RefundPayment(ctx context.Context, pspRef string, amount int64, currency, idempotencyKey string) (*RefundResponse, error) {
	path := fmt.Sprintf("/payments/%s/refunds", pspRef)
	body := map[string]any{
		"amount":          amountReq{Value: amount, Currency: currency},
		"merchantAccount": c.Merchant,
	}
	resp, err := c.do(ctx, http.MethodPost, path, idempotencyKey, body)
	if err != nil {
		return nil, err
	}
	if err := checkError(resp); err != nil {
		return nil, err
	}
	var rr RefundResponse
	if err := json.Unmarshal(resp.body, &rr); err != nil {
		return nil, err
	}
	return &rr, nil
}