-- Properties a calendar client keeps on a calendar for its own use (sidebar order, time zone,
-- a colour it picked). The server stores and returns them without interpreting them.
CREATE TABLE dav_props (
  user_id  TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  calendar TEXT NOT NULL,
  space    TEXT NOT NULL,
  local    TEXT NOT NULL,
  value    TEXT NOT NULL,
  PRIMARY KEY (user_id, calendar, space, local)
);
