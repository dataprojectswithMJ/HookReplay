package signing

import (
	"crypto/sha256"
	"strings"
	"testing"
	"time"
)

var testKey = []byte("whsec_test_secret_key")
var testBody = []byte(`{"id":"evt_1","type":"checkout.session.completed"}`)

func TestHMACPrimitiveRFC4231(t *testing.T) {
	// RFC 4231 Test Case 1 (SHA-256) validates the underlying HMAC usage.
	key := make([]byte, 20)
	for i := range key {
		key[i] = 0x0b
	}
	got, err := hmacDigest(sha256.New, key, []byte("Hi There"), "hex")
	if err != nil {
		t.Fatal(err)
	}
	want := "b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7"
	if got != want {
		t.Fatalf("HMAC-SHA256 vector mismatch:\n got  %s\n want %s", got, want)
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name   string
		scheme Scheme
		mat    Material
	}{
		{"stripe", SchemeStripe, Material{Key: testKey}},
		{"github", SchemeGitHub, Material{Key: testKey}},
		{"paystack", SchemePaystack, Material{Key: testKey}},
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

func TestSignWrongKeyFails(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name   string
		scheme Scheme
	}{
		{"stripe", SchemeStripe},
		{"github", SchemeGitHub},
		{"paystack", SchemePaystack},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hdr, err := Sign(c.scheme, testBody, Material{Key: testKey}, now)
			if err != nil {
				t.Fatal(err)
			}
			ok, err := Verify(c.scheme, testBody, Material{Key: []byte("wrong-key")}, hdr, now)
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				t.Fatalf("signature verified with the wrong key for %s", c.name)
			}
		})
	}
}

func TestStripeHeaderFormat(t *testing.T) {
	now := time.Now()
	hdr, err := Sign(SchemeStripe, testBody, Material{Key: testKey}, now)
	if err != nil {
		t.Fatal(err)
	}
	sig := hdr.Get("Stripe-Signature")
	if !strings.HasPrefix(sig, "t=") || !strings.Contains(sig, ",v1=") {
		t.Fatalf("unexpected Stripe-Signature format: %q", sig)
	}
}

func TestStripeStaleRejected(t *testing.T) {
	fresh := time.Now()
	hdr, err := Sign(SchemeStripe, testBody, Material{Key: testKey}, fresh)
	if err != nil {
		t.Fatal(err)
	}
	// Verify with a "now" far outside the 180s tolerance window.
	stale := fresh.Add(10 * time.Minute)
	if _, err := Verify(SchemeStripe, testBody, Material{Key: testKey}, hdr, stale); err == nil {
		t.Fatal("expected stale signature to be rejected")
	}
}

func TestPaystackHexEncoding(t *testing.T) {
	// Paystack is hex(HMAC-SHA512): 128 hex chars, never base64.
	hdr, err := Sign(SchemePaystack, testBody, Material{Key: testKey}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	sig := hdr.Get("x-paystack-signature")
	if len(sig) != 128 {
		t.Fatalf("expected 128 hex chars (HMAC-SHA512), got %d: %q", len(sig), sig)
	}
	for _, r := range sig {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("non-hex character in Paystack signature: %q", sig)
		}
	}
}

func TestGitHubPrefix(t *testing.T) {
	hdr, err := Sign(SchemeGitHub, testBody, Material{Key: testKey}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hdr.Get("X-Hub-Signature-256"), "sha256=") {
		t.Fatalf("missing sha256= prefix: %q", hdr.Get("X-Hub-Signature-256"))
	}
}

func TestCustomSigning(t *testing.T) {
	cfg := CustomConfig{
		Algorithm:       "hmac-sha256",
		Header:          "X-Signature",
		Format:          "{{timestamp}}.{{hmac}}",
		Encoding:        "hex",
		TimestampHeader: "X-Timestamp",
	}
	now := time.Now()
	hdr, err := Sign(SchemeCustom, testBody, Material{Key: testKey, Custom: &cfg}, now)
	if err != nil {
		t.Fatal(err)
	}
	sig := hdr.Get("X-Signature")
	parts := strings.Split(sig, ".")
	if len(parts) != 2 {
		t.Fatalf("expected timestamp.sig format, got %q", sig)
	}
	if hdr.Get("X-Timestamp") == "" {
		t.Fatal("expected X-Timestamp header to be set")
	}
	if parts[1] == "" {
		t.Fatal("expected non-empty hmac component")
	}
}

func TestCustomNoneAlgorithm(t *testing.T) {
	cfg := CustomConfig{Algorithm: "none", Header: "X-Signature", Format: "{{hmac}}"}
	hdr, err := Sign(SchemeCustom, testBody, Material{Key: testKey, Custom: &cfg}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if hdr.Get("X-Signature") != "" {
		t.Fatalf("expected empty signature for none algorithm, got %q", hdr.Get("X-Signature"))
	}
}

func TestUnsupportedScheme(t *testing.T) {
	if _, err := Sign("bogus", testBody, Material{Key: testKey}, time.Now()); err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}

func BenchmarkSign(b *testing.B) {
	body := testBody
	mat := Material{Key: testKey}
	now := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Sign(SchemeStripe, body, mat, now); err != nil {
			b.Fatal(err)
		}
	}
}
