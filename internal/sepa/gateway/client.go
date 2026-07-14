// Package gateway is a minimal HTTP client for the SEPA Instant gateway
// supporting mTLS (RAIL_SEPA_MTLS_CERT / RAIL_SEPA_MTLS_KEY).
package gateway

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client is the SEPA gateway client.
type Client struct {
	BaseURL  string
	APIKey   string
	MTLSCert string // path to PEM-encoded client cert
	MTLSKey  string // path to PEM-encoded client key
	HTTP     *http.Client
}

// New constructs a gateway Client. When certPath / keyPath are provided the
// client uses an mTLS-configured transport; otherwise it falls back to a
// plain HTTP client (so tests can point at an httptest.Server).
func New(baseURL, apiKey, certPath, keyPath string) (*Client, error) {
	c := &Client{BaseURL: baseURL, APIKey: apiKey, MTLSCert: certPath, MTLSKey: keyPath}
	if certPath != "" && keyPath != "" {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("gateway: load mTLS keypair: %w", err)
		}
		c.HTTP = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					Certificates: []tls.Certificate{cert},
					MinVersion:   tls.VersionTLS12,
				},
			},
		}
	} else {
		c.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	return c, nil
}

// SubmitPain001 posts a pain.001 XML body to the gateway under the given
// idempotency key and returns the raw response.
type SubmissionResponse struct {
	XMLName   xml.Name `xml:"Document"`
	MsgID     string   `xml:"CstmrCdtTrfInitn>GrpHdr>MsgId"`
	Status    string   `xml:"CstmrCdtTrfInitn>GrpHdr>Status,omitempty"`
	PaymentID string   `xml:"CstmrCdtTrfInitn>PmtInf>CdtTrfTxInf>PmtId>EndToEndId"`
	Reason    string   `xml:"CstmrCdtTrfInitn>PmtInf>CdtTrfTxInf>CdtTrfTxSts,omitempty"`
}

// SubmitPain001 submits the XML message.
func (c *Client) SubmitPain001(ctx context.Context, xmlBody []byte, idempotencyKey string) (*SubmissionResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/sepa/pain.001", bytes.NewReader(xmlBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/xml")
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
		return nil, &APIError{Status: resp.StatusCode, Code: parseReasonCode(data), Msg: string(data)}
	}
	var sr SubmissionResponse
	if err := xml.Unmarshal(data, &sr); err != nil {
		return nil, fmt.Errorf("gateway: unmarshal pain.001 response: %w", err)
	}
	return &sr, nil
}

// Pain002Status is the status returned by pain.002 polling.
type Pain002Status struct {
	XMLName    xml.Name `xml:"Document"`
	EndToEndID string   `xml:"CstmrPmtStsRpt>OrgnlPmtInf>TxInf>OrgnlEndToEndId"`
	Status     string   `xml:"CstmrPmtStsRpt>OrgnlPmtInf>TxInf>TxSts"`
	Reason     string   `xml:"CstmrPmtStsRpt>OrgnlPmtInf>TxInf>StsRsnInf>Rsn>Cd,omitempty"`
}

// GetPain002 polls pain.002 status for the given end-to-end id.
func (c *Client) GetPain002(ctx context.Context, endToEndID string) (*Pain002Status, error) {
	url := fmt.Sprintf("%s/v1/sepa/pain.002/%s", c.BaseURL, endToEndID)
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
		return nil, &APIError{Status: resp.StatusCode, Code: "NOT_FOUND", Msg: string(data)}
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{Status: resp.StatusCode, Code: parseReasonCode(data), Msg: string(data)}
	}
	var st Pain002Status
	if err := xml.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("gateway: unmarshal pain.002 response: %w", err)
	}
	return &st, nil
}

// APIError is the error returned for non-2xx responses.
type APIError struct {
	Status int
	Code   string
	Msg    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("gateway error: status=%d code=%s msg=%s", e.Status, e.Code, e.Msg)
}

// DeclineCode returns the SEPA reason code from an error if present.
func DeclineCode(err error) string {
	if ae, ok := err.(*APIError); ok {
		return ae.Code
	}
	return ""
}

func parseReasonCode(data []byte) string {
	s := string(data)
	if idx := indexOf(s, "Rsn>"); idx >= 0 {
		rest := s[idx+len("Rsn>"):]
		end := indexOf(rest, "<")
		if end >= 0 {
			return rest[:end]
		}
	}
	return ""
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// MTLSCertExists reports whether the configured mTLS cert file exists. Used
// by the adapter to decide whether to require mTLS.
func MTLSCertExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
