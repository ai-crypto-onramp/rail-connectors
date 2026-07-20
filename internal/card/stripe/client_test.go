package stripe

import (
	"context"
	"encoding/json"
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
	c := New(srv.URL, "sk_test")
	return c, srv
}

func TestCreatePaymentIntentHappy(t *testing.T) {
	t.Parallel()
	var gotPath, gotID, gotAuth, gotCT string
	var gotBody []byte
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotID = r.Header.Get("Idempotency-Key")
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"pi_1","status":"requires_capture","latest_charge":"ch_1","client_secret":"sec"}`))
	})
	pi, err := c.CreatePaymentIntent(context.Background(), 1500, "usd", "idem-1")
	if err != nil {
		t.Fatal(err)
	}
	if pi.ID != "pi_1" || pi.Status != "requires_capture" || pi.Charge != "ch_1" || pi.Receipt != "sec" {
		t.Fatalf("pi = %+v", pi)
	}
	if gotPath != "/v1/payment_intents" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotID != "idem-1" {
		t.Fatalf("idem = %q", gotID)
	}
	if gotAuth != "Bearer sk_test" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotCT != "application/x-www-form-urlencoded" {
		t.Fatalf("ct = %q", gotCT)
	}
	if !strings.Contains(string(gotBody), "amount=1500") || !strings.Contains(string(gotBody), "currency=usd") {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestCreatePaymentIntentError(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error":{"type":"card_error","code":"card_declined","decline_code":"insufficient_funds","message":"no funds"}}`))
	})
	_, err := c.CreatePaymentIntent(context.Background(), 100, "usd", "k")
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if ae.Status != http.StatusPaymentRequired || ae.Code != "card_declined" || ae.Decline != "insufficient_funds" || ae.Msg != "no funds" {
		t.Fatalf("ae = %+v", ae)
	}
	if DeclineCode(err) != "insufficient_funds" {
		t.Fatal("DeclineCode wrong")
	}
	if !strings.Contains(ae.Error(), "402") {
		t.Fatalf("error string = %q", ae.Error())
	}
}

func TestCreateCaptureHappy(t *testing.T) {
	t.Parallel()
	var gotPath string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ch_1","status":"succeeded"}`))
	})
	cr, err := c.CreateCapture(context.Background(), "ch_1", 500, "idem-c")
	if err != nil {
		t.Fatal(err)
	}
	if cr.ID != "ch_1" || cr.Status != "succeeded" {
		t.Fatalf("cr = %+v", cr)
	}
	if gotPath != "/v1/charges/ch_1/capture" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestCreateRefundHappy(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/refunds" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"re_1","status":"succeeded","charge":"ch_1"}`))
	})
	rr, err := c.CreateRefund(context.Background(), "ch_1", 250, "idem-r")
	if err != nil {
		t.Fatal(err)
	}
	if rr.ID != "re_1" || rr.Status != "succeeded" || rr.Charge != "ch_1" {
		t.Fatalf("rr = %+v", rr)
	}
}

func TestGetChargeHappy(t *testing.T) {
	t.Parallel()
	var gotPath string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ch_1","status":"succeeded"}`))
	})
	cr, err := c.GetCharge(context.Background(), "ch_1")
	if err != nil {
		t.Fatal(err)
	}
	if cr.Status != "succeeded" {
		t.Fatalf("status = %q", cr.Status)
	}
	if gotPath != "/v1/charges/ch_1" {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestDoTransportError(t *testing.T) {
	t.Parallel()
	c := New("http://127.0.0.1:0", "k")
	_, err := c.CreatePaymentIntent(context.Background(), 1, "usd", "k")
	if err == nil {
		t.Fatal("expected transport error")
	}
}

func TestCheckError2xx(t *testing.T) {
	t.Parallel()
	if err := checkError(&rawResp{status: 200, body: nil}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := checkError(&rawResp{status: 299, body: nil}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckErrorNoBody(t *testing.T) {
	t.Parallel()
	err := checkError(&rawResp{status: 500, body: []byte("not json")})
	ae, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if ae.Status != 500 {
		t.Fatalf("status = %d", ae.Status)
	}
}

func TestAuthorizeUsesChargeWhenPresent(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"pi_1","status":"requires_capture","latest_charge":"ch_99"}`))
	})
	railRef, status, err := c.Authorize(context.Background(), "ref", 100, "usd", "k")
	if err != nil {
		t.Fatal(err)
	}
	if railRef != "ch_99" {
		t.Fatalf("railRef = %q", railRef)
	}
	if status != "requires_capture" {
		t.Fatalf("status = %q", status)
	}
}

func TestAuthorizeFallsBackToIntentID(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"pi_only","status":"requires_capture"}`))
	})
	railRef, _, err := c.Authorize(context.Background(), "ref", 100, "usd", "k")
	if err != nil {
		t.Fatal(err)
	}
	if railRef != "pi_only" {
		t.Fatalf("railRef = %q", railRef)
	}
}

