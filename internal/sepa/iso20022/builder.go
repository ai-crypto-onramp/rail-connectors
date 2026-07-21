// Package iso20022 builds minimal ISO20022 pain.001.001.09 customer credit
// transfer initiation messages.
package iso20022

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Document is the root pain.001.001.09 document.
type Document struct {
	XMLName          xml.Name         `xml:"Document"`
	Xmlns            string           `xml:"xmlns,attr"`
	CstmrCdtTrfInitn CstmrCdtTrfInitn `xml:"CstmrCdtTrfInitn"`
}

// CstmrCdtTrfInitn is the customer credit transfer initiation.
type CstmrCdtTrfInitn struct {
	GrpHdr GrpHdr `xml:"GrpHdr"`
	PmtInf PmtInf `xml:"PmtInf"`
}

// GrpHdr is the group header.
type GrpHdr struct {
	MsgID    string `xml:"MsgId"`
	CreDtTm  string `xml:"CreDtTm"`
	NbOfTxs  string `xml:"NbOfTxs"`
	CtrlSum  string `xml:"CtrlSum"`
	InitgPty Pty    `xml:"InitgPty>Nm"`
}

// Pty is a party name.
type Pty struct {
	Nm string `xml:"Nm"`
}

// PmtInf is the payment information block.
type PmtInf struct {
	PmtInfID    string        `xml:"PmtInfId"`
	PmtMtd      string        `xml:"PmtMtd"` // "TRF"
	NbOfTxs     string        `xml:"NbOfTxs,omitempty"`
	CtrlSum     string        `xml:"CtrlSum,omitempty"`
	PmtTpInf    PmtTpInf      `xml:"PmtTpInf"`
	ReqdExctnDt string        `xml:"ReqdExctnDt"`
	Dbtr        Party         `xml:"Dbtr"`
	DbtrAcct    Acct          `xml:"DbtrAcct"`
	CdtTrfTxInf []CdtTrfTxInf `xml:"CdtTrfTxInf"`
}

// PmtTpInf is the payment type information.
type PmtTpInf struct {
	SvcLvl   SvcLvl `xml:"SvcLvl"`
	CtgyPurp string `xml:"CtgyPurp,omitempty"`
}

// SvcLvl is the service level.
type SvcLvl struct {
	Cd string `xml:"Cd"`
}

// Party is a debtor/creditor party.
type Party struct {
	Nm string `xml:"Nm"`
}

// Acct is an account.
type Acct struct {
	ID AcctID `xml:"Id"`
}

// AcctID is an account identifier (IBAN).
type AcctID struct {
	IBAN string `xml:"IBAN"`
}

// CdtTrfTxInf is a credit transfer transaction info.
type CdtTrfTxInf struct {
	PmtID    PmtID  `xml:"PmtId"`
	Amt      Amt    `xml:"Amt"`
	Cdtr     Party  `xml:"Cdtr"`
	CdtrAcct Acct   `xml:"CdtrAcct"`
	RmtInf   RmtInf `xml:"RmtInf"`
}

// PmtID is the payment identification.
type PmtID struct {
	EndToEndID string `xml:"EndToEndId"`
	UETR       string `xml:"UETR,omitempty"`
}

// Amt is the instructed amount.
type Amt struct {
	InstdAmt InstdAmt `xml:"InstdAmt"`
}

// InstdAmt is the instructed amount with currency.
type InstdAmt struct {
	Ccy string `xml:"Ccy,attr"`
	V   string `xml:",chardata"`
}

// RmtInf is the remittance information.
type RmtInf struct {
	Strd Strd `xml:"Strd"`
}

// Strd is structured remittance.
type Strd struct {
	CdtrRefInf CdtrRefInf `xml:"CdtrRefInf"`
}

// CdtrRefInf is the creditor reference information.
type CdtrRefInf struct {
	Ref string `xml:"Ref"`
}

