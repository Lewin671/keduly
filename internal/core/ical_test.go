package core

import (
	"strings"
	"testing"
	"time"

	"github.com/Lewin671/keduly/internal/store"
)

func series(lines ...string) *store.Event {
	body := append([]string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//x//EN", "BEGIN:VEVENT", "UID:u", "DTSTAMP:20260101T000000Z"}, lines...)
	text := strings.Join(append(body, "END:VEVENT", "END:VCALENDAR", ""), "\r\n")
	cal, err := DecodeICal(text)
	if err != nil {
		panic(err)
	}
	ev := &store.Event{ICal: text}
	if err := applyICal(ev, cal, time.UTC); err != nil {
		panic(err)
	}
	return ev
}

func starts(occ []occurrence) []string {
	out := []string{}
	for _, o := range occ {
		out = append(out, store.FormatTime(o.Start))
	}
	return out
}

func TestExpandKeepsWallClockAcrossDST(t *testing.T) {
	// 09:00 in New York every day; clocks go back on 1 November 2026.
	ev := series("DTSTART;TZID=America/New_York:20261031T090000", "DTEND;TZID=America/New_York:20261031T100000", "RRULE:FREQ=DAILY;COUNT=3")
	got := starts(expand(ev, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), time.UTC))
	want := []string{"2026-10-31T13:00:00Z", "2026-11-01T14:00:00Z", "2026-11-02T14:00:00Z"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExpandHonoursExdateListsUntilAndWindow(t *testing.T) {
	ev := series("DTSTART:20261001T100000Z", "DURATION:PT2H", "RRULE:FREQ=DAILY;UNTIL=20261006T100000Z",
		"EXDATE:20261002T100000Z,20261003T100000Z")
	all := starts(expand(ev, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.UTC))
	if strings.Join(all, ",") != "2026-10-01T10:00:00Z,2026-10-04T10:00:00Z,2026-10-05T10:00:00Z,2026-10-06T10:00:00Z" {
		t.Fatalf("all %v", all)
	}
	// An occurrence that started before the window but is still running counts;
	// one that starts exactly at the end does not.
	window := starts(expand(ev, time.Date(2026, 10, 4, 11, 0, 0, 0, time.UTC), time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC), time.UTC))
	if strings.Join(window, ",") != "2026-10-04T10:00:00Z" {
		t.Fatalf("window %v", window)
	}
}

func TestExpandIsBoundedForRunawayRules(t *testing.T) {
	ev := series("DTSTART:20000101T000000Z", "DTEND:20000101T000100Z", "RRULE:FREQ=SECONDLY")
	begin := time.Now()
	got := expand(ev, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), time.UTC)
	if len(got) != 0 || time.Since(begin) > 2*time.Second {
		t.Fatalf("%d occurrences in %s", len(got), time.Since(begin))
	}
	daily := series("DTSTART:19900101T000000Z", "DTEND:19900101T010000Z", "RRULE:FREQ=DAILY")
	got = expand(daily, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC), time.UTC)
	if len(got) != 7 {
		t.Fatalf("a daily rule from 1990 gave %d occurrences in a week of 2026", len(got))
	}
}

func TestApplyICalFallsBackOnUnknownZone(t *testing.T) {
	shanghai, _ := time.LoadLocation("Asia/Shanghai")
	text := strings.Join([]string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//x//EN", "BEGIN:VEVENT", "UID:u", "DTSTAMP:20260101T000000Z",
		"DTSTART;TZID=Customized Time Zone:20261013T100000", "SUMMARY:x", "END:VEVENT", "END:VCALENDAR", ""}, "\r\n")
	cal, err := DecodeICal(text)
	if err != nil {
		t.Fatal(err)
	}
	ev := &store.Event{}
	if err := applyICal(ev, cal, shanghai); err != nil {
		t.Fatal(err)
	}
	// No DTEND and no DURATION: the event ends when it starts.
	if *ev.StartAt != "2026-10-13T02:00:00Z" || *ev.EndAt != *ev.StartAt {
		t.Fatalf("start %s end %s", *ev.StartAt, *ev.EndAt)
	}
}
