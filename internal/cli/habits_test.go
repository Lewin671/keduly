package cli_test

import (
	"encoding/json"
	"github.com/Lewin671/keduly/internal/api"
	"testing"
)

func TestHabitCLI(t *testing.T) {
	h := newHarness(t, true)
	out := h.ok("habit", "add", "Daily walk", "--time", "11:00", "--duration", "20", "--reminder", "0", "--json")
	var r struct {
		Habit api.Habit `json:"habit"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if r.Habit.ID == "" {
		t.Fatal(out)
	}
	wantIn(t, h.ok("habit", "list"), "Daily walk")
	wantIn(t, h.ok("habit", "done", r.Habit.ID, "--date", "2026-10-13", "--json"), "done")
	wantIn(t, h.ok("habit", "log", r.Habit.ID, "--from", "2026-10-13", "--to", "2026-10-13", "--json"), "Daily walk")
	h.ok("habit", "pause", r.Habit.ID)
	h.ok("habit", "resume", r.Habit.ID)
}

func TestHabitCLIFieldsAndBuiltInLists(t *testing.T) {
	h := newHarness(t, true)
	out := h.ok("habit", "add", "Walk only", "--time", "11:00", "--notes", "initial note", "--json")
	var r struct {
		Habit api.Habit `json:"habit"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	wantIn(t, h.ok("item", "list", "--view", "today"), "Walk only")
	wantIn(t, h.ok("item", "list", "--view", "upcoming"), "Walk only")
	wantIn(t, h.ok("habit", "edit", r.Habit.ID, "--project", "none", "--notes", "changed note", "--json"), "changed note")
}

func TestHabitLogResolvesShortID(t *testing.T) {
	h := newHarness(t, true)
	out := h.ok("habit", "add", "Short ID habit", "--json")
	var r struct {
		Habit api.Habit `json:"habit"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	wantIn(t, h.ok("habit", "log", r.Habit.ID[:8], "--from", "2026-10-13", "--to", "2026-10-13", "--json"), "Short ID habit")
}

func TestHabitCalendarProjectionIsClearlyReadOnly(t *testing.T) {
	h := newHarness(t, true)
	out := h.ok("habit", "add", "Walk projection", "--time", "11:00", "--json")
	var r struct {
		Habit api.Habit `json:"habit"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	wantIn(t, h.ok("event", "list"), "习惯（只读")
	h.fails(2, []string{"event", "rm", "habit:" + r.Habit.ID + ":2026-10-13"}, "use keduly habit")
}

func TestHabitLogUsesPlanDateAndPreservesRecordKey(t *testing.T) {
	h := newHarness(t, true)
	out := h.ok("habit", "add", "Moved history", "--time", "11:00", "--json")
	var r struct {
		Habit api.Habit `json:"habit"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	h.ok("habit", "move", r.Habit.ID, "--date", "2026-10-13", "--to", "2026-10-14T12:00:00+08:00")
	wantIn(t, h.ok("habit", "log", r.Habit.ID, "--from", "2026-10-14", "--to", "2026-10-14"), "记录日期 2026-10-13", "Moved history")
	out = h.ok("habit", "log", r.Habit.ID, "--from", "2026-10-13", "--to", "2026-10-13", "--json")
	wantOut(t, out, "Moved history")
}