// Payment is a simplified input for building a pain.001 message.
type Payment struct {
	MsgID         string
	Initiator     string
	DebtorName    string
	DebtorIBAN    string
	PmtInfID      string
	ExecutionDate time.Time
	Currency      string
	Transfers     []Transfer
}

// Transfer is a single credit transfer.
type Transfer struct {
	EndToEndID   string
	CreditorName string
	CreditorIBAN string
	Amount       decimal.Decimal
	Reference    string
}

// BuildPain001 builds a pain.001.001.09 XML document string for the given
// payment. The idempotency key is used as the message id when present.
func BuildPain001(p Payment) (string, error) {
	if p.MsgID == "" {
		return "", fmt.Errorf("iso20022: missing MsgID")
	}
	if len(p.Transfers) == 0 {
		return "", fmt.Errorf("iso20022: no transfers")
	}
	total := decimal.Zero
	for _, tr := range p.Transfers {
		total = total.Add(tr.Amount)
	}
	twoPlaces := decimal.NewFromInt(100)
	ctrlSum := total.Mul(twoPlaces).String()
	doc := Document{
		Xmlns: "urn:iso:std:iso:20022:tech:xsd:pain.001.001.09",
		CstmrCdtTrfInitn: CstmrCdtTrfInitn{
			GrpHdr: GrpHdr{
				MsgID:    p.MsgID,
				CreDtTm:  time.Now().UTC().Format(time.RFC3339),
				NbOfTxs:  fmt.Sprintf("%d", len(p.Transfers)),
				CtrlSum:  ctrlSum,
				InitgPty: Pty{Nm: p.Initiator},
			},
			PmtInf: PmtInf{
				PmtInfID:    orDefault(p.PmtInfID, p.MsgID),
				PmtMtd:      "TRF",
				NbOfTxs:     fmt.Sprintf("%d", len(p.Transfers)),
				CtrlSum:     ctrlSum,
				PmtTpInf:    PmtTpInf{SvcLvl: SvcLvl{Cd: "INST"}},
				ReqdExctnDt: p.ExecutionDate.Format("2006-01-02"),
				Dbtr:        Party{Nm: p.DebtorName},
				DbtrAcct:    Acct{ID: AcctID{IBAN: p.DebtorIBAN}},
			},
		},
	}
	for _, tr := range p.Transfers {
		doc.CstmrCdtTrfInitn.PmtInf.CdtTrfTxInf = append(doc.CstmrCdtTrfInitn.PmtInf.CdtTrfTxInf, CdtTrfTxInf{
			PmtID:    PmtID{EndToEndID: tr.EndToEndID},
			Amt:      Amt{InstdAmt: InstdAmt{Ccy: p.Currency, V: tr.Amount.StringFixed(2)}},
			Cdtr:     Party{Nm: tr.CreditorName},
			CdtrAcct: Acct{ID: AcctID{IBAN: tr.CreditorIBAN}},
			RmtInf:   RmtInf{Strd: Strd{CdtrRefInf: CdtrRefInf{Ref: tr.Reference}}},
		})
	}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	return xml.Header + string(out) + "\n", nil
}

// Validate checks a pain.001 XML string structurally: root element name and
// required fields are present.
func Validate(s string) error {
	if !strings.Contains(s, "<Document") {
		return fmt.Errorf("iso20022: missing Document root")
	}
	if !strings.Contains(s, "pain.001.001.09") {
		return fmt.Errorf("iso20022: missing pain.001.001.09 namespace")
	}
	if !strings.Contains(s, "<CstmrCdtTrfInitn>") {
		return fmt.Errorf("iso20022: missing CstmrCdtTrfInitn")
	}
	if !strings.Contains(s, "<GrpHdr>") {
		return fmt.Errorf("iso20022: missing GrpHdr")
	}
	if !strings.Contains(s, "<PmtInf>") {
		return fmt.Errorf("iso20022: missing PmtInf")
	}
	if !strings.Contains(s, "<CdtTrfTxInf>") {
		return fmt.Errorf("iso20022: missing CdtTrfTxInf")
	}
	return nil
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
