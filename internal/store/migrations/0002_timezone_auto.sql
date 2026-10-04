-- Whether the account's time zone follows the device the web app runs on (the default)
-- or was chosen by hand in Settings.
ALTER TABLE users ADD COLUMN timezone_auto INTEGER NOT NULL DEFAULT 1;
