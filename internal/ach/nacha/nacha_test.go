package nacha

import "testing"

func TestEncodeAndValidate_HappyPath(t *testing.T) {
	f := &File{
		ImmediateOrigin:      "02100008",
		ImmediateDestination: "02600001",
		OriginName:           "ONRAMP",
		Batches: []*Batch{{
			CompanyName:             "ONRAMP",
			CompanyEntryDescription: "PAYMENT",
			OriginatingDFI:          "02100008",
			Entries: []*EntryDetail{
				{
					ReceiverName:  "ALICE",
					ReceiverDFI:   "123456789",
					AccountNumber: "000111222",
					Amount:        1500,
					IndividualID:  "tx-1",
				},
				{
					ReceiverName:  "BOB",
					ReceiverDFI:   "987654321",
					AccountNumber: "000333444",
					Amount:        2750,
					IndividualID:  "tx-2",
				},
			},
		}},
	}
	out, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if err := Validate(out); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got, want := countLines(out), 10; got != 10 {
		t.Errorf("line count: got %d want %d (blocking factor 10)", got, want)
	}
}

func TestValidate_EntryCountMismatch(t *testing.T) {
	f := &File{
		ImmediateOrigin:      "02100008",
		ImmediateDestination: "02600001",
		OriginName:           "ONRAMP",
		Batches: []*Batch{{
			CompanyName:             "ONRAMP",
			CompanyEntryDescription: "PAYMENT",
			OriginatingDFI:          "02100008",
			Entries: []*EntryDetail{
				{ReceiverName: "ALICE", ReceiverDFI: "123456789", AccountNumber: "1", Amount: 100},
			},
		}},
	}
	out, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Tamper with batch control entry count (position 7..12).
	lines := splitLines(out)
	lines[2] = lines[2][:6] + "000009" + lines[2][12:]
	if err := Validate(joinLines(lines)); err == nil {
		t.Fatalf("expected validation error for tampered entry count")
	}
}

func TestValidate_DebitMismatch(t *testing.T) {
	f := &File{
		ImmediateOrigin:      "02100008",
		ImmediateDestination: "02600001",
		OriginName:           "ONRAMP",
		Batches: []*Batch{{
			CompanyName:             "ONRAMP",
			CompanyEntryDescription: "PAYMENT",
			OriginatingDFI:          "02100008",
			Entries: []*EntryDetail{
				{ReceiverName: "ALICE", ReceiverDFI: "123456789", AccountNumber: "1", Amount: 100},
			},
		}},
	}
	out, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Tamper with batch control debit total (position 22..34).
	lines := splitLines(out)
	lines[2] = lines[2][:22] + "000000009999" + lines[2][34:]
	if err := Validate(joinLines(lines)); err == nil {
		t.Fatalf("expected validation error for tampered debit total")
	}
}

func TestEncode_ValidationErrors(t *testing.T) {
	if _, err := (&File{ImmediateOrigin: "1"}).Encode(); err == nil {
		t.Fatal("expected error for bad immediate origin")
	}
	if _, err := (&File{
		ImmediateOrigin:      "02100008",
		ImmediateDestination: "02600001",
		Batches: []*Batch{{
			OriginatingDFI: "12",
			Entries: []*EntryDetail{
				{ReceiverDFI: "123456789", Amount: 1},
			},
		}},
	}).Encode(); err == nil {
		t.Fatal("expected error for bad originating DFI")
	}
	if _, err := (&File{
		ImmediateOrigin:      "02100008",
		ImmediateDestination: "02600001",
		Batches: []*Batch{{
			OriginatingDFI: "02100008",
			Entries: []*EntryDetail{
				{ReceiverDFI: "12", Amount: 1},
			},
		}},
	}).Encode(); err == nil {
		t.Fatal("expected error for bad receiver DFI")
	}
}

func countLines(s string) int {
	return len(splitLines(s))
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out + "\n"
}
