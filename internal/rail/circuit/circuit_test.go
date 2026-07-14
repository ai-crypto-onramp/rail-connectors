package circuit

import (
	"testing"
	"time"
)

func TestClosedAllowsAndResets(t *testing.T) {
	t.Parallel()
	b := New("card:authorize", 3, 10*time.Millisecond, "")
	if b.State() != StateClosed {
		t.Fatalf("state = %v", b.State())
	}
	if !b.Allow() {
		t.Fatal("closed should allow")
	}
	b.RecordSuccess()
	if b.failures != 0 {
		t.Fatal("failures not reset")
	}
}

func TestOpensAfterMaxFailures(t *testing.T) {
	t.Parallel()
	b := New("card:capture", 3, 50*time.Millisecond, "rail_circuit_open_test")
	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("call %d should be allowed", i)
		}
		b.RecordFailure()
	}
	if b.State() != StateOpen {
		t.Fatalf("expected open, got %v", b.State())
	}
	if b.Allow() {
		t.Fatal("open should reject")
	}
}

func TestHalfOpenAfterCoolDown(t *testing.T) {
	t.Parallel()
	b := New("card:refund", 2, 10*time.Millisecond, "")
	b.RecordFailure()
	b.RecordFailure()
	if b.State() != StateOpen {
		t.Fatalf("expected open, got %v", b.State())
	}
	time.Sleep(15 * time.Millisecond)
	if !b.Allow() {
		t.Fatal("should allow after cooldown (half-open)")
	}
	if b.State() != StateHalfOpen {
		t.Fatalf("expected half-open, got %v", b.State())
	}
}

func TestHalfOpenSuccessCloses(t *testing.T) {
	t.Parallel()
	b := New("ach:authorize", 1, 5*time.Millisecond, "")
	b.RecordFailure()
	time.Sleep(10 * time.Millisecond)
	b.Allow()
	b.RecordSuccess()
	if b.State() != StateClosed {
		t.Fatalf("expected closed, got %v", b.State())
	}
}

func TestHalfOpenFailureReopens(t *testing.T) {
	t.Parallel()
	b := New("ach:capture", 1, 5*time.Millisecond, "")
	b.RecordFailure()
	time.Sleep(10 * time.Millisecond)
	b.Allow()
	b.RecordFailure()
	if b.State() != StateOpen {
		t.Fatalf("expected reopened, got %v", b.State())
	}
}

func TestOpenMetricIncremented(t *testing.T) {
	t.Parallel()
	b := New("sepa:authorize", 1, 5*time.Millisecond, "rail_circuit_open_sepa_test")
	b.RecordFailure()
	if b.openCounter == nil {
		t.Fatal("openCounter nil")
	}
	if b.openCounter.Value() != 1 {
		t.Fatalf("open counter = %v", b.openCounter.Value())
	}
}
