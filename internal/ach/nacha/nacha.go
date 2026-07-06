// Package nacha builds NACHA PPD debit files (file header, batch header,
// entry detail, batch control, and file control records).
package nacha

import (
	"fmt"
	"strings"
	"time"
)

// File is a NACHA file composed of one or more batches.
type File struct {
	// ImmediateOrigin is the ODFI routing number (8 digits, no check digit).
	ImmediateOrigin string
	// ImmediateDestination is the bank partner routing number (8 digits).
	ImmediateDestination string
	// OriginName is the company name appearing on the file header.
	OriginName string
	// FileCreationDate overrides time.Now when non-zero.
	FileCreationDate time.Time
	// FileIDModifier distinguishes multiple files sent on the same day.
	FileIDModifier string
	// Batches is the set of batches contained in the file.
	Batches []*Batch
}

// Batch is a single NACHA batch of PPD debit entries.
type Batch struct {
	// CompanyName appears in the batch header.
	CompanyName string
	// CompanyDiscretionaryData is free-form, up to 20 chars.
	CompanyDiscretionaryData string
	// CompanyEntryDescription appears on the receiver's statement (e.g. "PAYMENT").
	CompanyEntryDescription string
	// OriginatingDFI is the 8-digit ODFI routing number for the batch.
	OriginatingDFI string
	// Entries are the detail entries in the batch.
	Entries []*EntryDetail
}

// EntryDetail is a single PPD debit entry.
type EntryDetail struct {
	// ReceiverName is the consumer's name.
	ReceiverName string
	// ReceiverDFI is the 9-digit receiver routing + check digit (ABA).
	ReceiverDFI string
	// AccountNumber is the receiver's bank account number.
	AccountNumber string
	// Amount is the debit amount in cents.
	Amount int64
	// IndividualID identifies the transaction on the receiver's statement.
	IndividualID string
	// TraceNumber is optional; if zero it is assigned by Encode from
	// OriginatingDFI + a sequence counter.
	TraceNumber string
}

const (
	recordSize       = 94
	blockingFactor   = 10
	formatCode       = "1"
	headerRecordID   = "1"
	batchRecordID    = "5"
	entryRecordID    = "6"
	batchCtrlID      = "8"
	fileCtrlID       = "9"
	ppdServiceClass  = "225" // debits only
	ppdEntryClass    = "PPD"
	transactionCode  = "27" // checking debit
	originStatusCode = "1"
)

