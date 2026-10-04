-- Signing in on another device. A row lives for two minutes and is deleted when it is used.
--   kind 'request': a device that is not signed in asks to be. secret_hash is of the secret only
--     that device holds, pin is the number on its screen, and user_id is set once a signed-in
--     session approves; attempts counts wrong pins.
--   kind 'code': a signed-in session issued a one-time code. secret_hash is of that code and
--     user_id is the account it signs in to.
CREATE TABLE login_links (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  secret_hash TEXT NOT NULL UNIQUE,
  user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
  pin TEXT NOT NULL,
  device TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX login_links_user ON login_links(user_id);
