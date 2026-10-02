# GitHub — push

Signing scheme: `sha256=` + hex(HMAC-SHA256(body, secret)), emitted in the
`X-Hub-Signature-256` header.

Secret is the webhook secret set on the repository/org webhook configuration.
