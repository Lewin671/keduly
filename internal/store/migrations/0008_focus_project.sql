-- Free focus can be filed under a project afterwards. A session on an item leaves this empty:
-- its project is the item's.
ALTER TABLE focus_sessions ADD COLUMN project_id TEXT;
