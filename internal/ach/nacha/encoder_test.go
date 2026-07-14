package nacha

import (
	"strings"
	"testing"
)

func TestEncodePPDDebit(t *testing.T) {
	t.Parallel()
	f := File{
		ImmediateOrigin: "1234567890",
		ImmediateDest:   "0987654321",
		OriginName:      "ORIGIN CO",
		DestName:        "DEST BANK",
		Batches: []Batch{
			{
				CompanyID:   "ORIGID0001",
				CompanyName: "ORIG CO",
				Description: "PRENOTE",
				Entries: []Entry{
					{TraceNumber: "1234560001", DFIAccount: "acct1", Routing: "012345678", Amount: 100, ReceiverName: "Alice"},
					{TraceNumber: "1234560002", DFIAccount: "acct2", Routing: "012345678", Amount: 250, ReceiverName: "Bob"},
				},
			},
		},
	}
	s, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "1") || !strings.Contains(s, "9") {
		t.Fatal("missing records")
	}
}

func TestEncodeValidationErrors(t *testing.T) {
	t.Parallel()
	if _, err := (File{ImmediateOrigin: "1"}).Encode(); err == nil {
		t.Fatal("expected origin length error")
	}
	f := File{ImmediateOrigin: "1234567890", ImmediateDest: "0987654321", Batches: []Batch{{CompanyID: "short"}}}
	if _, err := f.Encode(); err == nil {
		t.Fatal("expected company id error")
	}
	f = File{ImmediateOrigin: "1234567890", ImmediateDest: "0987654321", Batches: []Batch{{CompanyID: "ORIGID0001", Entries: []Entry{{Routing: "1"}}}}}
	if _, err := f.Encode(); err == nil {
		t.Fatal("expected routing length error")
	}
}

func TestValidateBalances(t *testing.T) {
	t.Parallel()
	f := File{
		ImmediateOrigin: "1234567890",
		ImmediateDest:   "0987654321",
		Batches: []Batch{
			{
				CompanyID:   "ORIGID0001",
				Description: "PRENOTE",
				Entries: []Entry{
					{TraceNumber: "1234560001", DFIAccount: "acct1", Routing: "012345678", Amount: 100, ReceiverName: "Alice"},
					{TraceNumber: "1234560002", DFIAccount: "acct2", Routing: "012345678", Amount: 250, ReceiverName: "Bob"},
				},
			},
		},
	}
	s, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(s); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
