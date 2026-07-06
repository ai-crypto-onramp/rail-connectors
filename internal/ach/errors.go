package ach

import "github.com/ai-crypto-onramp/rail-connectors/internal/rail"

// MapReturnCode normalizes an ACH return code onto the common error
// taxonomy.
//
// Reference: Nacha return code list (R01..R83 subset relevant to debits).
func MapReturnCode(code string) (railCode, reason string) {
	switch code {
	case "R01":
		return rail.ErrInsufficientFunds, "insufficient funds"
	case "R02":
		return rail.ErrExpiredInstrument, "account closed"
	case "R03":
		return rail.ErrInvalidRequest, "no account / unable to locate"
	case "R04":
		return rail.ErrInvalidRequest, "invalid account number"
	case "R05":
		return rail.ErrInvalidRequest, "unauthorized debit"
	case "R06":
		return rail.ErrDoNotHonor, "returned per receiver request"
	case "R07":
		return rail.ErrFraudDecline, "authorization revoked"
	case "R08":
		return rail.ErrDoNotHonor, "payment stopped"
	case "R09":
		return rail.ErrInsufficientFunds, "uncollected funds"
	case "R10":
		return rail.ErrFraudDecline, "customer advises not authorized"
	case "R11":
		return rail.ErrInvalidRequest, "error in originating entry"
	case "R12":
		return rail.ErrExpiredInstrument, "account sold to another DFI"
	case "R13":
		return rail.ErrInvalidRequest, "invalid ACH routing number"
	case "R14":
		return rail.ErrExpiredInstrument, "account holder deceased"
	case "R16":
		return rail.ErrInvalidRequest, "account frozen"
	case "R20":
		return rail.ErrInvalidRequest, "non-transaction account"
	case "R29":
		return rail.ErrFraudDecline, "corporate customer advises not authorized"
	case "":
		return "", ""
	default:
		return rail.ErrDoNotHonor, "ach return code " + code
	}
}
