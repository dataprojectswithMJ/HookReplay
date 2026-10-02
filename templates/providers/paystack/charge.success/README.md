# Paystack — charge.success

Signing scheme: hex(HMAC-SHA512(body, secret)), emitted in the
`x-paystack-signature` header. **Encoding is hex, not base64** — this is a
deliberate test vector in CI.

Secret is the webhook secret from the Paystack dashboard (Business settings →
API Keys & Webhooks → Webhook Secret).
