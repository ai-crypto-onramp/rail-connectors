// Package spi is a minimal HTTP client for the Banco Central do Brasil SPI
// (Sistema de Pagamentos Instantâneos) PIX API: DICT key resolution, payment
// initiation, status, and refund.
package spi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

// Client is the SPI client.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// New constructs a SPI client.
func New(baseURL, apiKey string) *Client {
	return &Client{BaseURL: baseURL, APIKey: apiKey, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// DICTResponse is the DICT key resolution response.
type DICTResponse struct {
	Key      string `json:"key"`
	Account  string `json:"account"`
	BankCode string `json:"bank_code"`
	Owner    string `json:"owner_name"`
	Document string `json:"document"`
}

// ResolveDICT resolves a PIX key via the DICT endpoint.
func (c *Client) ResolveDICT(ctx context.Context, key string) (*DICTResponse, error) {
	url := fmt.Sprintf("%s/v1/pix/dict/%s", c.BaseURL, key)
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
		return nil, &APIError{Status: resp.StatusCode, Code: "KEY_NOT_FOUND", Msg: string(data)}
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{Status: resp.StatusCode, Code: parseCode(data), Msg: string(data)}
	}
	var dr DICTResponse
	if err := json.Unmarshal(data, &dr); err != nil {
		return nil, err
	}
	return &dr, nil
}

// PaymentResponse is the payment initiation response.
type PaymentResponse struct {
	PaymentID  string `json:"payment_id"`
	Status     string `json:"status"`
	ReturnCode string `json:"return_code,omitempty"`
}

// InitiatePayment starts a PIX instant payment.
func (c *Client) InitiatePayment(ctx context.Context, idemKey, endToEndID, debtorDocument, creditorAccount, creditorBank string, amount decimal.Decimal, currency string) (*PaymentResponse, error) {
	body := map[string]any{
		"end_to_end_id":      endToEndID,
		"debtor_document":    debtorDocument,
		"creditor_account":   creditorAccount,
		"creditor_bank_code": creditorBank,
		"amount":             amount.String(),
		"currency":           currency,
	}
	resp, err := c.doPost(ctx, "/v1/pix/payments", idemKey, body)
	if err != nil {
		return nil, err
	}
	var pr PaymentResponse
	if err := json.Unmarshal(resp, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// GetPayment retrieves a payment status.
func (c *Client) GetPayment(ctx context.Context, paymentID string) (*PaymentResponse, error) {
	url := fmt.Sprintf("%s/v1/pix/payments/%s", c.BaseURL, paymentID)
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
		return nil, &APIError{Status: resp.StatusCode, Code: "PAYMENT_NOT_FOUND", Msg: string(data)}
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{Status: resp.StatusCode, Code: parseCode(data), Msg: string(data)}
	}
	var pr PaymentResponse
	if err := json.Unmarshal(data, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// RefundPayment initiates a PIX refund.
func (c *Client) RefundPayment(ctx context.Context, idemKey, originalPaymentID string, amount decimal.Decimal, currency string) (*PaymentResponse, error) {
	body := map[string]any{
		"original_payment_id": originalPaymentID,
		"amount":              amount.String(),
		"currency":            currency,
	}
	resp, err := c.doPost(ctx, "/v1/pix/refunds", idemKey, body)
	if err != nil {
		return nil, err
	}
	var pr PaymentResponse
	if err := json.Unmarshal(resp, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
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
	return fmt.Sprintf("spi error: status=%d code=%s msg=%s", e.Status, e.Code, e.Msg)
}

// DeclineCode returns the SPI return code from an error if present.
func DeclineCode(err error) string {
	if ae, ok := err.(*APIError); ok {
		return ae.Code
	}
	return ""
}

func parseCode(data []byte) string {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err == nil {
		if c, ok := m["return_code"]; ok {
			return fmt.Sprintf("%v", c)
		}
		if c, ok := m["code"]; ok {
			return fmt.Sprintf("%v", c)
		}
	}
	return ""
}
