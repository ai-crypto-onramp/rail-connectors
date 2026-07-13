package rail

import "errors"

// Normalized error code taxonomy shared across all rails.
const (
	CodeInsufficientFunds = "INSUFFICIENT_FUNDS"
	CodeDoNotHonor        = "DO_NOT_HONOR"
	CodeFraudDecline      = "FRAUD_DECLINE"
	CodeExpiredInstrument = "EXPIRED_INSTRUMENT"
	CodeInvalidRequest    = "INVALID_REQUEST"
	CodeRailUnavailable   = "RAIL_UNAVAILABLE"
	CodeSettlementBreak   = "SETTLEMENT_BREAK"
)

// Error is a normalized rail error carrying a stable code and a human reason.
type Error struct {
	Code   string
	Reason string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Reason == "" {
		return e.Code
	}
	return e.Code + ": " + e.Reason
}

// NewError constructs a normalized rail error.
func NewError(code, reason string) *Error {
	return &Error{Code: code, Reason: reason}
}

// AsError unwraps err into a *Error if possible; otherwise it wraps the
// generic error in an INVALID_REQUEST code.
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var re *Error
	if errors.As(err, &re) {
		return re
	}
	return NewError(CodeInvalidRequest, err.Error())
}
