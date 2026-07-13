package rail

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Status represents the normalized lifecycle state of a rail payment.
type Status string

const (
	StatusUnknown    Status = "unknown"
	StatusPending    Status = "pending"
	StatusAuthorized Status = "authorized"
	StatusCaptured   Status = "captured"
	StatusSettled    Status = "settled"
	StatusRefunded   Status = "refunded"
	StatusFailed     Status = "failed"
	StatusChargeback Status = "chargeback"
)

// ValidStatus reports whether s is a recognized status value.
func ValidStatus(s Status) bool {
	switch s {
	case StatusPending, StatusAuthorized, StatusCaptured, StatusSettled,
		StatusRefunded, StatusFailed, StatusChargeback:
		return true
	}
	return false
}

// Context carries the normalized inputs for a rail operation.
type Context struct {
	PaymentID      string            `json:"payment_id"`
	Rail           string            `json:"rail"`
	Amount         float64           `json:"amount"`
	Currency       string            `json:"currency"`
	PayerRef       string            `json:"payer_ref"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	Attempt        int               `json:"attempt,omitempty"`
	RailSpecific   map[string]string `json:"rail_specific,omitempty"`
}

// Response is the normalized output of a rail operation.
type Response struct {
	Status       Status   `json:"status"`
	RailRef      string   `json:"rail_ref,omitempty"`
	SettleAmount *float64 `json:"settle_amount,omitempty"`
	ErrorCode    string   `json:"error_code,omitempty"`
	ErrorMessage string   `json:"error_message,omitempty"`
	RawResponse  []byte   `json:"raw_response,omitempty"`
}

// StatusResponse is the shape returned by GET /v1/status.
type StatusResponse struct {
	PaymentID string `json:"payment_id"`
	Status    Status `json:"status"`
	RailRef   string `json:"rail_ref,omitempty"`
}

// IdempotencyKey derives the stable outbound idempotency key from the
// platform payment id, the operation name, and the attempt number:
// `<payment_id>:<operation>:<attempt>`.
func IdempotencyKey(paymentID, operation string, attempt int) string {
	return fmt.Sprintf("%s:%s:%d", paymentID, operation, attempt)
}

// HashKey returns a stable SHA-256 hex digest of an idempotency key, useful
// for dedup indexes keyed on a fixed-width column.
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
