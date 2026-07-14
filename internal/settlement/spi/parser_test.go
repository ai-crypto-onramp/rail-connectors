package spi

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()
	body := `[{"payment_id":"pp1","amount":100.50,"currency":"brl","settled_at":"2026-07-14T12:00:00Z"},` +
		`{"payment_id":"pp2","amount":200.00,"currency":"brl","settled_at":"2026-07-14T12:01:00Z"}]`
	entries, err := Parse(strings.NewReader(body), "report.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d", len(entries))
	}
	if entries[0].PaymentID != "pp1" || entries[0].Amount != 100.50 {
		t.Fatalf("entry0 = %+v", entries[0])
	}
	if entries[0].Currency != "BRL" {
		t.Fatalf("currency = %q", entries[0].Currency)
	}
	if entries[0].SourceRef != "report.json" {
		t.Fatalf("source = %q", entries[0].SourceRef)
	}
}

func TestParseBadJSON(t *testing.T) {
	t.Parallel()
	if _, err := Parse(strings.NewReader("not json"), "x"); err == nil {
		t.Fatal("expected error")
	}
}
