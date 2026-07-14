package gateway

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSubmitPain001Happy(t *testing.T) {
	t.Parallel()
	var gotPath, gotID, gotCT, gotKey string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotID = r.Header.Get("Idempotency-Key")
		gotCT = r.Header.Get("Content-Type")
		gotKey = r.Header.Get("X-API-Key")
		gotBody = readAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Document xmlns="urn:iso:std:iso:20022:tech:xsd:pain.001.001.09"><CstmrCdtTrfInitn><GrpHdr><MsgId>MSG1</MsgId></GrpHdr><PmtInf><CdtTrfTxInf><PmtId><EndToEndId>E2E1</EndToEndId></PmtId></CdtTrfTxInf></PmtInf></CstmrCdtTrfInitn></Document>`))
	}))
	defer srv.Close()
	c, err := New(srv.URL, "apikey", "", "")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.SubmitPain001(context.Background(), []byte("<xml/>"), "idem-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.MsgID != "MSG1" {
		t.Fatalf("MsgID = %q", resp.MsgID)
	}
	if gotPath != "/v1/sepa/pain.001" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotID != "idem-1" {
		t.Fatalf("idem = %q", gotID)
	}
	if gotCT != "application/xml" {
		t.Fatalf("ct = %q", gotCT)
	}
	if gotKey != "apikey" {
		t.Fatalf("key = %q", gotKey)
	}
	if string(gotBody) != "<xml/>" {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestSubmitPain001ReasonCode(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`<Rsn>AM04</Rsn>`))
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "k", "", "")
	_, err := c.SubmitPain001(context.Background(), []byte("x"), "i")
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if ae.Code != "AM04" {
		t.Fatalf("code = %q", ae.Code)
	}
	if DeclineCode(err) != "AM04" {
		t.Fatal("DeclineCode wrong")
	}
}

func TestSubmitPain001ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "k", "", "")
	_, err := c.SubmitPain001(context.Background(), []byte("x"), "i")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("err = %v", err)
	}
}

func TestGetPain002Happy(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0"?><Document><CstmrPmtStsRpt><OrgnlPmtInf><TxInf><OrgnlEndToEndId>E2E1</OrgnlEndToEndId><TxSts>ACSC</TxSts></TxInf></OrgnlPmtInf></CstmrPmtStsRpt></Document>`))
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "k", "", "")
	st, err := c.GetPain002(context.Background(), "E2E1")
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "ACSC" {
		t.Fatalf("status = %q", st.Status)
	}
	if gotPath != "/v1/sepa/pain.002/E2E1" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestGetPain002NotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "k", "", "")
	_, err := c.GetPain002(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "NOT_FOUND") {
		t.Fatalf("err = %v", err)
	}
}

func TestMTLSConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	certPath, keyPath := writeSelfSignedCert(t, dir)
	c, err := New("https://localhost:0", "k", certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTP.Transport == nil {
		t.Fatal("expected custom transport for mTLS")
	}
	if !MTLSCertExists(certPath) {
		t.Fatal("MTLSCertExists should report true")
	}
	if MTLSCertExists(filepath.Join(dir, "nope.pem")) {
		t.Fatal("MTLSCertExists should report false")
	}
	// Missing cert returns plain client
	c2, err := New(srv0URL(), "k", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if c2.HTTP.Transport != nil {
		t.Fatal("expected nil transport without mTLS")
	}
}

func srv0URL() string { return "http://localhost:0" }

func TestNewMTLSBadCert(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	_ = os.WriteFile(certPath, []byte("not a cert"), 0600)
	_ = os.WriteFile(keyPath, []byte("not a key"), 0600)
	_, err := New("https://localhost:0", "k", certPath, keyPath)
	if err == nil {
		t.Fatal("expected error for bad cert")
	}
}

func writeSelfSignedCert(t *testing.T, dir string) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func readAll(r interface{ Read([]byte) (int, error) }) []byte {
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	return buf[:n]
}

func TestAPIErrorString(t *testing.T) {
	t.Parallel()
	e := &APIError{Status: 500, Code: "X", Msg: "boom"}
	if !strings.Contains(e.Error(), "500") {
		t.Fatal("missing status")
	}
	_ = fmt.Sprint(e)
}
