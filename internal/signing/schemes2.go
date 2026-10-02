package signing

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// signSlack: v0={hex(HMAC-SHA256("v0:{ts}:{body}", secret))}.
func signSlack(body, key []byte, ts time.Time) (http.Header, error) {
	tsStr := strconv.FormatInt(ts.Unix(), 10)
	sig, err := hmacDigest(sha256.New, key, []byte("v0:"+tsStr+":"+string(body)), "hex")
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("X-Slack-Signature", "v0="+sig)
	h.Set("X-Slack-Request-Timestamp", tsStr)
	return h, nil
}

// signMeta: sha256=hex(HMAC-SHA256(body, secret)) — same wire format as GitHub.
func signMeta(body, key []byte) (http.Header, error) {
	sig, err := hmacDigest(sha256.New, key, body, "hex")
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("X-Hub-Signature-256", "sha256="+sig)
	return h, nil
}

// signRazorpay: hex(HMAC-SHA256(body, secret)).
func signRazorpay(body, key []byte) (http.Header, error) {
	sig, err := hmacDigest(sha256.New, key, body, "hex")
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("x-razorpay-signature", sig)
	return h, nil
}

// signFlutterwave: verif-hash = hex(SHA256(secret)) — static, does not bind the
// payload. Template docs must note IP allowlisting is the real control.
func signFlutterwave(body, key []byte) (http.Header, error) {
	sum := sha256.Sum256(key)
	h := http.Header{}
	h.Set("verif-hash", hex.EncodeToString(sum[:]))
	return h, nil
}

// signResend: Ed25519 over the raw body; headers Resend-Signature (base64) and
// Resend-Signature-Timestamp (unix seconds).
func signResend(body, key []byte, ts time.Time) (http.Header, error) {
	if len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("signing: resend key must be %d bytes (got %d)", ed25519.PrivateKeySize, len(key))
	}
	sig := ed25519.Sign(ed25519.PrivateKey(key), body)
	h := http.Header{}
	h.Set("Resend-Signature", base64.StdEncoding.EncodeToString(sig))
	h.Set("Resend-Signature-Timestamp", strconv.FormatInt(ts.Unix(), 10))
	return h, nil
}

// signKlarna: base64(HMAC-SHA256(body, secret)).
func signKlarna(body, key []byte) (http.Header, error) {
	sig, err := hmacDigest(sha256.New, key, body, "base64")
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("X-Klarna-Signature", sig)
	return h, nil
}

// signAdyen: base64(HMAC-SHA256(body, secret)).
func signAdyen(body, key []byte) (http.Header, error) {
	sig, err := hmacDigest(sha256.New, key, body, "base64")
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("x-adyen-signature", sig)
	return h, nil
}
