package webhooks

import (
	"testing"
)

func TestVerifyGood(t *testing.T) {
	t.Parallel()
	body := []byte(`{"payment_id":"p","status":"captured"}`)
	sig := Compute(body, "dev-secret")
	if err := Verify(body, sig, "dev-secret"); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestVerifyBadSecret(t *testing.T) {
	t.Parallel()
	body := []byte(`x`)
	sig := Compute(body, "dev-secret")
	if err := Verify(body, sig, "wrong"); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestVerifyTamperedBody(t *testing.T) {
	t.Parallel()
	sig := Compute([]byte(`original`), "dev-secret")
	if err := Verify([]byte(`tampered`), sig, "dev-secret"); err == nil {
		t.Fatal("expected error for tampered body")
	}
}

func TestVerifyEmptySig(t *testing.T) {
	t.Parallel()
	if err := Verify([]byte(`x`), "", "dev-secret"); err == nil {
		t.Fatal("expected error for empty signature")
	}
}
