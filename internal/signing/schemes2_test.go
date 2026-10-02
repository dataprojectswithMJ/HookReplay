package signing

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestNewSchemesRoundTrip(t *testing.T) {
	now := time.Now()
	_, edPriv, _ := ed25519.GenerateKey(nil)

	cases := []struct {
		name   string
		scheme Scheme
		mat    Material
	}{
		{"slack", SchemeSlack, Material{Key: testKey}},
		{"meta", SchemeMeta, Material{Key: testKey}},
		{"razorpay", SchemeRazorpay, Material{Key: testKey}},
		{"flutterwave", SchemeFlutterwave, Material{Key: testKey}},
		{"klarna", SchemeKlarna, Material{Key: testKey}},
		{"adyen", SchemeAdyen, Material{Key: testKey}},
		{"resend", SchemeResend, Material{Key: edPriv}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hdr, err := Sign(c.scheme, testBody, c.mat, now)
			if err != nil {
				t.Fatal(err)
			}
			ok, err := Verify(c.scheme, testBody, c.mat, hdr, now)
			if err != nil {
				t.Fatal(err)
			}
			if !ok {
				t.Fatalf("signature did not verify for %s: %v", c.name, hdr)
			}
		})
	}
}

func TestSlackFormat(t *testing.T) {
	hdr, err := Sign(SchemeSlack, testBody, Material{Key: testKey}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hdr.Get("X-Slack-Signature"), "v0=") {
		t.Fatalf("slack signature missing v0= prefix: %q", hdr.Get("X-Slack-Signature"))
	}
	if hdr.Get("X-Slack-Request-Timestamp") == "" {
		t.Fatal("missing X-Slack-Request-Timestamp")
	}
}

func TestFlutterwaveStaticHash(t *testing.T) {
	hdr1, _ := Sign(SchemeFlutterwave, []byte("body-a"), Material{Key: testKey}, time.Now())
	hdr2, _ := Sign(SchemeFlutterwave, []byte("body-b-different"), Material{Key: testKey}, time.Now())
	if hdr1.Get("verif-hash") != hdr2.Get("verif-hash") {
		t.Fatal("flutterwave hash should be static (not bound to payload)")
	}
}

func TestResendEd25519(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	hdr, err := Sign(SchemeResend, testBody, Material{Key: priv}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	sig, err := base64.StdEncoding.DecodeString(hdr.Get("Resend-Signature"))
	if err != nil {
		t.Fatal(err)
	}
	pub := ed25519.PrivateKey(priv).Public().(ed25519.PublicKey)
	if !ed25519.Verify(pub, testBody, sig) {
		t.Fatal("resend ed25519 signature did not verify")
	}
	if hdr.Get("Resend-Signature-Timestamp") == "" {
		t.Fatal("missing Resend-Signature-Timestamp")
	}
}

func TestKlarnaBase64Encoding(t *testing.T) {
	hdr, err := Sign(SchemeKlarna, testBody, Material{Key: testKey}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// base64 of 32-byte HMAC-SHA256 → 44 chars ending in '='
	sig := hdr.Get("X-Klarna-Signature")
	if len(sig) != 44 || sig[len(sig)-1] != '=' {
		t.Fatalf("klarna signature should be base64 (44 chars, padding '='), got %q", sig)
	}
}
