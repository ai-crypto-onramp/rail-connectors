package rail

import (
	"fmt"
	"math/big"
)

// RailStatus is the normalized lifecycle state of a rail request.
type RailStatus int

const (
	StatusUnknown RailStatus = iota
	StatusAuthorized
	StatusCaptured
	StatusSettled
	StatusRefunded
	StatusFailed
	StatusChargeback
)

// String returns the canonical name of the status.
func (s RailStatus) String() string {
	switch s {
	case StatusAuthorized:
		return "authorized"
	case StatusCaptured:
		return "captured"
	case StatusSettled:
		return "settled"
	case StatusRefunded:
		return "refunded"
	case StatusFailed:
		return "failed"
	case StatusChargeback:
		return "chargeback"
	default:
		return "unknown"
	}
}

// Amount is a fixed-point monetary amount, in the smallest currency unit
// (cents, pence, etc.). It is safe for concurrent reads.
type Amount struct {
	// Value holds the integer amount in minor units of Currency.
	Value int64
	// Currency is the ISO 4217 currency code.
	Currency string
}

// AsBig returns the major-unit representation of the amount.
func (a Amount) AsBig() *big.Rat {
	return new(big.Rat).SetFrac(big.NewInt(a.Value), big.NewInt(100))
}

// RailContext carries the per-request inputs handed to a RailConnector.
type RailContext struct {
	TxID           string
	RailRequestID  string
	Amount         Amount
	IdempotencyKey string
	CustomerRef    string
	RailSpecific   map[string]string
}

// RailResponse is the normalized result of a RailConnector operation.
type RailResponse struct {
	Status        RailStatus
	RailRef       string
	SettledAmount *Amount
	ErrorCode     string
	ErrorReason   string
	RawPayload    []byte
}

// IdempotencyKey derives the stable outbound idempotency key from a
// transaction id, operation, and attempt number. Replays of the same
// (tx, op, attempt) tuple MUST yield the same key.
func IdempotencyKey(txID, operation string, attempt int) string {
	return fmt.Sprintf("%s:%s:%d", txID, operation, attempt)
}
