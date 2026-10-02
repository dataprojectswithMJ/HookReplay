package dispatch

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hookreplay/hookreplay/internal/signing"
)

func TestDispatchByteExact(t *testing.T) {
	var got []byte
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		gotSig = r.Header.Get("Stripe-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	body := []byte(`{"id":"evt_1","type":"checkout.session.completed"}`)
	secret := []byte("whsec_test_secret")

	res, err := New().Dispatch(context.Background(), Request{
		Scheme: signing.SchemeStripe,
		Body:   body,
		Secret: secret,
		Target: Target{Kind: "url", URL: srv.URL},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatal("sent body != original body (byte-exactness violated)")
	}
	if !bytes.Equal(res.RequestBody, body) {
		t.Fatal("Result.RequestBody != sent body")
	}
	if gotSig == "" {
		t.Fatal("expected Stripe-Signature header")
	}

	hdr := http.Header{}
	hdr.Set("Stripe-Signature", gotSig)
	ok, err := signing.Verify(signing.SchemeStripe, body, signing.Material{Key: secret}, hdr, time.Now())
	if err != nil || !ok {
		t.Fatalf("signature did not verify: ok=%v err=%v", ok, err)
	}
}

func TestDispatchMissingSignature(t *testing.T) {
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("Stripe-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := New().Dispatch(context.Background(), Request{
		Scheme:  signing.SchemeStripe,
		Body:    []byte(`{"id":"x"}`),
		Secret:  []byte("whsec_test_secret"),
		Target:  Target{Kind: "url", URL: srv.URL},
		Deliver: Deliver{Result: "fail", Reason: "missing_signature"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotSig != "" {
		t.Fatalf("expected no signature header, got %q", gotSig)
	}
}

func TestDispatchInvalidSignature(t *testing.T) {
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("Stripe-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	body := []byte(`{"id":"x"}`)
	secret := []byte("whsec_test_secret")
	_, err := New().Dispatch(context.Background(), Request{
		Scheme:  signing.SchemeStripe,
		Body:    body,
		Secret:  secret,
		Target:  Target{Kind: "url", URL: srv.URL},
		Deliver: Deliver{Result: "fail", Reason: "invalid_signature"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotSig == "" {
		t.Fatal("expected a signature header")
	}
	hdr := http.Header{}
	hdr.Set("Stripe-Signature", gotSig)
	ok, err := signing.Verify(signing.SchemeStripe, body, signing.Material{Key: secret}, hdr, time.Now())
	if err == nil && ok {
		t.Fatal("forged signature unexpectedly verified")
	}
}

func TestDispatchConnectionRefused(t *testing.T) {
	// Dial a port that nothing is listening on.
	res, err := New().Dispatch(context.Background(), Request{
		Scheme:  signing.SchemeStripe,
		Body:    []byte(`{"id":"x"}`),
		Secret:  []byte("whsec_test_secret"),
		Target:  Target{Kind: "url", URL: "http://127.0.0.1:1/"},
		Deliver: Deliver{Result: "fail", Reason: "connection_refused"},
	})
	if err == nil {
		t.Fatal("expected connection_refused error")
	}
	if res.ErrorCode != "connection_refused" {
		t.Fatalf("expected error_code connection_refused, got %q", res.ErrorCode)
	}
}
