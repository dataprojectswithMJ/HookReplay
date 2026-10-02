# Flutterwave — charge.completed

Signing scheme: `verif-hash = hex(SHA256(secret))` — a **static** hash of the
secret, emitted in the `verif-hash` header.

**Important:** this hash does **not** bind the payload — it proves nothing about
the request body. The real production control is **IP allowlisting** on your
endpoint. Failure-sim reasons that imply payload tamper-detection
(`invalid_signature`, `stale_signature`) are not meaningful for Flutterwave.
