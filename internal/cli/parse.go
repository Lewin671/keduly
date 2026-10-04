package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

var weekdayNames = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday, "mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tues": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday, "sat": time.Saturday, "saturday": time.Saturday,
}

// parseDate understands today, tomorrow, yesterday, weekday names (the next
// such day, today included) and YYYY-MM-DD.
func parseDate(s string, now time.Time, loc *time.Location) (string, error) {
	today := now.In(loc)
	word := strings.ToLower(strings.TrimSpace(s))
	switch word {
	case "today":
		return today.Format(dateLayout), nil
	case "tomorrow":
		return today.AddDate(0, 0, 1).Format(dateLayout), nil
	case "yesterday":
		return today.AddDate(0, 0, -1).Format(dateLayout), nil
	}
	if wd, ok := weekdayNames[word]; ok {
		ahead := (int(wd) - int(today.Weekday()) + 7) % 7
		return today.AddDate(0, 0, ahead).Format(dateLayout), nil
	}
	if t, err := time.Parse(dateLayout, word); err == nil {
		return t.Format(dateLayout), nil
	}
	return "", fmt.Errorf("cannot read %q as a date; use today, tomorrow, a weekday or YYYY-MM-DD", s)
}

var clockRE = regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)

func parseClock(s string) (hour, minute int, ok bool) {
	m := clockRE.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, false
	}
	hour, _ = strconv.Atoi(m[1])
	minute, _ = strconv.Atoi(m[2])
	return hour, minute, hour < 24 && minute < 60
}

// parseTime understands HH:MM (today), "<date> HH:MM", "<date>THH:MM" and RFC 3339.
func parseTime(s string, now time.Time, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	date, clock := "today", s
	if i := strings.LastIndexAny(s, " T"); i > 0 {
		date, clock = s[:i], s[i+1:]
	}
	hour, minute, ok := parseClock(clock)
	if !ok {
		return time.Time{}, fmt.Errorf(`cannot read %q as a time; use HH:MM, "YYYY-MM-DD HH:MM" or "tomorrow 14:00"`, s)
	}
	day, err := parseDate(date, now, loc)
	if err != nil {
		return time.Time{}, err
	}
	d, _ := time.ParseInLocation(dateLayout, day, loc)
	return time.Date(d.Year(), d.Month(), d.Day(), hour, minute, 0, 0, loc), nil
}

// parseMinutes understands 90m, 1h30m, 2h and a bare number of minutes.
func parseMinutes(s string) (int, error) {
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < time.Minute {
		return 0, fmt.Errorf("cannot read %q as a duration; use 90m, 1h30m or 2h", s)
	}
	return int(d.Minutes()), nil
}

// slot resolves --start with either --end or --duration into two instants.
func (a *app) slot(start, end, duration string) (time.Time, time.Time, error) {
	loc, err := a.zone()
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if start == "" {
		return time.Time{}, time.Time{}, usagef("--start is required")
	}
	if (end == "") == (duration == "") {
		return time.Time{}, time.Time{}, usagef("give exactly one of --end and --duration")
	}
	from, err := parseTime(start, a.env.Now(), loc)
	if err != nil {
		return from, from, err
	}
	if duration != "" {
		minutes, err := parseMinutes(duration)
		return from, from.Add(time.Duration(minutes) * time.Minute), err
	}
	to, err := parseTime(end, a.env.Now(), loc)
	return from, to, err
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }
