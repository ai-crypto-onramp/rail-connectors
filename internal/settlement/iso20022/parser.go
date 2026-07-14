// Package iso20022 is a settlement parser for ISO20022 camt.053 (statement)
// and camt.054 (debit advice) messages. It extracts settled entries
// (payment id, amount, currency, settled-at, source file reference) from the
// XML narrative.
package iso20022

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

// Entry is a single settled entry parsed from a camt.053/054 message.
type Entry struct {
	PaymentID string
	Rail      string
	Amount    float64
	Currency  string
	SettledAt time.Time
	SourceRef string
}

type txDetails struct {
	RmtInf struct {
		Strd struct {
			CdtrRefInf struct {
				Ref string `xml:"Ref"`
			} `xml:"CdtrRefInf"`
		} `xml:"Strd"`
	} `xml:"RmtInf"`
}

type ntry struct {
	Amt struct {
		Ccy string `xml:"Ccy,attr"`
		V   string `xml:",chardata"`
	} `xml:"Amt"`
	BookgDt struct {
		DtTm string `xml:"DtTm"`
	} `xml:"BookgDt"`
	NtryDtls struct {
		TxDtls []txDetails `xml:"TxDtls"`
	} `xml:"NtryDtls"`
}

type document struct {
	XMLName       xml.Name
	BkToCstmrStmt struct {
		Stmt []struct {
			Ntry []ntry `xml:"Ntry"`
		} `xml:"Stmt"`
	} `xml:"BkToCstmrStmt"`
	BkToCstmrDbtCdt struct {
		Rpt []struct {
			Ntry []ntry `xml:"Ntry"`
		} `xml:"Rpt"`
	} `xml:"BkToCstmrDbtCdt"`
}

// Parse parses a camt.053 or camt.054 XML document from r.
func Parse(r io.Reader, sourceRef string) ([]Entry, error) {
	var doc document
	dec := xml.NewDecoder(r)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("iso20022 settlement: decode: %w", err)
	}
	var entries []Entry
	for _, stmt := range doc.BkToCstmrStmt.Stmt {
		for _, n := range stmt.Ntry {
			entries = append(entries, entriesFromNtry(n, sourceRef)...)
		}
	}
	for _, rpt := range doc.BkToCstmrDbtCdt.Rpt {
		for _, n := range rpt.Ntry {
			entries = append(entries, entriesFromNtry(n, sourceRef)...)
		}
	}
	return entries, nil
}

func entriesFromNtry(n ntry, sourceRef string) []Entry {
	amt := parseFloat(n.Amt.V)
	t := parseTime(n.BookgDt.DtTm)
	var out []Entry
	for _, tx := range n.NtryDtls.TxDtls {
		ref := tx.RmtInf.Strd.CdtrRefInf.Ref
		if ref == "" {
			continue
		}
		out = append(out, Entry{
			PaymentID: ref,
			Rail:      "sepa",
			Amount:    amt,
			Currency:  strings.ToUpper(n.Amt.Ccy),
			SettledAt: t,
			SourceRef: sourceRef,
		})
	}
	return out
}

func parseFloat(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(strings.TrimSpace(s), "%f", &f)
	return f
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	formats := []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Now().UTC()
}
