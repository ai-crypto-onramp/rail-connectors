package nacha

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestParseWithHeader(t *testing.T) {
	t.Parallel()
	csv := "payment_id,rail,amount,currency,settled_at,source_ref\n" +
		"p1,ach,100.50,USD,2026-07-14T12:00:00Z,file1.csv\n" +
		"p2,ach,200.00,USD,2026-07-14T12:01:00Z,file1.csv\n"
	entries, err := Parse(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d", len(entries))
	}
	if entries[0].PaymentID != "p1" || !entries[0].Amount.Equal(decimal.NewFromFloat(100.50)) {
		t.Fatalf("entry0 = %+v", entries[0])
	}
	if entries[0].SourceRef != "file1.csv" {
		t.Fatalf("source = %q", entries[0].SourceRef)
	}
	if !entries[0].SettledAt.Equal(time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("settled_at = %v", entries[0].SettledAt)
	}
}

func TestParseNoHeader(t *testing.T) {
	t.Parallel()
	csv := "p3,ach,50,EUR,2026-07-14T12:00:00Z\n"
	entries, err := Parse(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].PaymentID != "p3" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestParseEmpty(t *testing.T) {
	t.Parallel()
	if _, err := Parse(strings.NewReader("")); err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestParseBadAmount(t *testing.T) {
	t.Parallel()
	csv := "payment_id,rail,amount,currency,settled_at\np4,ach,NOTANUM,EUR,2026-07-14T12:00:00Z\n"
	if _, err := Parse(strings.NewReader(csv)); err == nil {
		t.Fatal("expected error for bad amount")
	}
}

func TestParseTooFewCols(t *testing.T) {
	t.Parallel()
	csv := "payment_id,rail,amount\np5,ach,10\n"
	if _, err := Parse(strings.NewReader(csv)); err == nil {
		t.Fatal("expected error for too few cols")
	}
}
