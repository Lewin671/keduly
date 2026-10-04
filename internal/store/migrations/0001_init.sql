CREATE TABLE users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE COLLATE NOCASE,
  name TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  timezone TEXT NOT NULL,
  work_start TEXT NOT NULL,
  work_end TEXT NOT NULL,
  revision INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);

CREATE TABLE sessions (
  hash TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE tokens (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  scope TEXT NOT NULL,
  confirm_delete INTEGER NOT NULL,
  hash TEXT NOT NULL UNIQUE,
  last_used_at TEXT,
  created_at TEXT NOT NULL
);
CREATE INDEX tokens_user ON tokens(user_id);

CREATE TABLE areas (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  position INTEGER NOT NULL
);
CREATE INDEX areas_user ON areas(user_id);

CREATE TABLE projects (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  area_id TEXT REFERENCES areas(id) ON DELETE SET NULL,
  name TEXT NOT NULL,
  color TEXT NOT NULL,
  notes TEXT NOT NULL,
  position INTEGER NOT NULL,
  archived INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX projects_user ON projects(user_id);

CREATE TABLE headings (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  position INTEGER NOT NULL
);
CREATE INDEX headings_user ON headings(user_id);

CREATE TABLE items (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
  heading_id TEXT REFERENCES headings(id) ON DELETE SET NULL,
  title TEXT NOT NULL,
  notes TEXT NOT NULL,
  estimate_minutes INTEGER,
  planned_date TEXT,
  evening INTEGER NOT NULL,
  due_date TEXT,
  due_time TEXT,
  important INTEGER NOT NULL,
  status TEXT NOT NULL,
  completed_at TEXT,
  position INTEGER NOT NULL,
  created_by_kind TEXT NOT NULL,
  created_by_name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX items_user_status ON items(user_id, status);
CREATE INDEX items_project ON items(project_id);

CREATE TABLE events (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
  item_id TEXT REFERENCES items(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  notes TEXT NOT NULL,
  location TEXT NOT NULL,
  all_day INTEGER NOT NULL,
  start_at TEXT,
  end_at TEXT,
  start_date TEXT,
  end_date TEXT,
  rrule TEXT,
  uid TEXT NOT NULL,
  dav_name TEXT NOT NULL,
  ical TEXT NOT NULL,
  etag TEXT NOT NULL,
  created_by_kind TEXT NOT NULL,
  created_by_name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE INDEX events_user_start ON events(user_id, start_at);
CREATE INDEX events_item ON events(item_id);
CREATE INDEX events_dav ON events(user_id, dav_name);

CREATE TABLE suggestions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  kind TEXT NOT NULL,
  actor_kind TEXT NOT NULL,
  actor_name TEXT NOT NULL,
  reason TEXT NOT NULL,
  title TEXT NOT NULL,
  item_id TEXT,
  event_id TEXT,
  start_at TEXT,
  end_at TEXT,
  payload TEXT,
  created_at TEXT NOT NULL,
  decided_at TEXT
);
CREATE INDEX suggestions_user_status ON suggestions(user_id, status);

CREATE TABLE activities (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  id TEXT NOT NULL UNIQUE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  actor_kind TEXT NOT NULL,
  actor_name TEXT NOT NULL,
  action TEXT NOT NULL,
  summary TEXT NOT NULL,
  reason TEXT,
  undoable INTEGER NOT NULL,
  undone INTEGER NOT NULL,
  changes TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX activities_user ON activities(user_id, seq);

-- One counter per calendar ("inbox" or a project ID), bumped on every event write.
CREATE TABLE ctags (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  calendar TEXT NOT NULL,
  ctag INTEGER NOT NULL,
  PRIMARY KEY (user_id, calendar)
);
