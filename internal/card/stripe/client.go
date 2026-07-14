// Package stripe is a minimal Stripe card processor client implementing the
// payment intents, captures, refunds, and status retrieval endpoints.
package stripe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Client is an HTTP client for the subset of the Stripe API used by the card
// adapter. The base URL and API key are configurable so tests can point the
// client at an httptest.Server.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// New constructs a Stripe client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// PaymentIntent represents the subset of the Stripe payment intent response
// the adapter needs.
type PaymentIntent struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Charge  string `json:"latest_charge"`
	Receipt string `json:"client_secret"`
}

// CaptureResponse is the response from capturing a charge.
type CaptureResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// RefundResponse is the response from creating a refund.
type RefundResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Charge string `json:"charge"`
}

// ErrorResponse is the Stripe error envelope.
type ErrorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
		Decline string `json:"decline_code"`
	} `json:"error"`
}

// CreatePaymentIntent creates a payment intent for the given amount (in
// minor units), currency, and idempotency key.
func (c *Client) CreatePaymentIntent(ctx context.Context, amount int64, currency, idempotencyKey string) (*PaymentIntent, error) {
	body := fmt.Sprintf("amount=%d&currency=%s&payment_method_types[]=card&confirm=true", amount, currency)
	resp, err := c.do(ctx, http.MethodPost, "/v1/payment_intents", idempotencyKey, []byte(body))
	if err != nil {
		return nil, err
	}
	if err := checkError(resp); err != nil {
		return nil, err
	}
	var pi PaymentIntent
	if err := json.Unmarshal(resp.body, &pi); err != nil {
		return nil, err
	}
	return &pi, nil
}

// CreateCapture captures a previously authorized charge.
func (c *Client) CreateCapture(ctx context.Context, chargeID string, amount int64, idempotencyKey string) (*CaptureResponse, error) {
	path := fmt.Sprintf("/v1/charges/%s/capture", chargeID)
	body := []byte(fmt.Sprintf("amount=%d", amount))
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

// CreateRefund creates a refund for a charge.
func (c *Client) CreateRefund(ctx context.Context, chargeID string, amount int64, idempotencyKey string) (*RefundResponse, error) {
	body := []byte(fmt.Sprintf("charge=%s&amount=%d", chargeID, amount))
	resp, err := c.do(ctx, http.MethodPost, "/v1/refunds", idempotencyKey, body)
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

// GetCharge retrieves a charge by id (for status polling).
func (c *Client) GetCharge(ctx context.Context, chargeID string) (*CaptureResponse, error) {
	path := fmt.Sprintf("/v1/charges/%s", chargeID)
	resp, err := c.do(ctx, http.MethodGet, path, "", nil)
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

// DeclineCode returns the Stripe decline code from an error if present.
func DeclineCode(err error) string {
	if er, ok := err.(*APIError); ok {
		return er.Decline
	}
	return ""
}

// APIError is the error returned for non-2xx responses.
type APIError struct {
	Status  int
	Type    string
	Code    string
	Decline string
	Msg     string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("stripe api error: status=%d type=%s code=%s decline=%s msg=%s",
		e.Status, e.Type, e.Code, e.Decline, e.Msg)
}

type rawResp struct {
	status int
	body   []byte
}

func (c *Client) do(ctx context.Context, method, path, idempotencyKey string, body []byte) (*rawResp, error) {
	url := c.BaseURL + path
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

func checkError(r *rawResp) error {
	if r.status >= 200 && r.status < 300 {
		return nil
	}
	var er ErrorResponse
	_ = json.Unmarshal(r.body, &er)
	return &APIError{
		Status:  r.status,
		Type:    er.Error.Type,
		Code:    er.Error.Code,
		Decline: er.Error.Decline,
		Msg:     er.Error.Message,
	}
}

// AmountToMinorUnits converts a float64 major-unit amount to minor units
// (e.g. dollars to cents).
func AmountToMinorUnits(amount float64) int64 {
	return int64(amount*100 + 0.5)
}

// MinorUnitsToAmount converts minor units back to a float64 major-unit amount.
func MinorUnitsToAmount(minor int64) float64 {
	v, _ := strconv.ParseFloat(fmt.Sprintf("%d", minor), 64)
	return v / 100
}

// Authorize implements card.Processor. Returns the charge id
// (latest_charge) as the rail ref when present, falling back to the
// payment intent id.
func (c *Client) Authorize(ctx context.Context, ref string, amountMinor int64, currency, idemKey string) (string, string, error) {
	pi, err := c.CreatePaymentIntent(ctx, amountMinor, currency, idemKey)
	if err != nil {
		return "", "", err
	}
	railRef := pi.ID
	if pi.Charge != "" {
		railRef = pi.Charge
	}
	return railRef, pi.Status, nil
}

// Capture implements card.Processor.
func (c *Client) Capture(ctx context.Context, railRef string, amountMinor int64, currency, idemKey string) (string, error) {
	cr, err := c.doCapture(ctx, railRef, amountMinor, idemKey)
	if err != nil {
		return "", err
	}
	return cr.Status, nil
}

// Refund implements card.Processor.
func (c *Client) Refund(ctx context.Context, railRef string, amountMinor int64, currency, idemKey string) (string, error) {
	rr, err := c.doRefund(ctx, railRef, amountMinor, idemKey)
	if err != nil {
		return "", err
	}
	return rr.Status, nil
}

// Status implements card.Processor.
func (c *Client) Status(ctx context.Context, railRef string) (string, error) {
	cr, err := c.GetCharge(ctx, railRef)
	if err != nil {
		return "", err
	}
	return cr.Status, nil
}

// DeclineCode implements card.Processor.
func (c *Client) DeclineCode(err error) string {
	return DeclineCode(err)
}

// doCapture wraps CreateCapture to keep the Processor signature.
func (c *Client) doCapture(ctx context.Context, chargeID string, amount int64, idempotencyKey string) (*CaptureResponse, error) {
	return c.CreateCapture(ctx, chargeID, amount, idempotencyKey)
}

// doRefund wraps CreateRefund.
func (c *Client) doRefund(ctx context.Context, chargeID string, amount int64, idempotencyKey string) (*RefundResponse, error) {
	return c.CreateRefund(ctx, chargeID, amount, idempotencyKey)
}
