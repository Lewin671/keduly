-- The focus timer. A session is one stretch of work (a tomato) or of rest, with the end it was
-- planned to have: nothing needs to run for a tomato to complete. Giving a session up moves its
-- end to that moment. item_id is not a foreign key: sessions outlive the item they counted towards.
CREATE TABLE focus_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  item_id TEXT,
  title TEXT NOT NULL,
  start_at TEXT NOT NULL,
  end_at TEXT NOT NULL,
  planned_minutes INTEGER NOT NULL,
  -- Whether the user has said what comes after this session. A tomato that ran out and is not
  -- answered yet is what the timer shows as "over".
  answered INTEGER NOT NULL,
  created_by_kind TEXT NOT NULL,
  created_by_name TEXT NOT NULL
);
CREATE INDEX focus_sessions_user_start ON focus_sessions(user_id, start_at);
CREATE INDEX focus_sessions_item ON focus_sessions(item_id);

ALTER TABLE users ADD COLUMN focus_minutes INTEGER NOT NULL DEFAULT 25;
ALTER TABLE users ADD COLUMN rest_minutes INTEGER NOT NULL DEFAULT 5;
ALTER TABLE users ADD COLUMN long_rest_minutes INTEGER NOT NULL DEFAULT 15;
ALTER TABLE users ADD COLUMN round_size INTEGER NOT NULL DEFAULT 4;