func TestProcessorCaptureRefundStatus(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/capture") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"ch_1","status":"succeeded"}`))
			return
		}
		if r.URL.Path == "/v1/refunds" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"re_1","status":"succeeded"}`))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/charges/") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"ch_1","status":"succeeded"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	st, err := c.Capture(context.Background(), "ch_1", 100, "usd", "k")
	if err != nil {
		t.Fatal(err)
	}
	if st != "succeeded" {
		t.Fatalf("capture = %q", st)
	}
	st, err = c.Refund(context.Background(), "ch_1", 100, "usd", "k")
	if err != nil {
		t.Fatal(err)
	}
	if st != "succeeded" {
		t.Fatalf("refund = %q", st)
	}
	st, err = c.Status(context.Background(), "ch_1")
	if err != nil {
		t.Fatal(err)
	}
	if st != "succeeded" {
		t.Fatalf("status = %q", st)
	}
	if c.DeclineCode(nil) != "" {
		t.Fatal("DeclineCode(nil) should be empty")
	}
}

func TestCaptureError(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"bad","message":"nope"}}`))
	})
	_, err := c.Capture(context.Background(), "ch_1", 100, "usd", "k")
	if err == nil {
		t.Fatal("expected error")
	}
	if DeclineCode(err) != "" {
		t.Fatalf("decline should be empty, got %q", DeclineCode(err))
	}
}

func TestAmountMinorUnitsRoundTrip(t *testing.T) {
	t.Parallel()
	cases := []float64{0, 0.01, 1.0, 12.34, 99.99, 1234.56}
	for _, amt := range cases {
		minor := AmountToMinorUnits(amt)
		back := MinorUnitsToAmount(minor)
		// allow for sub-cent float slop
		if back-amt > 0.01 || amt-back > 0.01 {
			t.Fatalf("round trip %v -> %d -> %v", amt, minor, back)
		}
	}
	if AmountToMinorUnits(12.345) != 1235 {
		t.Fatalf("expected rounding to nearest cent, got %d", AmountToMinorUnits(12.345))
	}
	if MinorUnitsToAmount(12345) != 123.45 {
		t.Fatalf("minor->amount = %v", MinorUnitsToAmount(12345))
	}
}

func TestNewClientDefaults(t *testing.T) {
	t.Parallel()
	c := New("https://api.stripe.com", "sk")
	if c.BaseURL != "https://api.stripe.com" || c.APIKey != "sk" {
		t.Fatalf("client = %+v", c)
	}
	if c.HTTP == nil {
		t.Fatal("HTTP client nil")
	}
}

func TestInvalidJSONResponse(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not json`))
	})
	if _, err := c.CreatePaymentIntent(context.Background(), 1, "usd", "k"); err == nil {
		t.Fatal("expected json error")
	}
}

func TestNilBodyGetRequest(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			if rb, _ := io.ReadAll(r.Body); len(rb) != 0 {
				t.Fatalf("get should have empty body, got %q", rb)
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ch_1","status":"succeeded"}`))
	})
	if _, err := c.GetCharge(context.Background(), "ch_1"); err != nil {
		t.Fatal(err)
	}
}

// ensure json import is used in case compiler strips it
var _ = json.RawMessage(nil)