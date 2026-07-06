package ach

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ai-crypto-onramp/rail-connectors/internal/ach/nacha"
	"github.com/ai-crypto-onramp/rail-connectors/internal/rail"
)

func newCtx(tx, op string, amount int64) rail.RailContext {
	return rail.RailContext{
		TxID:           tx,
		RailRequestID:  tx,
		Amount:         rail.Amount{Value: amount, Currency: "USD"},
		IdempotencyKey: rail.IdempotencyKey(tx, op, 1),
		RailSpecific: map[string]string{
			"receiver_dfi":     "123456789",
			"receiver_account": "000111222",
			"receiver_name":    "ALICE",
			"origin_dfi":       "02100008",
		},
	}
}

func startPartner(t *testing.T, status string, returnCode string) (*httptest.Server, *int) {
	t.Helper()
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/batches":
			if r.Header.Get("Idempotency-Key") == "" {
				t.Errorf("missing idempotency key header")
			}
			body, _ := io.ReadAll(r.Body)
			if err := nacha.Validate(string(body)); err != nil {
				t.Errorf("submitted NACHA failed validation: %v", err)
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"batch_id":    "b-" + r.Header.Get("Idempotency-Key"),
				"status":      status,
				"rail_ref":    "REF-" + r.Header.Get("Idempotency-Key"),
				"status_code": status,
			})
		default:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"batch_id":    "b-x",
				"status":      status,
				"status_code": status,
				"return_code": returnCode,
			})
		}
	}))
	return srv, &calls
}

func TestAdapter_AuthorizeCapture(t *testing.T) {
	srv, calls := startPartner(t, "submitted", "")
	defer srv.Close()

	a := New(Config{PartnerURL: srv.URL})
	_, err := a.Authorize(context.Background(), newCtx("tx-1", "authorize", 0))
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	resp, err := a.Capture(context.Background(), newCtx("tx-1", "capture", 1500))
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if resp.Status != rail.StatusCaptured {
		t.Errorf("capture status: %s", resp.Status)
	}
	if resp.RailRef == "" {
		t.Errorf("expected rail ref")
	}
	rows := a.Requests()
	if len(rows) != 2 {
		t.Errorf("expected 2 rail_requests rows, got %d", len(rows))
	}
	if a.AuthorizeLatency("tx-1") <= 0 {
		t.Errorf("expected authorize latency recorded")
	}
	if a.CaptureLatency("tx-1") <= 0 {
		t.Errorf("expected capture latency recorded")
	}
	if *calls != 2 {
		t.Errorf("expected 2 partner calls, got %d", *calls)
	}
}

func TestAdapter_GetStatus_ReturnCodeNormalized(t *testing.T) {
	srv, _ := startPartner(t, "returned", "R01")
	defer srv.Close()

	a := New(Config{PartnerURL: srv.URL})
	resp, err := a.GetStatus(context.Background(), newCtx("tx-1", "capture", 1500))
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if resp.Status != rail.StatusFailed {
		t.Errorf("status: %s", resp.Status)
	}
	if resp.ErrorCode != rail.ErrInsufficientFunds {
		t.Errorf("error code: %s want %s", resp.ErrorCode, rail.ErrInsufficientFunds)
	}
}

func TestAdapter_Refund(t *testing.T) {
	srv, _ := startPartner(t, "submitted", "")
	defer srv.Close()

	a := New(Config{PartnerURL: srv.URL})
	resp, err := a.Refund(context.Background(), newCtx("tx-1", "refund", 1500))
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if resp.Status != rail.StatusRefunded {
		t.Errorf("status: %s", resp.Status)
	}
}

func TestAdapter_Authorize_PartnerDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	a := New(Config{PartnerURL: srv.URL})
	resp, err := a.Authorize(context.Background(), newCtx("tx-1", "authorize", 0))
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if resp.Status != rail.StatusFailed {
		t.Errorf("status: %s", resp.Status)
	}
	if resp.ErrorCode != rail.ErrRailUnavailable {
		t.Errorf("error code: %s", resp.ErrorCode)
	}
}

func TestAdapter_Chargeback(t *testing.T) {
	a := New(Config{})
	resp, err := a.Chargeback(context.Background(), newCtx("tx-1", "chargeback", 100))
	if err != nil {
		t.Fatalf("Chargeback: %v", err)
	}
	if resp.Status != rail.StatusChargeback {
		t.Errorf("status: %s", resp.Status)
	}
}

func TestMapReturnCode(t *testing.T) {
	cases := []struct {
		code, wantCode, wantReason string
	}{
		{"R01", rail.ErrInsufficientFunds, "insufficient funds"},
		{"R02", rail.ErrExpiredInstrument, "account closed"},
		{"R10", rail.ErrFraudDecline, "customer advises not authorized"},
		{"R29", rail.ErrFraudDecline, "corporate customer advises not authorized"},
		{"R08", rail.ErrDoNotHonor, "payment stopped"},
		{"", "", ""},
		{"R99", rail.ErrDoNotHonor, "ach return code R99"},
	}
	for _, c := range cases {
		gotCode, gotReason := MapReturnCode(c.code)
		if gotCode != c.wantCode || gotReason != c.wantReason {
			t.Errorf("MapReturnCode(%q) = (%q,%q), want (%q,%q)", c.code, gotCode, gotReason, c.wantCode, c.wantReason)
		}
	}
}

func TestNACHASubmissionIsValid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := nacha.Validate(string(body)); err != nil {
			t.Errorf("NACHA validation: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "submitted", "rail_ref": "REF"})
	}))
	defer srv.Close()

	a := New(Config{PartnerURL: srv.URL})
	if _, err := a.Capture(context.Background(), newCtx("tx-1", "capture", 2500)); err != nil {
		t.Fatalf("Capture: %v", err)
	}
}
