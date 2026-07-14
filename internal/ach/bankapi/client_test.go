package bankapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSubmitNACHAHappy(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"batch_id":"BATCH1","status":"accepted"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "apikey")
	resp, err := c.SubmitNACHA(context.Background(), []byte("NACHAFILE"), "idem-1")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Accepted {
		t.Fatal("expected accepted")
	}
	if resp.BatchID != "BATCH1" {
		t.Fatalf("batch_id = %q", resp.BatchID)
	}
	if gotPath != "/v1/ach/batches" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotID != "idem-1" {
		t.Fatalf("idem = %q", gotID)
	}
	if gotCT != "text/plain" {
		t.Fatalf("content-type = %q", gotCT)
	}
	if gotKey != "apikey" {
		t.Fatalf("apikey = %q", gotKey)
	}
	if string(gotBody) != "NACHAFILE" {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestSubmitNACHARejected(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"batch_id":"","status":"rejected","reject_message":"bad file"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	resp, err := c.SubmitNACHA(context.Background(), []byte("x"), "i")
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Rejected {
		t.Fatal("expected rejected")
	}
	if resp.RejectMsg != "bad file" {
		t.Fatalf("msg = %q", resp.RejectMsg)
	}
}

func TestSubmitNACHAServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("oops"))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.SubmitNACHA(context.Background(), []byte("x"), "i")
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
}

func TestGetBatchStatusHappy(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"batch_id":"B1","status":"settled"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	bs, err := c.GetBatchStatus(context.Background(), "B1")
	if err != nil {
		t.Fatal(err)
	}
	if bs.Status != "settled" {
		t.Fatalf("status = %q", bs.Status)
	}
	if gotPath != "/v1/ach/batches/B1" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestGetBatchStatusReturnCode(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"batch_id":"B1","status":"returned","return_code":"R01"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	bs, err := c.GetBatchStatus(context.Background(), "B1")
	if err != nil {
		t.Fatal(err)
	}
	if bs.ReturnCode != "R01" {
		t.Fatalf("return_code = %q", bs.ReturnCode)
	}
}

func TestGetBatchStatusNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("nope"))
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	_, err := c.GetBatchStatus(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "BATCH_NOT_FOUND") {
		t.Fatalf("err = %v", err)
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
