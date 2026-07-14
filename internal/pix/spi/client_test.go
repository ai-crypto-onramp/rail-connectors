package spi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveDICTHappy(t *testing.T) {
	t.Parallel()
	var gotPath, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"key":"alice@example.com","account":"acct1","bank_code":"001","owner_name":"Alice","document":"123"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "apikey")
	dr, err := c.ResolveDICT(context.Background(), "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if dr.Account != "acct1" {
		t.Fatalf("account = %q", dr.Account)
	}
	if gotPath != "/v1/pix/dict/alice@example.com" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotKey != "apikey" {
		t.Fatalf("key = %q", gotKey)
	}
}

func TestResolveDICTNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.ResolveDICT(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "KEY_NOT_FOUND") {
		t.Fatalf("err = %v", err)
	}
}

func TestInitiatePaymentHappy(t *testing.T) {
	t.Parallel()
	var gotPath, gotID, gotCT string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotID = r.Header.Get("Idempotency-Key")
		gotCT = r.Header.Get("Content-Type")
		gotBody = readAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"payment_id":"P1","status":"CONFIRMED"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	pr, err := c.InitiatePayment(context.Background(), "idem-1", "E2E1", "123", "acct1", "001", 50.0, "BRL")
	if err != nil {
		t.Fatal(err)
	}
	if pr.PaymentID != "P1" {
		t.Fatalf("payment_id = %q", pr.PaymentID)
	}
	if pr.Status != "CONFIRMED" {
		t.Fatalf("status = %q", pr.Status)
	}
	if gotPath != "/v1/pix/payments" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotID != "idem-1" {
		t.Fatalf("idem = %q", gotID)
	}
	if gotCT != "application/json" {
		t.Fatalf("ct = %q", gotCT)
	}
	if !strings.Contains(string(gotBody), "E2E1") {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestInitiatePaymentReturnCode(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"return_code":"BE01","message":"no balance"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.InitiatePayment(context.Background(), "i", "E2E", "d", "a", "b", 1.0, "BRL")
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if ae.Code != "BE01" {
		t.Fatalf("code = %q", ae.Code)
	}
	if DeclineCode(err) != "BE01" {
		t.Fatal("DeclineCode wrong")
	}
}

func TestGetPaymentHappy(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"payment_id":"P1","status":"CONFIRMED"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	pr, err := c.GetPayment(context.Background(), "P1")
	if err != nil {
		t.Fatal(err)
	}
	if pr.Status != "CONFIRMED" {
		t.Fatalf("status = %q", pr.Status)
	}
	if gotPath != "/v1/pix/payments/P1" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestGetPaymentNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetPayment(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "PAYMENT_NOT_FOUND") {
		t.Fatalf("err = %v", err)
	}
}

func TestRefundPaymentHappy(t *testing.T) {
	t.Parallel()
	var gotPath, gotID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotID = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"payment_id":"R1","status":"REFUNDED"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	pr, err := c.RefundPayment(context.Background(), "idem-r", "P1", 25.0, "BRL")
	if err != nil {
		t.Fatal(err)
	}
	if pr.PaymentID != "R1" {
		t.Fatalf("payment_id = %q", pr.PaymentID)
	}
	if gotPath != "/v1/pix/refunds" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotID != "idem-r" {
		t.Fatalf("idem = %q", gotID)
	}
}

func TestServerErrorMaps(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.ResolveDICT(context.Background(), "x")
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
	_ = json.RawMessage(nil)
}

func readAll(r interface{ Read([]byte) (int, error) }) []byte {
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	return buf[:n]
}
