package httpapi_test

import (
	"github.com/Lewin671/keduly/internal/api"
	"testing"
	"time"
)

func habitCreate(t *testing.T, c interface {
	Call(string, string, any, int, any)
}, body M) api.Habit {
	t.Helper()
	var r struct {
		Habit api.Habit `json:"habit"`
	}
	c.Call("POST", "/habits", body, 201, &r)
	return r.Habit
}
func TestHabitHistoryIsolationAndTimeBudget(t *testing.T) {
	s, c := setup(t)
	h := habitCreate(t, c, M{"title": "Walk", "time": "11:00", "duration_minutes": 20, "reminder_minutes": 0})
	var before api.Today
	c.Call("GET", "/today", nil, 200, &before)
	if len(before.Habits) != 1 || before.FreeMinutes != 460 {
		t.Fatalf("today %+v", before)
	}
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"status": "done"}, 200, nil)
	c.Call("PATCH", "/habits/"+h.ID, M{"title": "New walk", "version": h.Version, "paused": true}, 200, nil)
	var r struct {
		Occurrences []api.HabitOccurrence `json:"occurrences"`
	}
	c.Call("GET", "/habit-occurrences?from=2026-10-13&to=2026-10-15", nil, 200, &r)
	if len(r.Occurrences) != 1 || r.Occurrences[0].Title != "Walk" || r.Occurrences[0].Status != "done" {
		t.Fatalf("history %+v", r)
	}
	c.Call("PATCH", "/habits/"+h.ID, M{"title": "stale", "version": h.Version}, 409, nil)
	other := s.Register("other@example.com")
	other.Call("PATCH", "/habits/"+h.ID, M{"title": "attack", "version": 2}, 404, nil)
	other.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"status": "done"}, 404, nil)
	var empty struct {
		Habits []api.Habit `json:"habits"`
	}
	other.Call("GET", "/habits", nil, 200, &empty)
	if len(empty.Habits) != 0 {
		t.Fatal("account leak")
	}
}
func TestHabitMidnightMoveUndoAndFuture(t *testing.T) {
	s, c := setup(t)
	h := habitCreate(t, c, M{"title": "Bed", "time": "00:00", "day_offset": 1, "reminder_minutes": 15})
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-14", M{"status": "done"}, 400, nil)
	s.SetNow(s.Now().Add(15 * time.Hour))
	var today api.Today
	c.Call("GET", "/today", nil, 200, &today)
	if len(today.Habits) != 2 {
		t.Fatalf("midnight carry: %+v", today.Habits)
	}
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"status": "done"}, 200, nil)
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"status": "open"}, 200, nil)
	var r struct {
		Occurrences []api.HabitOccurrence `json:"occurrences"`
	}
	c.Call("GET", "/habit-occurrences?from=2026-10-13&to=2026-10-13", nil, 200, &r)
	if r.Occurrences[0].CompletedAt != nil || r.Occurrences[0].Status != "open" {
		t.Fatal(r)
	}
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"scheduled_at": "2026-10-14T15:00:00Z"}, 200, nil)
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"scheduled_at": "2027-01-01T15:00:00Z"}, 400, nil)
}
func TestHabitDryRunAndPointDoesNotSplitFreeTime(t *testing.T) {
	_, c := setup(t)
	c.Call("POST", "/habits?dry_run=1", M{"title": "dry"}, 201, nil)
	var r struct {
		Habits []api.Habit `json:"habits"`
	}
	c.Call("GET", "/habits", nil, 200, &r)
	if len(r.Habits) != 0 {
		t.Fatal("dry-run wrote")
	}
	habitCreate(t, c, M{"title": "point", "time": "11:00"})
	var slots struct {
		Slots []api.Slot `json:"slots"`
	}
	c.Call("GET", "/free?date=2026-10-13&duration=450", nil, 200, &slots)
	if len(slots.Slots) != 1 {
		t.Fatal(slots)
	}
}

func TestHabitAccountZoneChangePreservesHistoryAndCalendarMetadata(t *testing.T) {
	_, c := setup(t)
	h := habitCreate(t, c, M{"title": "Walk", "time": "11:00", "duration_minutes": 20})
	c.Call("PATCH", "/me", M{"timezone": "America/New_York"}, 200, nil)
	var r struct {
		Occurrences []api.HabitOccurrence `json:"occurrences"`
	}
	c.Call("GET", "/habit-occurrences?from=2026-10-13&to=2026-10-14", nil, 200, &r)
	// The new account day is October 12, so its next day is October 13. The change
	// must not overwrite already existing historical plans in the old zone.
	_ = h
	if len(r.Occurrences) != 2 || r.Occurrences[0].Timezone != "Asia/Shanghai" || r.Occurrences[1].Timezone != "America/New_York" {
		t.Fatal(r)
	}
	events := listEvents(t, c, "2026-10-13T00:00:00Z", "2026-10-15T00:00:00Z")
	if len(events) == 0 || events[0].CreatedBy.Kind != "system" || events[0].UpdatedAt == "" {
		t.Fatal(events)
	}
}

