package rail

import "testing"

func TestIdempotencyKey_Stable(t *testing.T) {
	a := IdempotencyKey("tx-1", "capture", 1)
	b := IdempotencyKey("tx-1", "capture", 1)
	if a != b {
		t.Fatalf("idempotency key not stable: %q != %q", a, b)
	}
	if a != "tx-1:capture:1" {
		t.Fatalf("unexpected key: %q", a)
	}
	if IdempotencyKey("tx-1", "capture", 1) == IdempotencyKey("tx-1", "capture", 2) {
		t.Fatalf("keys should differ by attempt")
	}
	if IdempotencyKey("tx-1", "capture", 1) == IdempotencyKey("tx-1", "refund", 1) {
		t.Fatalf("keys should differ by operation")
	}
}

func TestStatusString(t *testing.T) {
	if StatusAuthorized.String() != "authorized" {
		t.Errorf("status string mismatch")
	}
	if StatusUnknown.String() != "unknown" {
		t.Errorf("status string mismatch")
	}
}

func TestRailError(t *testing.T) {
	e := NewRailError(ErrInsufficientFunds, "nsf")
	if e.Code != ErrInsufficientFunds || e.Error() == "" {
		t.Errorf("rail error: %+v", e)
	}
}
