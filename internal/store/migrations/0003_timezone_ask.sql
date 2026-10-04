-- Following the device silently turned out to be the wrong default: an account opened on a
-- device set to another zone changed without the user knowing. The zone now stays as it is and
-- the web app asks before changing it; following the device is an explicit choice in Settings.
UPDATE users SET timezone_auto = 0;
