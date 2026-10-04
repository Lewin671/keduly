package cli

import (
	"testing"
	"time"
)

func TestParseDatesTimesDurations(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	// 23:30 UTC on Monday is 07:30 on Tuesday 13 October in Shanghai.
	now := time.Date(2026, 10, 12, 23, 30, 0, 0, time.UTC)
	dates := map[string]string{
		"today": "2026-10-13", "tomorrow": "2026-10-14", "yesterday": "2026-10-12",
		"tue": "2026-10-13", "Friday": "2026-10-16", "mon": "2026-10-19", "2027-01-02": "2027-01-02",
	}
	for in, want := range dates {
		if got, err := parseDate(in, now, loc); err != nil || got != want {
			t.Errorf("parseDate(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "someday", "2026-02-30", "13/10/2026"} {
		if _, err := parseDate(bad, now, loc); err == nil {
			t.Errorf("parseDate(%q) should fail", bad)
		}
	}
	times := map[string]string{
		"14:00":                "2026-10-13T06:00:00Z",
		"9:05":                 "2026-10-13T01:05:00Z",
		"2026-10-20 09:00":     "2026-10-20T01:00:00Z",
		"2026-10-20T09:00":     "2026-10-20T01:00:00Z",
		"tomorrow 14:00":       "2026-10-14T06:00:00Z",
		"Thursday 08:00":       "2026-10-15T00:00:00Z",
		"2026-10-20T09:00:00Z": "2026-10-20T09:00:00Z",
	}
	for in, want := range times {
		got, err := parseTime(in, now, loc)
		if err != nil || stamp(got) != want {
			t.Errorf("parseTime(%q) = %s, %v; want %s", in, stamp(got), err, want)
		}
	}
	for _, bad := range []string{"", "25:00", "tomorrow", "noon"} {
		if _, err := parseTime(bad, now, loc); err == nil {
			t.Errorf("parseTime(%q) should fail", bad)
		}
	}
	minutes := map[string]int{"90m": 90, "1h30m": 90, "2h": 120, "45": 45}
	for in, want := range minutes {
		if got, err := parseMinutes(in); err != nil || got != want {
			t.Errorf("parseMinutes(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "0", "soon", "30s"} {
		if _, err := parseMinutes(bad); err == nil {
			t.Errorf("parseMinutes(%q) should fail", bad)
		}
	}
}
