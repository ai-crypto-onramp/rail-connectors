package npci

import (
	"strings"
	"testing"
)

func TestParseWithHeader(t *testing.T) {
	t.Parallel()
	csv := "payment_id,rail,amount,currency,settled_at,source_ref\n" +
		"pu1,upi,100.00,INR,2026-07-14T12:00:00Z,settlement.csv\n" +
		"pu2,upi,200.00,INR,2026-07-14T12:01:00Z,settlement.csv\n"
	entries, err := Parse(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d", len(entries))
	}
	if entries[0].PaymentID != "pu1" || entries[0].Amount != 100.00 {
		t.Fatalf("entry0 = %+v", entries[0])
	}
	if entries[0].SourceRef != "settlement.csv" {
		t.Fatalf("source = %q", entries[0].SourceRef)
	}
}

func TestParseEmpty(t *testing.T) {
	t.Parallel()
	if _, err := Parse(strings.NewReader("")); err == nil {
		t.Fatal("expected error for empty")
	}
}

func TestParseBadAmount(t *testing.T) {
	t.Parallel()
	csv := "payment_id,rail,amount,currency,settled_at\npu3,upi,X,INR,2026-07-14T12:00:00Z\n"
	if _, err := Parse(strings.NewReader(csv)); err == nil {
		t.Fatal("expected error")
	}
}
