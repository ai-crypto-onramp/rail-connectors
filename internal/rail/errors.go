package rail

import "fmt"

// Normalized error code taxonomy consumed by Payment Orchestration and the
// Policy Engine.
const (
	ErrInsufficientFunds = "INSUFFICIENT_FUNDS"
	ErrDoNotHonor        = "DO_NOT_HONOR"
	ErrFraudDecline      = "FRAUD_DECLINE"
	ErrExpiredInstrument = "EXPIRED_INSTRUMENT"
	ErrInvalidRequest    = "INVALID_REQUEST"
	ErrRailUnavailable   = "RAIL_UNAVAILABLE"
	ErrSettlementBreak   = "SETTLEMENT_BREAK"
)

// RailError carries a normalized error code plus a human-readable reason.
type RailError struct {
	Code   string
	Reason string
}

func (e *RailError) Error() string {
	return fmt.Sprintf("rail error: %s: %s", e.Code, e.Reason)
}

// NewRailError constructs a RailError.
func NewRailError(code, reason string) *RailError {
	return &RailError{Code: code, Reason: reason}
}