func TestHabitProjectDeletionKeepsRecordsEditable(t *testing.T) {
	_, c := setup(t)
	p := mkProject(t, c, "Health")
	h := habitCreate(t, c, M{"title": "Walk", "project_id": p.ID})
	c.Call("DELETE", "/projects/"+p.ID, nil, 204, nil)
	c.Call("PATCH", "/habits/"+h.ID, M{"version": 1, "paused": true}, 200, nil)
	var r struct {
		Occurrences []api.HabitOccurrence `json:"occurrences"`
	}
	c.Call("GET", "/habit-occurrences?from=2026-10-13&to=2026-10-13", nil, 200, &r)
	if len(r.Occurrences) != 1 || r.Occurrences[0].ProjectID != nil {
		t.Fatal(r)
	}
}

func TestMovedHabitFollowsPhysicalPlanDate(t *testing.T) {
	_, c := setup(t)
	h := habitCreate(t, c, M{"title": "Walk", "time": "11:00"})
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"scheduled_at": "2026-10-14T11:30:00+08:00"}, 200, nil)
	var today api.Today
	c.Call("GET", "/today", nil, 200, &today)
	if len(today.Habits) != 0 {
		t.Fatal(today.Habits)
	}
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"status": "done"}, 400, nil)
	var days struct {
		Days []api.Day `json:"days"`
	}
	c.Call("GET", "/upcoming?from=2026-10-14&to=2026-10-14", nil, 200, &days)
	if len(days.Days) != 1 || len(days.Days[0].Habits) != 2 {
		t.Fatal(days)
	}
	var r struct {
		Occurrences []api.HabitOccurrence `json:"occurrences"`
	}
	c.Call("GET", "/habit-occurrences?from=2026-10-13&to=2026-10-13", nil, 200, &r)
	if r.Occurrences[0].PlanDate != "2026-10-14" || r.Occurrences[0].Status != "future" {
		t.Fatal(r)
	}
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"scheduled_at": "2026-10-21T00:00:00+08:00"}, 400, nil)
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"scheduled_at": "2026-10-20T23:59:00+08:00"}, 200, nil)
}

func TestHabitActivityUndoRedoAndPlanWindow(t *testing.T) {
	s, c := setup(t)
	h := habitCreate(t, c, M{"title": "Walk", "time": "11:00", "reason": "requested daily walk"})
	acts := activities(t, c)
	if len(acts) != 1 || acts[0].Reason == nil || *acts[0].Reason != "requested daily walk" || !acts[0].Undoable {
		t.Fatal(acts)
	}
	c.Call("POST", "/activity/"+acts[0].ID+"/undo", nil, 200, nil)
	c.Call("POST", "/activity/"+acts[0].ID+"/redo", nil, 200, nil)
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"status": "done", "reason": "user reported completion"}, 200, nil)
	record := activities(t, c)[0]
	c.Call("POST", "/activity/"+acts[0].ID+"/undo", nil, 409, nil)
	c.Call("POST", "/activity/"+record.ID+"/undo", nil, 200, nil)
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"scheduled_at": "2026-10-20T11:00:00+08:00"}, 200, nil)
	s.SetNow(s.Now().AddDate(0, 0, 7))
	var history struct {
		Occurrences []api.HabitOccurrence `json:"occurrences"`
	}
	c.Call("GET", "/habit-occurrences?from=2026-10-20&to=2026-10-20&basis=plan", nil, 200, &history)
	if len(history.Occurrences) != 2 {
		t.Fatal(history)
	}
	c.Call("GET", "/habit-occurrences?from=2026-10-13&to=2026-10-13&basis=plan", nil, 200, &history)
	if len(history.Occurrences) != 0 {
		t.Fatal(history)
	}
}

