package adyen

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(srv.URL, "akey", "TestMerch"), srv
}

func TestAuthorizePaymentHappy(t *testing.T) {
	t.Parallel()
	var gotPath, gotID, gotAuth, gotCT string
	var gotBody []byte
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotID = r.Header.Get("Idempotency-Key")
		gotAuth = r.Header.Get("X-API-Key")
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pspReference":"PSP1","resultCode":"Authorised"}`))
	})
	pr, err := c.AuthorizePayment(context.Background(), "ref-1", 1500, "eur", "idem-1")
	if err != nil {
		t.Fatal(err)
	}
	if pr.PSPReference != "PSP1" || pr.ResultCode != "Authorised" {
		t.Fatalf("pr = %+v", pr)
	}
	if gotPath != "/payments" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotID != "idem-1" {
		t.Fatalf("idem = %q", gotID)
	}
	if gotAuth != "akey" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Fatalf("ct = %q", gotCT)
	}
	if !strings.Contains(string(gotBody), `"merchantAccount":"TestMerch"`) || !strings.Contains(string(gotBody), `"reference":"ref-1"`) {
		t.Fatalf("body = %q", gotBody)
	}
	if !strings.Contains(string(gotBody), `"value":1500`) {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestAuthorizePaymentError(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"status":403,"errorCode":"401","message":"forbidden"}`))
	})
	_, err := c.AuthorizePayment(context.Background(), "r", 1, "eur", "k")
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if ae.Status != http.StatusForbidden || ae.Code != "401" || ae.Msg != "forbidden" {
		t.Fatalf("ae = %+v", ae)
	}
	if DeclineCode(err) != "401" {
		t.Fatalf("DeclineCode = %q", DeclineCode(err))
	}
	if !strings.Contains(ae.Error(), "403") {
		t.Fatalf("error string = %q", ae.Error())
	}
}

func TestGetPaymentHappy(t *testing.T) {
	t.Parallel()
	var gotPath string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pspReference":"PSP1","resultCode":"Settled"}`))
	})
	pr, err := c.GetPayment(context.Background(), "PSP1")
	if err != nil {
		t.Fatal(err)
	}
	if pr.ResultCode != "Settled" {
		t.Fatalf("resultCode = %q", pr.ResultCode)
	}
	if gotPath != "/payments/PSP1" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestCapturePaymentHappy(t *testing.T) {
	t.Parallel()
	var gotPath string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pspReference":"PSP1","status":"received"}`))
	})
	cr, err := c.CapturePayment(context.Background(), "PSP1", 500, "eur", "k")
	if err != nil {
		t.Fatal(err)
	}
	if cr.Status != "received" || cr.PSPReference != "PSP1" {
		t.Fatalf("cr = %+v", cr)
	}
	if gotPath != "/payments/PSP1/captures" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestRefundPaymentHappy(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/payments/PSP1/refunds" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"pspReference":"PSP1","status":"received"}`))
	})
	rr, err := c.RefundPayment(context.Background(), "PSP1", 250, "eur", "k")
	if err != nil {
		t.Fatal(err)
	}
	if rr.Status != "received" {
		t.Fatalf("status = %q", rr.Status)
	}
}

func TestCapturePaymentError(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorCode":"E1","message":"bad"}`))
	})
	_, err := c.CapturePayment(context.Background(), "PSP1", 1, "eur", "k")
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*APIError)
	if !ok || ae.Code != "E1" {
		t.Fatalf("err = %+v", err)
	}
}

func TestRefundPaymentError(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`not json`))
	})
	_, err := c.RefundPayment(context.Background(), "PSP1", 1, "eur", "k")
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
}

func TestGetPaymentError(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.GetPayment(context.Background(), "ghost")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTransportError(t *testing.T) {
	t.Parallel()
	c := New("http://127.0.0.1:0", "k", "m")
	if _, err := c.GetPayment(context.Background(), "x"); err == nil {
		t.Fatal("expected transport error")
	}
}

func TestInvalidJSONResponse(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{bad json`))
	})
	if _, err := c.GetPayment(context.Background(), "x"); err == nil {
		t.Fatal("expected json error")
	}
}

func TestProcessorWrappers(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/payments":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"pspReference":"PSP1","resultCode":"Authorised"}`))
		case "/payments/PSP1/captures":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"pspReference":"PSP1","status":"received"}`))
		case "/payments/PSP1/refunds":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"pspReference":"PSP1","status":"received"}`))
		case "/payments/PSP1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"pspReference":"PSP1","resultCode":"Settled"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	railRef, status, err := c.Authorize(context.Background(), "ref", 100, "eur", "k")
	if err != nil {
		t.Fatal(err)
	}
	if railRef != "PSP1" || status != "Authorised" {
		t.Fatalf("authorize = %q %q", railRef, status)
	}
	st, err := c.Capture(context.Background(), "PSP1", 100, "eur", "k")
	if err != nil {
		t.Fatal(err)
	}
	if st != "received" {
		t.Fatalf("capture = %q", st)
	}
	st, err = c.Refund(context.Background(), "PSP1", 100, "eur", "k")
	if err != nil {
		t.Fatal(err)
	}
	if st != "received" {
		t.Fatalf("refund = %q", st)
	}
	st, err = c.Status(context.Background(), "PSP1")
	if err != nil {
		t.Fatal(err)
	}
	if st != "Settled" {
		t.Fatalf("status = %q", st)
	}
	if c.DeclineCode(nil) != "" {
		t.Fatal("DeclineCode(nil) should be empty")
	}
}

func TestProcessorAuthorizeError(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorCode":"E1","message":"x"}`))
	})
	_, _, err := c.Authorize(context.Background(), "ref", 100, "eur", "k")
	if err == nil {
		t.Fatal("expected error")
	}
	if DeclineCode(err) != "E1" {
		t.Fatalf("decline = %q", DeclineCode(err))
	}
}

func TestProcessorCaptureError(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := c.Capture(context.Background(), "PSP1", 100, "eur", "k")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewClientDefaults(t *testing.T) {
	t.Parallel()
	c := New("https://x.test", "k", "m")
	if c.BaseURL != "https://x.test" || c.APIKey != "k" || c.Merchant != "m" {
		t.Fatalf("client = %+v", c)
	}
	if c.HTTP == nil {
		t.Fatal("HTTP nil")
	}
}