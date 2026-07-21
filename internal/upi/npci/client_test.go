package npci

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestInitiateCollectHappy(t *testing.T) {
	t.Parallel()
	var gotPath, gotID, gotCT, gotKey string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotID = r.Header.Get("Idempotency-Key")
		gotCT = r.Header.Get("Content-Type")
		gotKey = r.Header.Get("X-API-Key")
		gotBody = readAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"collect_id":"C1","status":"INITIATED","response_code":"00"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "apikey")
	cr, err := c.InitiateCollect(context.Background(), "idem-1", "payer@upi", "payee@upi", decimal.NewFromInt(100), "INR", "test")
	if err != nil {
		t.Fatal(err)
	}
	if cr.CollectID != "C1" {
		t.Fatalf("collect_id = %q", cr.CollectID)
	}
	if cr.ResponseCode != "00" {
		t.Fatalf("response_code = %q", cr.ResponseCode)
	}
	if gotPath != "/v1/upi/collect" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotID != "idem-1" {
		t.Fatalf("idem = %q", gotID)
	}
	if gotCT != "application/json" {
		t.Fatalf("ct = %q", gotCT)
	}
	if gotKey != "apikey" {
		t.Fatalf("key = %q", gotKey)
	}
	if !strings.Contains(string(gotBody), "payer@upi") {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestInitiateCollectResponseCode(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"response_code":"ZD","message":"declined"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.InitiateCollect(context.Background(), "i", "p@upi", "y@upi", decimal.NewFromInt(1), "INR", "x")
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if ae.Code != "ZD" {
		t.Fatalf("code = %q", ae.Code)
	}
	if DeclineCode(err) != "ZD" {
		t.Fatal("DeclineCode wrong")
	}
}

func TestGetCollectStatusHappy(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"collect_id":"C1","status":"CONFIRMED","response_code":"00"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	cr, err := c.GetCollectStatus(context.Background(), "C1")
	if err != nil {
		t.Fatal(err)
	}
	if cr.Status != "CONFIRMED" {
		t.Fatalf("status = %q", cr.Status)
	}
	if gotPath != "/v1/upi/collect/C1" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestGetCollectStatusNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetCollectStatus(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "COLLECT_NOT_FOUND") {
		t.Fatalf("err = %v", err)
	}
}

func TestRefundHappy(t *testing.T) {
	t.Parallel()
	var gotPath, gotID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotID = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"refund_id":"R1","status":"REFUNDED","response_code":"00"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	rr, err := c.Refund(context.Background(), "idem-r", "C1", decimal.NewFromInt(50), "INR")
	if err != nil {
		t.Fatal(err)
	}
	if rr.RefundID != "R1" {
		t.Fatalf("refund_id = %q", rr.RefundID)
	}
	if gotPath != "/v1/upi/refunds" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotID != "idem-r" {
		t.Fatalf("idem = %q", gotID)
	}
}

func TestRecordDisputeHappy(t *testing.T) {
	t.Parallel()
	var gotPath, gotID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotID = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"dispute_id":"D1","status":"RECORDED","reason_code":"CHARGEBACK"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	dr, err := c.RecordDispute(context.Background(), "idem-d", "C1", "CHARGEBACK", decimal.NewFromInt(100), "INR")
	if err != nil {
		t.Fatal(err)
	}
	if dr.DisputeID != "D1" {
		t.Fatalf("dispute_id = %q", dr.DisputeID)
	}
	if gotPath != "/v1/upi/disputes" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotID != "idem-d" {
		t.Fatalf("idem = %q", gotID)
	}
}

func TestServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetCollectStatus(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
}

func TestAPIErrorString(t *testing.T) {
	t.Parallel()
	e := &APIError{Status: 500, Code: "X", Msg: "boom"}
	if !strings.Contains(e.Error(), "500") {
		t.Fatal("missing status")
	}
}

func readAll(r interface{ Read([]byte) (int, error) }) []byte {
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	return buf[:n]
}