// Encode produces the full NACHA file as a string with \n line separators.
func (f *File) Encode() (string, error) {
	if err := f.validate(); err != nil {
		return "", err
	}
	creationDate := f.FileCreationDate
	if creationDate.IsZero() {
		creationDate = time.Now()
	}
	dateYYMMDD := creationDate.Format("060102")
	dateYYMMDDSpace := creationDate.Format("060102")

	var b strings.Builder
	// File header (record 1).
	b.WriteString(headerRecordID)
	b.WriteString(padRight(f.OriginName, 23))
	b.WriteString(padRight(f.ImmediateOrigin, 23))
	b.WriteString(dateYYMMDDSpace)
	b.WriteString(time.Now().Format("1504")) // file creation time
	b.WriteString(padRight(f.FileIDModifier, 1))
	b.WriteString(padRight(formatCode, 6)) // record size + blocking + format
	b.WriteString(padRight(blockingFactorString(), 6))
	b.WriteString(" " + padRight("", 39)) // reserved
	b.WriteString("\n")

	entryCount := 0
	blockCount := 1 // file header
	totalDebit := int64(0)
	var traceOrigin string
	for i, batch := range f.Batches {
		batchNum := i + 1
		// Batch header (record 5).
		b.WriteString(batchRecordID)
		b.WriteString(ppdServiceClass)
		b.WriteString(padRight(batch.CompanyName, 16))
		b.WriteString(padRight(batch.CompanyDiscretionaryData, 20))
		b.WriteString(padRight(batch.CompanyEntryDescription, 10))
		b.WriteString(dateYYMMDD)
		b.WriteString("   ")           // blank effective entry date separator
		b.WriteString(padRight("", 3)) // blank settlement date
		b.WriteString(padRight(originStatusCode, 1))
		b.WriteString(padRight(batch.OriginatingDFI, 8))
		b.WriteString(fmt.Sprintf("%07d", batchNum))
		b.WriteString(padRight(ppdEntryClass, 10))
		b.WriteString("\n")

		batchEntryCount := 0
		batchDebit := int64(0)
		entryHash := int64(0)
		for j, e := range batch.Entries {
			// Entry detail (record 6).
			b.WriteString(entryRecordID)
			b.WriteString(transactionCode)
			b.WriteString(padRight(e.ReceiverDFI[:8], 8))              // routing without check digit
			b.WriteString(string(e.ReceiverDFI[len(e.ReceiverDFI)-1])) // check digit
			b.WriteString(padRight(e.AccountNumber, 17))
			b.WriteString(fmt.Sprintf("%010d", e.Amount))
			b.WriteString(padRight(e.IndividualID, 15))
			b.WriteString(padRight(e.ReceiverName, 22))
			b.WriteString("   ") // discretionary data (3 chars)
			if e.TraceNumber == "" {
				e.TraceNumber = fmt.Sprintf("%s%08d", batch.OriginatingDFI, j+1)
			}
			b.WriteString(e.TraceNumber)
			b.WriteString("\n")

			batchEntryCount++
			entryCount++
			batchDebit += e.Amount
			totalDebit += e.Amount
			rd := e.ReceiverDFI
			if len(rd) >= 8 {
				entryHash += parseInt64(rd[:8])
			}
			traceOrigin = batch.OriginatingDFI
		}

		// Batch control (record 8), 94 chars.
		b.WriteString(batchCtrlID)                          // 1
		b.WriteString(ppdServiceClass)                      // 3  (1-3)
		b.WriteString(fmt.Sprintf("%06d", batchEntryCount)) // 6  (4-9)
		b.WriteString(fmt.Sprintf("%010d", entryHash))      // 10 (10-19)
		b.WriteString(fmt.Sprintf("%012d", batchDebit))     // 12 (20-31)
		b.WriteString(fmt.Sprintf("%012d", 0))              // 12 (32-43) credit total
		b.WriteString(padRight(batch.OriginatingDFI, 8))    // 8  (44-51) message auth code field reused for ODFI
		b.WriteString(padRight(traceOrigin, 8))             // 8  (52-59) ODFI
		b.WriteString(fmt.Sprintf("%07d", batchNum))        // 7  (60-66) batch number
		b.WriteString(padRight("", 27))                     // 27 (67-93) reserved
		b.WriteString("\n")
		blockCount += 1 + len(batch.Entries) + 1
	}

	// File control (record 9), 94 chars.
	b.WriteString(fileCtrlID)                          // 1
	b.WriteString(fmt.Sprintf("%06d", len(f.Batches))) // 6  (1-6)  batch count
	b.WriteString(fmt.Sprintf("%06d", blockCount))     // 6  (7-12) block count
	b.WriteString(fmt.Sprintf("%08d", entryCount))     // 8  (13-20) entry count
	b.WriteString(fmt.Sprintf("%010d", totalDebit))    // 10 (21-30) debit total
	b.WriteString(fmt.Sprintf("%012d", 0))             // 12 (31-42) credit total
	b.WriteString(padRight("", 51))                    // 51 (43-93) reserved
	b.WriteString("\n")

	// Pad to blocking factor of 10 records.
	padLines := (10 - (blockCount+1)%10) % 10
	for i := 0; i < padLines; i++ {
		b.WriteString(strings.Repeat("9", 94))
		b.WriteString("\n")
	}
	return b.String(), nil
}

