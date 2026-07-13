package audit

import "testing"

func TestRecorderEmitAndCount(t *testing.T) {
	t.Parallel()
	r := NewRecorder()
	r.Emit(Event{Type: "x", PaymentID: "p1"})
	r.Emit(Event{Type: "y", PaymentID: "p2"})
	if r.Count() != 2 {
		t.Fatalf("count = %d", r.Count())
	}
	if len(r.Events()) != 2 {
		t.Fatalf("events len = %d", len(r.Events()))
	}
	r.Reset()
	if r.Count() != 0 {
		t.Fatalf("count after reset = %d", r.Count())
	}
}

func TestEmitFillsOccurredAt(t *testing.T) {
	t.Parallel()
	r := NewRecorder()
	r.Emit(Event{Type: "z"})
	if r.Events()[0].OccurredAt.IsZero() {
		t.Fatal("expected OccurredAt set")
	}
}
