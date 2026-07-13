package rail

import "testing"

func TestIdempotencyKeyStability(t *testing.T) {
	t.Parallel()
	cases := []struct {
		pid, op string
		attempt int
		want    string
	}{
		{"tx-1", "authorize", 1, "tx-1:authorize:1"},
		{"tx-1", "authorize", 2, "tx-1:authorize:2"},
		{"tx-2", "capture", 1, "tx-2:capture:1"},
	}
	for _, c := range cases {
		got := IdempotencyKey(c.pid, c.op, c.attempt)
		if got != c.want {
			t.Fatalf("IdempotencyKey(%q,%q,%d) = %q, want %q", c.pid, c.op, c.attempt, got, c.want)
		}
		// Replaying with the same inputs must yield the same key.
		if IdempotencyKey(c.pid, c.op, c.attempt) != got {
			t.Fatalf("IdempotencyKey not stable across replays")
		}
	}
}

func TestHashKeyDeterministic(t *testing.T) {
	t.Parallel()
	k := IdempotencyKey("p1", "authorize", 1)
	h1 := HashKey(k)
	h2 := HashKey(k)
	if h1 != h2 {
		t.Fatalf("HashKey not deterministic")
	}
	if len(h1) != 64 {
		t.Fatalf("HashKey wrong length %d", len(h1))
	}
}

func TestValidStatus(t *testing.T) {
	t.Parallel()
	valid := []Status{StatusPending, StatusAuthorized, StatusCaptured, StatusSettled, StatusRefunded, StatusFailed, StatusChargeback}
	for _, s := range valid {
		if !ValidStatus(s) {
			t.Errorf("expected %q valid", s)
		}
	}
	if ValidStatus(Status("weird")) {
		t.Errorf("expected weird invalid")
	}
}

func TestErrorFormat(t *testing.T) {
	t.Parallel()
	e := NewError(CodeDoNotHonor, "bank said no")
	if e.Error() != "DO_NOT_HONOR: bank said no" {
		t.Fatalf("unexpected: %q", e.Error())
	}
	e2 := NewError(CodeInvalidRequest, "")
	if e2.Error() != "INVALID_REQUEST" {
		t.Fatalf("unexpected: %q", e2.Error())
	}
}

func TestAsErrorWraps(t *testing.T) {
	t.Parallel()
	custom := &Error{Code: CodeFraudDecline, Reason: "x"}
	if AsError(custom).Code != CodeFraudDecline {
		t.Fatal("AsError did not unwrap")
	}
	other := AsError(plainErr{"boom"})
	if other.Code != CodeInvalidRequest {
		t.Fatalf("AsError did not wrap; got %q", other.Code)
	}
}

type plainErr struct{ s string }

func (p plainErr) Error() string { return p.s }

func TestRegistryUnknown(t *testing.T) {
	t.Parallel()
	_, err := New("definitely-not-a-rail", nil)
	if err == nil {
		t.Fatal("expected error for unknown family")
	}
	re, ok := err.(*Error)
	if !ok || re.Code != CodeInvalidRequest {
		t.Fatalf("expected INVALID_REQUEST, got %v", err)
	}
}

func TestRegisterKnownFamilies(t *testing.T) {
	t.Parallel()
	Register("test-only-family", func(cfg map[string]string) (Connector, error) { return nil, nil })
	if !ValidFamily("test-only-family") {
		t.Fatal("expected test-only-family to be registered")
	}
	if ValidFamily("still-not-real") {
		t.Fatal("expected still-not-real invalid")
	}
	if len(Families()) == 0 {
		t.Fatal("expected non-empty families")
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	Register("dup-family", func(cfg map[string]string) (Connector, error) { return nil, nil })
	Register("dup-family", func(cfg map[string]string) (Connector, error) { return nil, nil })
}
