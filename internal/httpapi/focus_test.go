package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/testutil"
)

func focusCall(t *testing.T, c *testutil.Client, method, path string, body any) api.Focus {
	t.Helper()
	var resp struct {
		Focus api.Focus `json:"focus"`
	}
	c.Call(method, path, body, http.StatusOK, &resp)
	return resp.Focus
}

func getFocus(t *testing.T, c *testutil.Client) api.Focus {
	t.Helper()
	return focusCall(t, c, "GET", "/focus", nil)
}

// tomato runs one full tomato on the item ("" = free focus) and leaves the timer idle.
func tomato(t *testing.T, s *testutil.Server, c *testutil.Client, itemID string) {
	t.Helper()
	body := M{}
	if itemID != "" {
		body["item_id"] = itemID
	}
	focusCall(t, c, "POST", "/focus/start", body)
	s.SetNow(s.Now().Add(25 * time.Minute))
	focusCall(t, c, "POST", "/focus/stop", nil)
}

func wantFocus(t *testing.T, f api.Focus, state string, tomatoes, roundDone, rest int) {
	t.Helper()
	if f.State != state || f.TomatoesToday != tomatoes || f.RoundDone != roundDone || f.RestMinutes != rest {
		t.Fatalf("focus = state %s, tomatoes %d, round %d, rest %d; want %s, %d, %d, %d",
			f.State, f.TomatoesToday, f.RoundDone, f.RestMinutes, state, tomatoes, roundDone, rest)
	}
}

func TestFocusTomato(t *testing.T) {
	s, c := setup(t)
	item := mkItem(t, c, M{"title": "write the sync", "estimate_minutes": 90})

	wantFocus(t, getFocus(t, c), "idle", 0, 0, 5)
	f := focusCall(t, c, "POST", "/focus/start", M{"item_id": item.ID})
	wantFocus(t, f, "work", 0, 0, 5)
	if f.Session == nil || f.Session.Title != "write the sync" || f.Session.PlannedMinutes != 25 || f.Session.Completed {
		t.Fatalf("session %+v", f.Session)
	}
	start, _ := time.Parse(time.RFC3339, f.Session.Start)
	end, _ := time.Parse(time.RFC3339, f.Session.End)
	if end.Sub(start) != 25*time.Minute {
		t.Fatalf("a running session ends at its planned end, got %s to %s", f.Session.Start, f.Session.End)
	}
	if got := getItem(t, c, item.ID).Focus; got.Tomatoes != 0 || got.Minutes != 0 {
		t.Fatalf("a running session must not count yet: %+v", got)
	}

	// Nothing has to happen for the tomato to complete: the clock passing its end is enough.
	s.SetNow(s.Now().Add(25 * time.Minute))
	f = getFocus(t, c)
	wantFocus(t, f, "over", 1, 1, 5)
	if !f.Session.Completed || f.MinutesToday != 25 {
		t.Fatalf("over: %+v, minutes %d", f.Session, f.MinutesToday)
	}
	if got := getItem(t, c, item.ID).Focus; got.Tomatoes != 1 || got.Minutes != 25 {
		t.Fatalf("item focus %+v", got)
	}

	f = focusCall(t, c, "POST", "/focus/rest", nil)
	wantFocus(t, f, "rest", 1, 1, 5)
	if f.Session.Kind != "rest" || f.Session.PlannedMinutes != 5 || f.Session.ItemID != nil {
		t.Fatalf("rest session %+v", f.Session)
	}
	s.SetNow(s.Now().Add(5 * time.Minute))
	wantFocus(t, getFocus(t, c), "idle", 1, 1, 5)
}

func TestFocusGiveUp(t *testing.T) {
	s, c := setup(t)
	item := mkItem(t, c, M{"title": "a"})

	// Given up within the first minute: a slip, no record.
	focusCall(t, c, "POST", "/focus/start", M{"item_id": item.ID})
	s.SetNow(s.Now().Add(40 * time.Second))
	wantFocus(t, focusCall(t, c, "POST", "/focus/stop", nil), "idle", 0, 0, 5)
	if got := getItem(t, c, item.ID).Focus; got.Minutes != 0 {
		t.Fatalf("a slip left %+v", got)
	}

	// Given up after 12 minutes: the minutes stay, no tomato.
	focusCall(t, c, "POST", "/focus/start", M{"item_id": item.ID})
	s.SetNow(s.Now().Add(12 * time.Minute))
	f := focusCall(t, c, "POST", "/focus/stop", nil)
	wantFocus(t, f, "idle", 0, 0, 5)
	if f.MinutesToday != 12 {
		t.Fatalf("minutes_today %d, want 12", f.MinutesToday)
	}
	if got := getItem(t, c, item.ID).Focus; got.Tomatoes != 0 || got.Minutes != 12 {
		t.Fatalf("item focus %+v", got)
	}

	// Starting on something else gives up what was running.
	other := mkItem(t, c, M{"title": "b"})
	focusCall(t, c, "POST", "/focus/start", M{"item_id": item.ID})
	s.SetNow(s.Now().Add(3 * time.Minute))
	f = focusCall(t, c, "POST", "/focus/start", M{"item_id": other.ID})
	if f.State != "work" || *f.Session.ItemID != other.ID || f.MinutesToday != 15 {
		t.Fatalf("after switching: %+v, minutes %d", f.Session, f.MinutesToday)
	}
	wantCode(t, c, "POST", "/focus/rest", nil, http.StatusConflict, "conflict")
}

