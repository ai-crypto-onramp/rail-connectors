package iso20022

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPain001Valid(t *testing.T) {
	t.Parallel()
	p := Payment{
		MsgID:         "MSG-1",
		Initiator:     "ONRAMP",
		DebtorName:    "Origin Co",
		DebtorIBAN:    "DE89370400440532013000",
		ExecutionDate: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
		Currency:      "EUR",
		Transfers: []Transfer{
			{EndToEndID: "E2E-1", CreditorName: "Bob", CreditorIBAN: "FR1420041010050500013M02606", Amount: 100.50, Reference: "REF-1"},
			{EndToEndID: "E2E-2", CreditorName: "Cara", CreditorIBAN: "FR1420041010050500013M02607", Amount: 200.00, Reference: "REF-2"},
		},
	}
	out, err := BuildPain001(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(out); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !strings.Contains(out, "pain.001.001.09") {
		t.Fatal("missing namespace")
	}
	if !strings.Contains(out, "MSG-1") {
		t.Fatal("missing MsgID")
	}
	if !strings.Contains(out, "E2E-1") {
		t.Fatal("missing end-to-end id")
	}
}

func TestBuildPain001Errors(t *testing.T) {
	t.Parallel()
	if _, err := BuildPain001(Payment{}); err == nil {
		t.Fatal("expected missing MsgID error")
	}
	if _, err := BuildPain001(Payment{MsgID: "x"}); err == nil {
		t.Fatal("expected no transfers error")
	}
}

func TestValidateRejectsBadDoc(t *testing.T) {
	t.Parallel()
	if err := Validate("<x/>"); err == nil {
		t.Fatal("expected error")
	}
	if err := Validate(`<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pain.001.001.09"></Document>`); err == nil {
		t.Fatal("expected missing CstmrCdtTrfInitn error")
	}
}
