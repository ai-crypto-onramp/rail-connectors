package settlement

import (
	"testing"

	"github.com/ai-crypto-onramp/rail-connectors/internal/store"
)

func TestTrackerTotals(t *testing.T) {
	t.Parallel()
	s := store.New()
	tr := New(s)
	tr.RecordCapture("card", 100)
	tr.RecordCapture("ach", 50)
	tr.RecordRefund("card", 30)
	if tr.Total("card") != 70 {
		t.Fatalf("card total = %v", tr.Total("card"))
	}
	if tr.Total("ach") != 50 {
		t.Fatalf("ach total = %v", tr.Total("ach"))
	}
	if tr.Totals()["card"] != 70 {
		t.Fatalf("totals card = %v", tr.Totals()["card"])
	}
}

func TestRecordSettleUpdatesStore(t *testing.T) {
	t.Parallel()
	s := store.New()
	tr := New(s)
	s.Upsert(store.Record{PaymentID: "p1", Rail: "card", Status: "captured"})
	tr.RecordSettle(store.SettleEntry{Rail: "card", PaymentID: "p1", Amount: 25})
	if s.SettledAmount("p1") != 25 {
		t.Fatalf("settled = %v", s.SettledAmount("p1"))
	}
	if tr.Total("card") != 25 {
		t.Fatalf("card total = %v", tr.Total("card"))
	}
}
