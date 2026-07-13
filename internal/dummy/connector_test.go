package dummy

import (
	"context"
	"testing"

	"github.com/ai-crypto-onramp/rail-connectors/internal/audit"
	"github.com/ai-crypto-onramp/rail-connectors/internal/rail"
	"github.com/ai-crypto-onramp/rail-connectors/internal/settlement"
	"github.com/ai-crypto-onramp/rail-connectors/internal/store"
)

func newTestConnector(t *testing.T, fail bool) (*Connector, *store.Store, *settlement.Tracker, *audit.Recorder) {
	t.Helper()
	s := store.New()
	tr := settlement.New(s)
	rec := audit.NewRecorder()
	c := New(s, tr, Config{Rail: "card", Fail: fail, AuditSink: rec})
	return c, s, tr, rec
}

func TestAuthorizeHappy(t *testing.T) {
	t.Parallel()
	c, s, _, rec := newTestConnector(t, false)
	ctx := rail.Context{PaymentID: "p1", Rail: "card", Amount: 10.5, Currency: "USD"}
	resp, err := c.Authorize(context.Background(), ctx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Status != rail.StatusAuthorized {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.RailRef == "" {
		t.Fatal("expected rail_ref")
	}
	r, ok := s.Get("p1")
	if !ok || r.Status != rail.StatusAuthorized {
		t.Fatalf("store record wrong: %+v", r)
	}
	if rec.Count() != 1 || rec.Events()[0].Operation != "authorize" {
		t.Fatalf("audit not emitted: %+v", rec.Events())
	}
}

func TestAuthorizeMissingPaymentID(t *testing.T) {
	t.Parallel()
	c, _, _, _ := newTestConnector(t, false)
	resp, _ := c.Authorize(context.Background(), rail.Context{Rail: "card"})
	if resp.Status != rail.StatusFailed || resp.ErrorCode != rail.CodeInvalidRequest {
		t.Fatalf("expected failed invalid, got %+v", resp)
	}
}

func TestAuthorizeForcedFail(t *testing.T) {
	t.Parallel()
	c, _, _, _ := newTestConnector(t, true)
	resp, err := c.Authorize(context.Background(), rail.Context{PaymentID: "p2"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != rail.StatusFailed || resp.ErrorCode != rail.CodeRailUnavailable {
		t.Fatalf("got %+v", resp)
	}
}

func TestCaptureHappy(t *testing.T) {
	t.Parallel()
	c, s, tr, _ := newTestConnector(t, false)
	ctx := rail.Context{PaymentID: "p3", Rail: "card", Amount: 25.0, Currency: "USD"}
	if _, err := c.Authorize(context.Background(), ctx); err != nil {
		t.Fatal(err)
	}
	resp, err := c.Capture(context.Background(), ctx, 25.0)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != rail.StatusCaptured {
		t.Fatalf("status = %q", resp.Status)
	}
	r, _ := s.Get("p3")
	if r.Status != rail.StatusCaptured {
		t.Fatalf("store status = %q", r.Status)
	}
	if tr.Total("card") != 25.0 {
		t.Fatalf("tracker total = %v", tr.Total("card"))
	}
}

func TestCaptureUnknownPayment(t *testing.T) {
	t.Parallel()
	c, _, _, _ := newTestConnector(t, false)
	resp, _ := c.Capture(context.Background(), rail.Context{PaymentID: "ghost"}, 1.0)
	if resp.ErrorCode != rail.CodeInvalidRequest {
		t.Fatalf("expected invalid, got %+v", resp)
	}
}

func TestCaptureWrongState(t *testing.T) {
	t.Parallel()
	c, _, _, _ := newTestConnector(t, false)
	// No authorize first; store record will be absent.
	resp, _ := c.Capture(context.Background(), rail.Context{PaymentID: "p4"}, 5.0)
	if resp.ErrorCode != rail.CodeInvalidRequest {
		t.Fatalf("expected invalid, got %+v", resp)
	}
}

func TestRefundHappy(t *testing.T) {
	t.Parallel()
	c, s, tr, _ := newTestConnector(t, false)
	ctx := rail.Context{PaymentID: "p5", Rail: "card", Amount: 50.0, Currency: "EUR"}
	if _, err := c.Authorize(context.Background(), ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Capture(context.Background(), ctx, 50.0); err != nil {
		t.Fatal(err)
	}
	resp, err := c.Refund(context.Background(), ctx, 20.0)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != rail.StatusRefunded {
		t.Fatalf("status = %q", resp.Status)
	}
	r, _ := s.Get("p5")
	if r.Status != rail.StatusRefunded {
		t.Fatalf("store status = %q", r.Status)
	}
	if tr.Total("card") != 30.0 {
		t.Fatalf("net total = %v", tr.Total("card"))
	}
}

func TestRefundWrongState(t *testing.T) {
	t.Parallel()
	c, _, _, _ := newTestConnector(t, false)
	ctx := rail.Context{PaymentID: "p6", Rail: "card", Amount: 10.0}
	if _, err := c.Authorize(context.Background(), ctx); err != nil {
		t.Fatal(err)
	}
	resp, _ := c.Refund(context.Background(), ctx, 10.0)
	if resp.ErrorCode != rail.CodeInvalidRequest {
		t.Fatalf("expected invalid (not captured), got %+v", resp)
	}
}

func TestGetStatus(t *testing.T) {
	t.Parallel()
	c, _, _, _ := newTestConnector(t, false)
	ctx := rail.Context{PaymentID: "p7", Rail: "card"}
	if _, err := c.Authorize(context.Background(), ctx); err != nil {
		t.Fatal(err)
	}
	st, err := c.GetStatus(context.Background(), ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st != rail.StatusAuthorized {
		t.Fatalf("status = %q", st)
	}
}

func TestGetStatusUnknown(t *testing.T) {
	t.Parallel()
	c, _, _, _ := newTestConnector(t, false)
	_, err := c.GetStatus(context.Background(), rail.Context{PaymentID: "missing"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSetFailRuntimeToggle(t *testing.T) {
	t.Parallel()
	c, _, _, _ := newTestConnector(t, false)
	ctx := rail.Context{PaymentID: "p8", Rail: "card"}
	if _, err := c.Authorize(context.Background(), ctx); err != nil {
		t.Fatal(err)
	}
	c.SetFail(true)
	resp, _ := c.Capture(context.Background(), ctx, 1.0)
	if resp.ErrorCode != rail.CodeRailUnavailable {
		t.Fatalf("expected forced fail, got %+v", resp)
	}
}

func TestRegistryNew(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"card", "ach", "sepa", "pix", "upi", "dummy"} {
		conn, err := rail.New(f, nil)
		if err != nil {
			t.Fatalf("New(%q): %v", f, err)
		}
		if conn == nil {
			t.Fatalf("New(%q) returned nil connector", f)
		}
	}
}

func TestFormatAmount(t *testing.T) {
	t.Parallel()
	if FormatAmount(1) != "1.00" {
		t.Fatalf("got %q", FormatAmount(1))
	}
}
