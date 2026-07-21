// Package npci is a settlement parser for NPCI UPI settlement files (CSV).
// Columns: payment_id, rail, amount, currency, settled_at, source_ref.
package npci

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Entry is a single settled entry parsed from an NPCI settlement CSV row.
type Entry struct {
	PaymentID string
	Rail      string
	Amount    decimal.Decimal
	Currency  string
	SettledAt time.Time
	SourceRef string
}

// Parse parses an NPCI settlement CSV file from r.
func Parse(r io.Reader) ([]Entry, error) {
	cr := csv.NewReader(r)
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("npci settlement: read csv: %w", err)
	}
	if len(records) == 0 {
		return nil, errors.New("npci settlement: empty file")
	}
	var entries []Entry
	startIdx := 0
	if isHeader(records[0]) {
		startIdx = 1
	}
	for i := startIdx; i < len(records); i++ {
		row := records[i]
		if len(row) < 5 {
			return nil, fmt.Errorf("npci settlement: row %d has %d cols, want >=5", i, len(row))
		}
		amt, err := decimal.NewFromString(strings.TrimSpace(row[2]))
		if err != nil {
			return nil, fmt.Errorf("npci settlement: row %d amount: %w", i, err)
		}
		t, _ := time.Parse(time.RFC3339, strings.TrimSpace(row[4]))
		e := Entry{
			PaymentID: strings.TrimSpace(row[0]),
			Rail:      strings.TrimSpace(row[1]),
			Amount:    amt,
			Currency:  strings.TrimSpace(row[3]),
			SettledAt: t,
		}
		if len(row) >= 6 {
			e.SourceRef = strings.TrimSpace(row[5])
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func isHeader(row []string) bool {
	if len(row) == 0 {
		return false
	}
	return strings.EqualFold(row[0], "payment_id")
}