func TestHabitRuleBoundaryNeverRegressesAcrossTimezones(t *testing.T) {
	_, c := setup(t)
	h := habitCreate(t, c, M{"title": "Original history", "time": "11:00"})
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"status": "done"}, 200, nil)
	c.Call("PATCH", "/me", M{"timezone": "America/New_York"}, 200, nil)
	var edited struct {
		Habit api.Habit `json:"habit"`
	}
	c.Call("PATCH", "/habits/"+h.ID, M{"version": 2, "title": "Future rule"}, 200, &edited)
	if edited.Habit.EffectiveDate != "2026-10-14" {
		t.Fatal(edited)
	}
	c.Call("PATCH", "/me", M{"timezone": "Pacific/Honolulu"}, 200, nil)
	var r struct {
		Occurrences []api.HabitOccurrence `json:"occurrences"`
	}
	c.Call("GET", "/habit-occurrences?from=2026-10-13&to=2026-10-13", nil, 200, &r)
	if len(r.Occurrences) != 1 || r.Occurrences[0].Title != "Original history" || r.Occurrences[0].Timezone != "Asia/Shanghai" || r.Occurrences[0].Status != "done" {
		t.Fatal(r)
	}
}
func TestHabitDisplayOrderUsesMovedInstant(t *testing.T) {
	_, c := setup(t)
	a := habitCreate(t, c, M{"title": "Early", "time": "09:00"})
	b := habitCreate(t, c, M{"title": "Mid", "time": "10:00"})
	c.Call("PATCH", "/habits/"+a.ID+"/occurrences/2026-10-13", M{"scheduled_at": "2026-10-13T20:00:00+08:00"}, 200, nil)
	var view api.Today
	c.Call("GET", "/today", nil, 200, &view)
	if len(view.Habits) != 2 || view.Habits[0].HabitID != b.ID || view.Habits[1].HabitID != a.ID {
		t.Fatal(view.Habits)
	}
}

func TestNextDayNoOpMoveKeepsTonight(t *testing.T) {
	_, c := setup(t)
	h := habitCreate(t, c, M{"title": "Bed", "time": "00:00", "day_offset": 1})
	var out struct {
		Occurrence api.HabitOccurrence `json:"occurrence"`
	}
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"scheduled_at": "2026-10-14T00:00:00+08:00"}, 200, &out)
	if out.Occurrence.PlanDate != "2026-10-13" || out.Occurrence.Status != "open" {
		t.Fatal(out)
	}
	var today api.Today
	c.Call("GET", "/today", nil, 200, &today)
	if len(today.Habits) != 1 {
		t.Fatal(today)
	}
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"status": "done"}, 200, nil)
}
func TestRuleEditCannotHideFutureRecord(t *testing.T) {
	_, c := setup(t)
	h := habitCreate(t, c, M{"title": "Walk", "time": "11:00"})
	path := "/habits/" + h.ID + "/occurrences/2026-10-14"
	c.Call("PATCH", path, M{"scheduled_at": "2026-10-14T12:00:00+08:00"}, 200, nil)
	move := activities(t, c)[0]
	wantCode(t, c, "PATCH", "/habits/"+h.ID, M{"version": 1, "paused": true}, 409, "habit_records_depend")
	wantCode(t, c, "PATCH", "/habits/"+h.ID, M{"version": 1, "weekdays": []int{1}}, 409, "habit_records_depend")
	c.Call("PATCH", path, M{"status": "open"}, 200, nil)
	// An unchanged open write did not hide or replace the move; it remains undoable.
	c.Call("POST", "/activity/"+move.ID+"/undo", nil, 200, nil)
	c.Call("PATCH", "/habits/"+h.ID, M{"version": 1, "paused": true}, 200, nil)
	pause := activities(t, c)[0]
	c.Call("POST", "/activity/"+pause.ID+"/undo", nil, 200, nil)
	c.Call("PATCH", path, M{"scheduled_at": "2026-10-14T12:00:00+08:00"}, 200, nil)
	c.Call("PATCH", path, M{"scheduled_at": nil, "status": "open"}, 200, nil)
	c.Call("PATCH", "/habits/"+h.ID, M{"version": 1, "paused": true}, 200, nil)
}
func TestPauseCannotLeaveEarlierMoveIntoPausedPeriod(t *testing.T) {
	_, c := setup(t)
	h := habitCreate(t, c, M{"title": "Walk", "time": "11:00"})
	c.Call("PATCH", "/habits/"+h.ID+"/occurrences/2026-10-13", M{"scheduled_at": "2026-10-14T12:00:00+08:00"}, 200, nil)
	wantCode(t, c, "PATCH", "/habits/"+h.ID, M{"version": 1, "paused": true}, 409, "habit_records_depend")
}
