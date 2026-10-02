package signing

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// stripeTolerance is the Stripe signature freshness window (§7.1).
const stripeTolerance = 180 * time.Second

// Verify independently checks that header matches body under the given scheme.
// This is a deliberately separate implementation from Sign: it is used as the
// reference verifier in tests and (later) the sandbox gate, so a bug in Sign
// cannot also hide in Verify.
func Verify(scheme Scheme, body []byte, m Material, header http.Header, now time.Time) (bool, error) {
	switch scheme {
	case SchemeStripe:
		return verifyStripe(body, m.Key, header.Get("Stripe-Signature"), now)
	case SchemeGitHub:
		return verifyHMAC(body, m.Key, header.Get("X-Hub-Signature-256"), "sha256=", sha256.New)
	case SchemePaystack:
		return verifyHMAC(body, m.Key, header.Get("x-paystack-signature"), "", sha512.New)
	case SchemeSlack:
		return verifySlack(body, m.Key, header.Get("X-Slack-Signature"), header.Get("X-Slack-Request-Timestamp"), now)
	case SchemeMeta:
		return verifyHMAC(body, m.Key, header.Get("X-Hub-Signature-256"), "sha256=", sha256.New)
	case SchemeRazorpay:
		return verifyHMAC(body, m.Key, header.Get("x-razorpay-signature"), "", sha256.New)
	case SchemeFlutterwave:
		return verifyFlutterwave(m.Key, header.Get("verif-hash")), nil
	case SchemeResend:
		return verifyResend(body, m.Key, header.Get("Resend-Signature"))
	case SchemeKlarna:
		return verifyBase64HMAC(body, m.Key, header.Get("X-Klarna-Signature"), sha256.New)
	case SchemeAdyen:
		return verifyBase64HMAC(body, m.Key, header.Get("x-adyen-signature"), sha256.New)
	default:
		return false, fmt.Errorf("verify: unsupported scheme %q", scheme)
	}
}

func verifyStripe(body, key []byte, sig string, now time.Time) (bool, error) {
	var t, v1 string
	for _, p := range strings.Split(sig, ",") {
		switch {
		case strings.HasPrefix(p, "t="):
			t = strings.TrimPrefix(p, "t=")
		case strings.HasPrefix(p, "v1="):
			v1 = strings.TrimPrefix(p, "v1=")
		}
	}
	if t == "" || v1 == "" {
		return false, fmt.Errorf("verify: malformed Stripe-Signature")
	}
	ts, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return false, err
	}
	if diff := now.Sub(time.Unix(ts, 0)); diff < -stripeTolerance || diff > stripeTolerance {
		return false, fmt.Errorf("verify: stale signature (ts %d, now %d)", ts, now.Unix())
	}
	return hmacEqualHex(sha256.New, key, []byte(t+"."+string(body)), v1), nil
}

func verifySlack(body, key []byte, sig, tsStr string, now time.Time) (bool, error) {
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return false, err
	}
	if diff := now.Sub(time.Unix(ts, 0)); diff < -5*time.Minute || diff > 5*time.Minute {
		return false, fmt.Errorf("verify: stale slack signature")
	}
	sig = strings.TrimPrefix(sig, "v0=")
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("v0:" + tsStr + ":" + string(body)))
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(sig)) == 1, nil
}

func verifyHMAC(body, key []byte, got, prefix string, newHash func() hash.Hash) (bool, error) {
	got = strings.TrimPrefix(got, prefix)
	return hmacEqualHex(newHash, key, body, got), nil
}

func verifyBase64HMAC(body, key []byte, got string, newHash func() hash.Hash) (bool, error) {
	mac := hmac.New(newHash, key)
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(got)) == 1, nil
}

func verifyFlutterwave(key []byte, got string) bool {
	sum := sha256.Sum256(key)
	expected := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(got)) == 1
}

func verifyResend(body, key []byte, got string) (bool, error) {
	if len(key) != ed25519.PrivateKeySize {
		return false, fmt.Errorf("verify: resend key size %d", len(key))
	}
	pub := ed25519.PrivateKey(key).Public().(ed25519.PublicKey)
	sig, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		return false, err
	}
	return ed25519.Verify(pub, body, sig), nil
}

func hmacEqualHex(newHash func() hash.Hash, key, data []byte, wantHex string) bool {
	mac := hmac.New(newHash, key)
	mac.Write(data)
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(wantHex)) == 1
}