// Validate checks the structural integrity of an encoded NACHA file:
// file/batch control totals balance and record counts match.
func Validate(s string) error {
	lines := splitLines(s)
	if len(lines) == 0 {
		return fmt.Errorf("nacha: empty file")
	}
	if len(lines[0]) < 94 || string(lines[0][0]) != headerRecordID {
		return fmt.Errorf("nacha: missing file header")
	}
	if string(lines[len(lines)-1][0]) == "9" && len(lines[len(lines)-1]) >= 94 && allNines(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1] // allow one trailing filler block line
	}

	var batches int
	fileBlockCount := 1 // include file header record
	fileEntryCount := 0
	fileDebitTotal := int64(0)
	i := 1 // skip file header
	for i < len(lines) {
		rec := lines[i]
		if len(rec) < 1 {
			return fmt.Errorf("nacha: empty record at line %d", i)
		}
		switch rec[0] {
		case '5':
			batches++
			i++
			batchEntryCount := 0
			batchDebitTotal := int64(0)
			batchHash := int64(0)
			entries := 0
			for i < len(lines) && string(lines[i][0]) == entryRecordID {
				if len(lines[i]) < 94 {
					return fmt.Errorf("nacha: short entry detail at line %d", i)
				}
				entries++
				amount := parseInt64(lines[i][29:39])
				batchDebitTotal += amount
				batchHash += parseInt64(lines[i][3:11])
				batchEntryCount++
				i++
			}
			if i >= len(lines) || string(lines[i][0]) != batchCtrlID {
				return fmt.Errorf("nacha: missing batch control at line %d", i)
			}
			ctrl := lines[i]
			if len(ctrl) < 94 {
				return fmt.Errorf("nacha: short batch control")
			}
			ctrlEntry := int(parseInt64(ctrl[4:10]))
			ctrlHash := parseInt64(ctrl[10:20])
			ctrlDebit := parseInt64(ctrl[20:32])
			if ctrlEntry != batchEntryCount {
				return fmt.Errorf("nacha: batch entry count mismatch (got %d, want %d)", ctrlEntry, batchEntryCount)
			}
			if ctrlHash != batchHash {
				return fmt.Errorf("nacha: batch hash mismatch (got %d, want %d)", ctrlHash, batchHash)
			}
			if ctrlDebit != batchDebitTotal {
				return fmt.Errorf("nacha: batch debit total mismatch (got %d, want %d)", ctrlDebit, batchDebitTotal)
			}
			i++
			fileBlockCount += 1 + entries + 1
			fileEntryCount += batchEntryCount
			fileDebitTotal += batchDebitTotal
		case '9':
			if len(rec) < 80 {
				return fmt.Errorf("nacha: short file control")
			}
			ctrlBatches := int(parseInt64(rec[1:7]))
			ctrlBlock := int(parseInt64(rec[7:13]))
			ctrlEntry := int(parseInt64(rec[13:21]))
			ctrlDebit := parseInt64(rec[21:31])
			if ctrlBatches != batches {
				return fmt.Errorf("nacha: file batch count mismatch (got %d, want %d)", ctrlBatches, batches)
			}
			if ctrlEntry != fileEntryCount {
				return fmt.Errorf("nacha: file entry count mismatch (got %d, want %d)", ctrlEntry, fileEntryCount)
			}
			if ctrlDebit != fileDebitTotal {
				return fmt.Errorf("nacha: file debit total mismatch (got %d, want %d)", ctrlDebit, fileDebitTotal)
			}
			if ctrlBlock != fileBlockCount {
				return fmt.Errorf("nacha: file block count mismatch (got %d, want %d)", ctrlBlock, fileBlockCount)
			}
			return nil
		default:
			return fmt.Errorf("nacha: unexpected record id %q at line %d", rec[0], i)
		}
	}
	return fmt.Errorf("nacha: missing file control")
}

func (f *File) validate() error {
	if len(f.ImmediateOrigin) != 8 {
		return fmt.Errorf("nacha: immediate origin must be 8 digits")
	}
	if len(f.ImmediateDestination) != 8 {
		return fmt.Errorf("nacha: immediate destination must be 8 digits")
	}
	if f.FileIDModifier == "" {
		f.FileIDModifier = "A"
	}
	if len(f.FileIDModifier) != 1 {
		return fmt.Errorf("nacha: file id modifier must be 1 char")
	}
	for i, b := range f.Batches {
		if len(b.OriginatingDFI) != 8 {
			return fmt.Errorf("nacha: batch %d originating DFI must be 8 digits", i)
		}
		for j, e := range b.Entries {
			if len(e.ReceiverDFI) != 9 {
				return fmt.Errorf("nacha: batch %d entry %d receiver DFI must be 9 digits", i, j)
			}
			if e.Amount < 0 {
				return fmt.Errorf("nacha: batch %d entry %d amount must be non-negative", i, j)
			}
		}
	}
	return nil
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

func parseInt64(s string) int64 {
	var v int64
	for _, c := range s {
		if c < '0' || c > '9' {
			c = '0'
		}
		v = v*10 + int64(c-'0')
	}
	return v
}

func blockingFactorString() string {
	return fmt.Sprintf("%d", blockingFactor)
}

func allNines(s string) bool {
	for _, c := range s {
		if c != '9' && c != '\n' {
			return false
		}
	}
	return true
}
