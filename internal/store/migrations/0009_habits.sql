CREATE TABLE habits (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 created_at TEXT NOT NULL
);
CREATE INDEX habits_user ON habits(user_id);
CREATE TABLE habit_rules (
 habit_id TEXT NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
 version INTEGER NOT NULL, effective_date TEXT NOT NULL, body TEXT NOT NULL,
 PRIMARY KEY(habit_id,version)
);
CREATE TABLE habit_records (
 habit_id TEXT NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
 date TEXT NOT NULL, body TEXT NOT NULL,
 PRIMARY KEY(habit_id,date)
);
