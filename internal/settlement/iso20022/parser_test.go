package iso20022

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

const camt053 = `<?xml version="1.0"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.02">
  <BkToCstmrStmt>
    <Stmt>
      <Ntry>
        <Amt Ccy="EUR">100.50</Amt>
        <BookgDt><DtTm>2026-07-14T12:00:00Z</DtTm></BookgDt>
        <NtryDtls>
          <TxDtls>
            <RmtInf><Strd><CdtrRefInf><Ref>E2E-1</Ref></CdtrRefInf></Strd></RmtInf>
          </TxDtls>
        </NtryDtls>
      </Ntry>
      <Ntry>
        <Amt Ccy="EUR">200.00</Amt>
        <BookgDt><DtTm>2026-07-14T12:01:00Z</DtTm></BookgDt>
        <NtryDtls>
          <TxDtls>
            <RmtInf><Strd><CdtrRefInf><Ref>E2E-2</Ref></CdtrRefInf></Strd></RmtInf>
          </TxDtls>
        </NtryDtls>
      </Ntry>
    </Stmt>
  </BkToCstmrStmt>
</Document>`

const camt054 = `<?xml version="1.0"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.054.001.02">
  <BkToCstmrDbtCdt>
    <Rpt>
      <Ntry>
        <Amt Ccy="EUR">50.00</Amt>
        <BookgDt><DtTm>2026-07-14T13:00:00Z</DtTm></BookgDt>
        <NtryDtls>
          <TxDtls>
            <RmtInf><Strd><CdtrRefInf><Ref>E2E-3</Ref></CdtrRefInf></Strd></RmtInf>
          </TxDtls>
        </NtryDtls>
      </Ntry>
    </Rpt>
  </BkToCstmrDbtCdt>
</Document>`

func TestParseCamt053(t *testing.T) {
	t.Parallel()
	entries, err := Parse(strings.NewReader(camt053), "stmt.xml")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d", len(entries))
	}
	if entries[0].PaymentID != "E2E-1" || !entries[0].Amount.Equal(decimal.NewFromFloat(100.50)) {
		t.Fatalf("entry0 = %+v", entries[0])
	}
	if entries[0].Currency != "EUR" {
		t.Fatalf("currency = %q", entries[0].Currency)
	}
	if entries[0].SourceRef != "stmt.xml" {
		t.Fatalf("source = %q", entries[0].SourceRef)
	}
}

func TestParseCamt054(t *testing.T) {
	t.Parallel()
	entries, err := Parse(strings.NewReader(camt054), "advice.xml")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d", len(entries))
	}
	if entries[0].PaymentID != "E2E-3" {
		t.Fatalf("entry0 = %+v", entries[0])
	}
}

func TestParseBadXML(t *testing.T) {
	t.Parallel()
	if _, err := Parse(strings.NewReader("not xml"), "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseEmptyRefSkipped(t *testing.T) {
	t.Parallel()
	xml := `<?xml version="1.0"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.02">
  <BkToCstmrStmt><Stmt><Ntry>
    <Amt Ccy="EUR">10</Amt>
    <BookgDt><DtTm>2026-07-14T12:00:00Z</DtTm></BookgDt>
    <NtryDtls><TxDtls><RmtInf><Strd><CdtrRefInf><Ref></Ref></CdtrRefInf></Strd></RmtInf></TxDtls></NtryDtls>
  </Ntry></Stmt></BkToCstmrStmt>
</Document>`
	entries, err := Parse(strings.NewReader(xml), "x")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}
