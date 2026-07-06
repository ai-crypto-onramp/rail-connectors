package bankapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSubmit_HappyPath(t *testing.T) {
	var gotPath, gotMethod, gotKey, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotKey = r.Header.Get("Idempotency-Key")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(SubmitResponse{
			BatchID:    "b-1",
			Status:     "submitted",
			RailRef:    "REF-1",
			StatusCode: "submitted",
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	resp, err := c.Submit(context.Background(), "tx:auth:1", "NACHA-BODY")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if gotPath != "/batches" || gotMethod != http.MethodPost {
		t.Errorf("request: %s %s", gotMethod, gotPath)
	}
	if gotKey != "tx:auth:1" {
		t.Errorf("idempotency key: %s", gotKey)
	}
	if gotBody != "NACHA-BODY" {
		t.Errorf("body: %s", gotBody)
	}
	if resp.RailRef != "REF-1" || resp.BatchID != "b-1" {
		t.Errorf("response: %+v", resp)
	}
}

func TestSubmit_5xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		io.WriteString(w, "down")
	}))
	defer srv.Close()

	c := New(srv.URL)
	if _, err := c.Submit(context.Background(), "k", "x"); err == nil {
		t.Fatal("expected error on 5xx")
	}
}

func TestGetStatus_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/batches/b-1") {
			t.Errorf("path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(StatusResponse{
			BatchID:    "b-1",
			Status:     "returned",
			StatusCode: "returned",
			ReturnCode: "R01",
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	resp, err := c.GetStatus(context.Background(), "b-1")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if resp.ReturnCode != "R01" {
		t.Errorf("return code: %s", resp.ReturnCode)
	}
}

func TestGetStatus_4xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if _, err := c.GetStatus(context.Background(), "missing"); err == nil {
		t.Fatal("expected error on 4xx")
	}
}

func TestSubmit_EmptyBaseURL(t *testing.T) {
	c := &Client{}
	if _, err := c.Submit(context.Background(), "k", "x"); err == nil {
		t.Fatal("expected error for empty base url")
	}
}
