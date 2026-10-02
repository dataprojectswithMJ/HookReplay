# Stripe — checkout.session.completed

Signing scheme: HMAC-SHA256 over `"{t}.{body}"` with the webhook signing secret
(`whsec_...`), emitted in the `Stripe-Signature` header as `t={unix},v1={hex}`.

Stripe rejects events older than ~180s, so HookReplay signs with a **fresh**
timestamp at dispatch time — never at render time.

Override example: `data.object.amount_total: 25000`.
