// Package spi is a settlement parser for Banco Central do Brasil SPI reports.
// The report is a simple JSON array of settled payment objects.
package spi

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Entry is a single settled entry parsed from an SPI report.
type Entry struct {
	PaymentID string
	Rail      string
	Amount    float64
	Currency  string
	SettledAt time.Time
	SourceRef string
}

type rawEntry struct {
	PaymentID string  `json:"payment_id"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	SettledAt string  `json:"settled_at"`
}

// Parse parses a JSON SPI settlement report from r.
func Parse(r io.Reader, sourceRef string) ([]Entry, error) {
	var rows []rawEntry
	dec := json.NewDecoder(r)
	if err := dec.Decode(&rows); err != nil {
		return nil, fmt.Errorf("spi settlement: decode: %w", err)
	}
	var out []Entry
	for _, row := range rows {
		t := parseTime(row.SettledAt)
		out = append(out, Entry{
			PaymentID: strings.TrimSpace(row.PaymentID),
			Rail:      "pix",
			Amount:    row.Amount,
			Currency:  strings.ToUpper(row.Currency),
			SettledAt: t,
			SourceRef: sourceRef,
		})
	}
	return out, nil
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	formats := []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Now().UTC()
}
