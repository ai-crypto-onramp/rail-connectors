// Package nacha implements a minimal NACHA file encoder for PPD debit
// batches: file header, batch header, entry detail, batch control, and file
// control records.
package nacha

import (
	"fmt"
	"strings"
	"time"
)

// File is a NACHA file composed of a header, one or more batches of entry
// details, and a file control record.
type File struct {
	ImmediateOrigin string // 10 digits
	ImmediateDest   string // 10 digits
	OriginName      string // up to 23 chars
	DestName        string // up to 23 chars
	FileIDModifier  string // single char A-Z, default 'A'
	Batches         []Batch
}

// Batch is a single batch of PPD debit entries.
type Batch struct {
	CompanyID   string // 10 digits, originator company id
	CompanyName string // up to 16 chars
	Description string // up to 10 chars, e.g. "PRENOTE"
	Entries     []Entry
}

// Entry is a single entry detail record (PPD debit).
type Entry struct {
	TraceNumber  string // up to 15 digits
	DFIAccount   string // receiver account
	Routing      string // 9 digits
	Amount       int64  // cents
	ReceiverName string // up to 22 chars
}

const (
	recordSize   = 94
	blockSize    = 10
	fileHeader   = "1"
	batchHeader  = "5"
	entryDetail  = "6"
	batchControl = "8"
	fileControl  = "9"
	padChar      = " "
)

