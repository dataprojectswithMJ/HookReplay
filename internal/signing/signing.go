// Package signing implements provider webhook signature schemes (§7.1).
//
// The signer always operates over the raw request body bytes exactly as they
// will be transmitted. It never parses or re-serializes the payload: the body
// slice is opaque and passed straight into the HMAC/EdDSA primitive. Signing
// happens at dispatch time (fresh timestamps) — never at parse/render time.
package signing

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Scheme identifies a provider signing scheme.
type Scheme string

const (
	SchemeStripe      Scheme = "stripe"
	SchemeGitHub      Scheme = "github"
	SchemePaystack    Scheme = "paystack"
	SchemeCustom      Scheme = "custom"
	SchemeSlack       Scheme = "slack"
	SchemeMeta        Scheme = "meta"
	SchemeRazorpay    Scheme = "razorpay"
	SchemeFlutterwave Scheme = "flutterwave"
	SchemeResend      Scheme = "resend"
	SchemeKlarna      Scheme = "klarna"
	SchemeAdyen       Scheme = "adyen"
)

// Material is the key material used by a scheme. For custom schemes the
// Custom field carries the chain YAML `sign:` block.
type Material struct {
	Key    []byte
	Custom *CustomConfig
}

// CustomConfig is the configuration for the "custom" scheme (§6 sign: block).
// Format is a template for the header value containing exactly one {{hmac}}
// placeholder (where the signature is emitted) and optionally {{timestamp}}.
// The HMAC input is the same template with {{timestamp}} replaced by the
// unix-timestamp string and {{hmac}} replaced by the raw body bytes.
type CustomConfig struct {
	Algorithm       string // hmac-sha1 | hmac-sha256 | hmac-sha512 | none
	Header          string // e.g. "X-Signature"
	Format          string // e.g. "{{timestamp}}.{{hmac}}" or "{{hmac}}" (raw)
	Encoding        string // hex | base64 (default hex)
	TimestampHeader string // optional; set when Format uses {{timestamp}}
}

// Sign computes the signature header(s) for the given scheme over body, using
// key material m and the current time ts. Returns the HTTP headers to attach
// to the outgoing request.
func Sign(scheme Scheme, body []byte, m Material, ts time.Time) (http.Header, error) {
	switch scheme {
	case SchemeStripe:
		return signStripe(body, m.Key, ts)
	case SchemeGitHub:
		return signGitHub(body, m.Key)
	case SchemePaystack:
		return signPaystack(body, m.Key)
	case SchemeSlack:
		return signSlack(body, m.Key, ts)
	case SchemeMeta:
		return signMeta(body, m.Key)
	case SchemeRazorpay:
		return signRazorpay(body, m.Key)
	case SchemeFlutterwave:
		return signFlutterwave(body, m.Key)
	case SchemeResend:
		return signResend(body, m.Key, ts)
	case SchemeKlarna:
		return signKlarna(body, m.Key)
	case SchemeAdyen:
		return signAdyen(body, m.Key)
	case SchemeCustom:
		if m.Custom == nil {
			return nil, fmt.Errorf("signing: custom scheme requires Custom config")
		}
		return signCustom(body, m.Key, *m.Custom, ts)
	default:
		return nil, fmt.Errorf("signing: unsupported scheme %q", scheme)
	}
}

// NormalizeScheme maps a template/provider scheme string to a Scheme.
func NormalizeScheme(s string) (Scheme, error) {
	switch Scheme(strings.ToLower(strings.TrimSpace(s))) {
	case SchemeStripe:
		return SchemeStripe, nil
	case SchemeGitHub:
		return SchemeGitHub, nil
	case SchemePaystack:
		return SchemePaystack, nil
	case SchemeSlack:
		return SchemeSlack, nil
	case SchemeMeta:
		return SchemeMeta, nil
	case SchemeRazorpay:
		return SchemeRazorpay, nil
	case SchemeFlutterwave:
		return SchemeFlutterwave, nil
	case SchemeResend:
		return SchemeResend, nil
	case SchemeKlarna:
		return SchemeKlarna, nil
	case SchemeAdyen:
		return SchemeAdyen, nil
	case SchemeCustom:
		return SchemeCustom, nil
	default:
		return "", fmt.Errorf("signing: unknown scheme %q", s)
	}
}

// hmacDigest computes HMAC over data with the given hash constructor and key,
// returning the encoded output.
func hmacDigest(newHash func() hash.Hash, key, data []byte, encoding string) (string, error) {
	mac := hmac.New(newHash, key)
	mac.Write(data)
	switch encoding {
	case "hex":
		return hex.EncodeToString(mac.Sum(nil)), nil
	case "base64":
		return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
	default:
		return "", fmt.Errorf("signing: unsupported encoding %q", encoding)
	}
}

func signStripe(body, key []byte, ts time.Time) (http.Header, error) {
	t := strconv.FormatInt(ts.Unix(), 10)
	signed := t + "." + string(body)
	v1, err := hmacDigest(sha256.New, key, []byte(signed), "hex")
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("Stripe-Signature", "t="+t+",v1="+v1)
	return h, nil
}

func signGitHub(body, key []byte) (http.Header, error) {
	sig, err := hmacDigest(sha256.New, key, body, "hex")
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("X-Hub-Signature-256", "sha256="+sig)
	return h, nil
}

func signPaystack(body, key []byte) (http.Header, error) {
	// Paystack uses hex(HMAC-SHA512) — hex, not base64. This is a CI-mandated
	// encoding vector.
	sig, err := hmacDigest(sha512.New, key, body, "hex")
	if err != nil {
		return nil, err
	}
	h := http.Header{}
	h.Set("x-paystack-signature", sig)
	return h, nil
}

func signCustom(body, key []byte, cfg CustomConfig, ts time.Time) (http.Header, error) {
	alg := cfg.Algorithm
	if alg == "" {
		alg = "hmac-sha256"
	}

	var newHash func() hash.Hash
	switch alg {
	case "hmac-sha1":
		newHash = sha1.New
	case "hmac-sha256":
		newHash = sha256.New
	case "hmac-sha512":
		newHash = sha512.New
	case "none":
		newHash = nil
	default:
		return nil, fmt.Errorf("signing: unsupported custom algorithm %q", alg)
	}

	header := cfg.Header
	if header == "" {
		header = "X-Signature"
	}
	format := cfg.Format
	if format == "" || format == "raw" {
		format = "{{hmac}}"
	}
	encoding := cfg.Encoding
	if encoding == "" {
		encoding = "hex"
	}
	if encoding != "hex" && encoding != "base64" {
		return nil, fmt.Errorf("signing: unsupported custom encoding %q", encoding)
	}

	tsStr := strconv.FormatInt(ts.Unix(), 10)

	sig := ""
	if newHash != nil {
		input := strings.ReplaceAll(format, "{{timestamp}}", tsStr)
		input = strings.ReplaceAll(input, "{{hmac}}", string(body))
		var err error
		sig, err = hmacDigest(newHash, key, []byte(input), encoding)
		if err != nil {
			return nil, err
		}
	}

	value := strings.ReplaceAll(format, "{{timestamp}}", tsStr)
	value = strings.ReplaceAll(value, "{{hmac}}", sig)

	h := http.Header{}
	h.Set(header, value)
	if cfg.TimestampHeader != "" {
		h.Set(cfg.TimestampHeader, tsStr)
	}
	return h, nil
}