func TestFocusRoundAndLongRest(t *testing.T) {
	s, c := setup(t)
	for i := 0; i < 3; i++ {
		tomato(t, s, c, "")
	}
	wantFocus(t, getFocus(t, c), "idle", 3, 3, 5)

	// The fourth tomato completes the round: all marks filled, and the long rest is next.
	focusCall(t, c, "POST", "/focus/start", nil)
	s.SetNow(s.Now().Add(25 * time.Minute))
	wantFocus(t, getFocus(t, c), "over", 4, 4, 15)
	f := focusCall(t, c, "POST", "/focus/rest", nil)
	wantFocus(t, f, "rest", 4, 4, 15)
	if f.Session.PlannedMinutes != 15 {
		t.Fatalf("long rest is %d minutes", f.Session.PlannedMinutes)
	}
	// Once that rest is over, a new round begins.
	s.SetNow(s.Now().Add(15 * time.Minute))
	wantFocus(t, getFocus(t, c), "idle", 4, 0, 15)
	tomato(t, s, c, "")
	wantFocus(t, getFocus(t, c), "idle", 5, 1, 5)
}

func TestFocusCompletingTheItem(t *testing.T) {
	s, c := setup(t)
	item := mkItem(t, c, M{"title": "a"})
	focusCall(t, c, "POST", "/focus/start", M{"item_id": item.ID})
	s.SetNow(s.Now().Add(10 * time.Minute))
	var resp struct {
		Item api.Item `json:"item"`
	}
	c.Call("PATCH", "/items/"+item.ID, M{"status": "done"}, http.StatusOK, &resp)
	if resp.Item.Focus.Minutes != 10 {
		t.Fatalf("completing kept %+v, want 10 minutes", resp.Item.Focus)
	}
	wantFocus(t, getFocus(t, c), "idle", 0, 0, 5)
	wantCode(t, c, "POST", "/focus/start", M{"item_id": item.ID}, http.StatusBadRequest, "invalid_request")

	// A tomato that ran out on an item is answered by completing the item.
	other := mkItem(t, c, M{"title": "b"})
	focusCall(t, c, "POST", "/focus/start", M{"item_id": other.ID})
	s.SetNow(s.Now().Add(25 * time.Minute))
	wantFocus(t, getFocus(t, c), "over", 1, 1, 5)
	c.Call("PATCH", "/items/"+other.ID, M{"status": "done"}, http.StatusOK, nil)
	wantFocus(t, getFocus(t, c), "idle", 1, 1, 5)
}

