-- WS-8: email/password auth (password_hash on users).

ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash text;
