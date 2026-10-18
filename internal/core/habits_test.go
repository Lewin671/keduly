package core

import (
	"testing"
	"time"
)

func TestHabitDST(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	if got := wallInstant("2026-03-08", "02:30", loc).Format(time.RFC3339); got != "2026-03-08T07:00:00Z" {
		t.Fatal(got)
	}
	if got := wallInstant("2026-11-01", "01:30", loc).Format(time.RFC3339); got != "2026-11-01T05:30:00Z" {
		t.Fatal(got)
	}
}
