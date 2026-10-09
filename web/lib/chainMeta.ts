// Accepted values for the Chain builder + docs. Single source of truth so the
// UI tooltips, the templates page, and the docs never drift apart.

export const DELIVER_REASONS = [
  "invalid_signature",
  "missing_signature",
  "stale_signature",
  "malformed_payload",
  "connection_refused",
] as const;

export const SIGN_ALGORITHMS = [
  "hmac-sha1",
  "hmac-sha256",
  "hmac-sha512",
  "ed25519",
  "none",
] as const;

export const SIGN_ENCODINGS = ["hex", "base64"] as const;

export const ENVIRONMENTS = ["local", "staging", "production"] as const;

export const DELIVER_RESULTS = ["success", "fail"] as const;

export const API_METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE"] as const;