func TestFocusSessionsAndStats(t *testing.T) {
	s, c := setup(t)
	p := mkProject(t, c, "Keduly")
	inProject := mkItem(t, c, M{"title": "in project", "project_id": p.ID})
	inbox := mkItem(t, c, M{"title": "inbox"})

	// Two days ago, yesterday and today all have a tomato; three days ago does not.
	base := s.Now()
	s.SetNow(base.Add(-48 * time.Hour))
	tomato(t, s, c, inProject.ID)
	s.SetNow(base.Add(-24 * time.Hour))
	tomato(t, s, c, inProject.ID)
	tomato(t, s, c, inbox.ID)
	s.SetNow(base)
	tomato(t, s, c, "")
	focusCall(t, c, "POST", "/focus/start", M{"item_id": inProject.ID})
	s.SetNow(s.Now().Add(10 * time.Minute))
	focusCall(t, c, "POST", "/focus/stop", nil)

	var stats api.FocusStats
	c.Call("GET", "/focus/stats", nil, http.StatusOK, &stats)
	if len(stats.Days) != 7 || stats.Days[6].Date != day {
		t.Fatalf("days %+v", stats.Days)
	}
	if stats.Tomatoes != 4 || stats.Minutes != 110 || stats.Streak != 3 {
		t.Fatalf("stats: %d tomatoes, %d minutes, streak %d; want 4, 110, 3", stats.Tomatoes, stats.Minutes, stats.Streak)
	}
	if d := stats.Days[5]; d.Tomatoes != 2 || d.Minutes != 50 || len(d.Projects) != 2 {
		t.Fatalf("yesterday %+v", d)
	}
	if d := stats.Days[6]; d.Tomatoes != 1 || d.Minutes != 35 {
		t.Fatalf("today %+v", d)
	}
	// The project leads with 2 tomatoes and 60 minutes; free focus and the inbox share the null entry.
	if len(stats.Projects) != 2 || stats.Projects[0].ProjectID == nil || *stats.Projects[0].ProjectID != p.ID ||
		stats.Projects[0].Tomatoes != 2 || stats.Projects[0].Minutes != 60 ||
		stats.Projects[1].ProjectID != nil || stats.Projects[1].Tomatoes != 2 || stats.Projects[1].Minutes != 50 {
		t.Fatalf("projects %+v", stats.Projects)
	}

	var list struct {
		Sessions []api.FocusSession `json:"sessions"`
	}
	c.Call("GET", "/focus/sessions?from="+day+"&to="+day, nil, http.StatusOK, &list)
	if len(list.Sessions) != 2 || list.Sessions[0].Title != "" || !list.Sessions[0].Completed ||
		list.Sessions[1].Title != "in project" || list.Sessions[1].Completed || *list.Sessions[1].ProjectID != p.ID {
		t.Fatalf("sessions %+v", list.Sessions)
	}
	wantCode(t, c, "GET", "/focus/sessions?from=2026-01-01&to=2026-12-31", nil, http.StatusBadRequest, "invalid_request")

	var detail api.ProjectDetail
	c.Call("GET", "/projects/"+p.ID, nil, http.StatusOK, &detail)
	// Monday 12 October and Tuesday 13 October: 25 + 10 minutes. Sunday's tomato is last week's.
	if detail.FocusWeekMinutes != 35 {
		t.Fatalf("focus_week_minutes %d, want 35", detail.FocusWeekMinutes)
	}

	// Deleting the item keeps its sessions, with the title they had and no project.
	c.Call("DELETE", "/items/"+inProject.ID, nil, http.StatusNoContent, nil)
	c.Call("GET", "/focus/sessions?from="+day+"&to="+day, nil, http.StatusOK, &list)
	if len(list.Sessions) != 2 || list.Sessions[1].Title != "in project" || list.Sessions[1].ProjectID != nil {
		t.Fatalf("after deleting the item: %+v", list.Sessions)
	}
}

func TestFocusSettingsAndToday(t *testing.T) {
	s, c := setup(t)
	var me struct {
		User api.User `json:"user"`
	}
	c.Call("PATCH", "/me", M{"focus_minutes": 50, "rest_minutes": 10, "long_rest_minutes": 30, "round_size": 2}, http.StatusOK, &me)
	if me.User.FocusMinutes != 50 || me.User.RoundSize != 2 {
		t.Fatalf("user %+v", me.User)
	}
	wantCode(t, c, "PATCH", "/me", M{"round_size": 1}, http.StatusBadRequest, "invalid_request")
	wantCode(t, c, "PATCH", "/me", M{"focus_minutes": 0}, http.StatusBadRequest, "invalid_request")

	item := mkItem(t, c, M{"title": "a", "planned_date": day, "estimate_minutes": 60})
	f := focusCall(t, c, "POST", "/focus/start", M{"item_id": item.ID})
	if f.Session.PlannedMinutes != 50 || f.RoundSize != 2 {
		t.Fatalf("focus %+v", f)
	}
	s.SetNow(s.Now().Add(50 * time.Minute))
	focusCall(t, c, "POST", "/focus/stop", nil)
	var today api.Today
	c.Call("GET", "/today", nil, http.StatusOK, &today)
	if today.UnplannedMinutes != 10 {
		t.Fatalf("unplanned_minutes %d, want 60 - 50", today.UnplannedMinutes)
	}

	// A read-only token may look at the timer but not start it; a dry run starts nothing.
	reader := c.NewToken("Brief", "agent", "read", true)
	wantFocus(t, getFocus(t, reader), "idle", 1, 1, 10)
	wantCode(t, reader, "POST", "/focus/start", nil, http.StatusForbidden, "forbidden")
	c.Call("POST", "/focus/start?dry_run=1", nil, http.StatusOK, nil)
	wantFocus(t, getFocus(t, c), "idle", 1, 1, 10)
}