// Encode renders the NACHA file as a single string with one 94-character
// record per line. The trailing block is padded with 9s-filled filler
// records so the total record count is a multiple of blockSize.
func (f File) Encode() (string, error) {
	if err := f.validate(); err != nil {
		return "", err
	}
	var b strings.Builder
	mod := f.FileIDModifier
	if mod == "" {
		mod = "A"
	}
	// File header (record type 1).
	b.WriteString(fileHeader)
	b.WriteString(padRight(f.OriginName, 23))
	b.WriteString(padRight(f.DestName, 23))
	b.WriteString(f.ImmediateOrigin)
	b.WriteString(f.ImmediateDest)
	b.WriteString("1010") // format code + priority + blank + blank
	b.WriteString(strings.ToUpper(mod))
	b.WriteString(strings.Repeat(" ", 39))
	b.WriteString("\n")

	var totalEntries int
	var totalAmount int64
	var totalHash int64

	for bi, batch := range f.Batches {
		entries := len(batch.Entries)
		totalEntries += entries
		var batchAmount int64
		var batchHash int64
		for _, e := range batch.Entries {
			batchAmount += e.Amount
			batchHash += routingHash(e.Routing)
		}
		totalAmount += batchAmount
		totalHash += batchHash

		// Batch header (record type 5).
		b.WriteString(batchHeader)
		b.WriteString("225") // PPD + debit
		b.WriteString(fmt.Sprintf("%02d", bi+1))
		b.WriteString(padRight(batch.CompanyName, 16))
		b.WriteString(strings.Repeat(" ", 20))
		b.WriteString(batch.CompanyID)
		b.WriteString("1") // standard
		b.WriteString(padRight(batch.Description, 10))
		b.WriteString(formatDate(time.Now()))
		b.WriteString(strings.Repeat(" ", 3)) // julian settle date blank
		b.WriteString("1")                    // origin status
		b.WriteString(f.ImmediateOrigin)
		b.WriteString(strings.Repeat(" ", 7))
		b.WriteString("\n")

		// Entry detail records (record type 6).
		for _, e := range batch.Entries {
			b.WriteString(entryDetail)
			b.WriteString("22") // checking + debit
			b.WriteString(e.Routing)
			b.WriteString(padRight(e.DFIAccount, 17))
			b.WriteString(fmt.Sprintf("%010d", e.Amount))
			b.WriteString(padRight(e.ReceiverName, 22))
			b.WriteString(strings.Repeat(" ", 2)) // discretionary
			b.WriteString("0")                    // addenda none
			b.WriteString(padLeft(e.TraceNumber, 15))
			b.WriteString("\n")
		}

		// Batch control (record type 8).
		b.WriteString(batchControl)
		b.WriteString(fmt.Sprintf("%06d", entries+1))
		b.WriteString(fmt.Sprintf("%012d", batchAmount))
		b.WriteString(fmt.Sprintf("%010d", batchHash))
		b.WriteString(f.ImmediateOrigin)
		b.WriteString(strings.Repeat(" ", 25))
		b.WriteString("\n")
	}

	// File control (record type 9).
	b.WriteString(fileControl)
	b.WriteString(fmt.Sprintf("%06d", totalEntries+len(f.Batches)*2+2))
	b.WriteString(fmt.Sprintf("%012d", totalAmount))
	b.WriteString(fmt.Sprintf("%010d", totalHash))
	b.WriteString(strings.Repeat(" ", 39))
	b.WriteString("\n")

	// Pad to block multiple.
	records := totalEntries + len(f.Batches)*2 + 2
	pad := blockSize - (records % blockSize)
	if pad != blockSize {
		for i := 0; i < pad; i++ {
			b.WriteString(strings.Repeat("9", recordSize))
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

func (f File) validate() error {
	if len(f.ImmediateOrigin) != 10 {
		return fmt.Errorf("nacha: immediate origin must be 10 chars, got %d", len(f.ImmediateOrigin))
	}
	if len(f.ImmediateDest) != 10 {
		return fmt.Errorf("nacha: immediate dest must be 10 chars, got %d", len(f.ImmediateDest))
	}
	for bi, batch := range f.Batches {
		if len(batch.CompanyID) != 10 {
			return fmt.Errorf("nacha: batch %d company id must be 10 chars", bi)
		}
		for ei, e := range batch.Entries {
			if len(e.Routing) != 9 {
				return fmt.Errorf("nacha: entry %d.%d routing must be 9 digits", bi, ei)
			}
			if e.Amount < 0 {
				return fmt.Errorf("nacha: entry %d.%d negative amount", bi, ei)
			}
		}
	}
	return nil
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(padChar, n-len(s))
}

func padLeft(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat("0", n-len(s)) + s
}

func formatDate(t time.Time) string {
	return t.Format("0602") // YYMMDD
}

func routingHash(routing string) int64 {
	if len(routing) < 8 {
		return 0
	}
	var sum int64
	for _, c := range routing[:8] {
		if c >= '0' && c <= '9' {
			sum += int64(c - '0')
		}
	}
	return sum
}

// Validate performs structural validation of an encoded NACHA file string:
// record counts balance and control totals match. Returns nil on success.
func Validate(s string) error {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) == 0 {
		return fmt.Errorf("nacha: empty file")
	}
	if !strings.HasPrefix(lines[0], fileHeader) {
		return fmt.Errorf("nacha: missing file header")
	}
	if !strings.HasPrefix(lines[len(lines)-1], fileControl) && !strings.HasPrefix(lines[len(lines)-1], strings.Repeat("9", recordSize)) {
		return fmt.Errorf("nacha: missing file control")
	}

	var batchCount, entryCount, totalAmount int64
	var totalHash int64
	controlSeen := false
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		switch {
		case strings.HasPrefix(line, batchHeader):
			batchCount++
		case strings.HasPrefix(line, entryDetail):
			entryCount++
			if len(line) > 20 {
				amt, _ := parseInt(line[29:39])
				totalAmount += amt
			}
			if len(line) >= 11 {
				totalHash += routingHash(line[3:12])
			}
		case strings.HasPrefix(line, fileControl) && !controlSeen:
			controlSeen = true
			if len(line) < 39 {
				continue
			}
			recCount, _ := parseInt(line[1:7])
			amount, _ := parseInt(line[7:19])
			hash, _ := parseInt(line[19:29])
			wantRecords := entryCount + batchCount*2 + 2
			if recCount != wantRecords {
				return fmt.Errorf("nacha: control record count %d != expected %d", recCount, wantRecords)
			}
			if amount != totalAmount {
				return fmt.Errorf("nacha: control amount %d != expected %d", amount, totalAmount)
			}
			if hash != totalHash {
				return fmt.Errorf("nacha: control hash %d != expected %d", hash, totalHash)
			}
		}
	}
	return nil
}

func parseInt(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-digit %q", c)
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}
