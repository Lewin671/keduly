package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/testutil"
)

// seedDay fills Tuesday 13 October 2026 (Asia/Shanghai, working 09:00-18:00):
//
//	08:00-09:30 early call     10:00-11:30 review     11:00-12:00 sync (overlaps)
//	14:00-15:00 block for "planned today"             16:00-17:00 pending suggestion
//	all day: holiday
func seedDay(t *testing.T) (me *testutil.Client, ids map[string]string) {
	t.Helper()
	_, c := setup(t)
	ids = map[string]string{}
	mkEvent(t, c, M{"title": "early call", "start": "2026-10-13T00:00:00Z", "end": "2026-10-13T01:30:00Z"})
	mkEvent(t, c, M{"title": "review", "start": "2026-10-13T02:00:00Z", "end": "2026-10-13T03:30:00Z"})
	mkEvent(t, c, M{"title": "sync", "start": "2026-10-13T03:00:00Z", "end": "2026-10-13T04:00:00Z"})
	mkEvent(t, c, M{"title": "holiday", "all_day": true, "start_date": day, "end_date": day})
	// 01:00 on the 14th local time: not today.
	mkEvent(t, c, M{"title": "late", "start": "2026-10-13T17:00:00Z", "end": "2026-10-13T18:00:00Z"})

	planned := mkItem(t, c, M{"title": "planned today", "planned_date": day, "estimate_minutes": 60})
	c.Call("POST", "/items/"+planned.ID+"/schedule", M{"start": "2026-10-13T06:00:00Z", "end": "2026-10-13T07:00:00Z"}, http.StatusOK, nil)
	suggested := mkItem(t, c, M{"title": "suggested", "planned_date": day, "estimate_minutes": 45})
	agent := c.NewToken("Claude Code", "agent", "write", true)
	sug := suggest(t, agent, M{"kind": "schedule_item", "item_id": suggested.ID, "start": "2026-10-13T08:00:00Z", "end": "2026-10-13T09:00:00Z", "reason": "free hour"})
	mkItem(t, c, M{"title": "important today", "planned_date": day, "estimate_minutes": 30, "important": true})
	mkItem(t, c, M{"title": "carried over", "planned_date": "2026-10-10", "estimate_minutes": 20})
	mkItem(t, c, M{"title": "tonight", "planned_date": day, "estimate_minutes": 120, "evening": true})
	mkItem(t, c, M{"title": "tomorrow", "planned_date": "2026-10-14"})
	mkItem(t, c, M{"title": "due tomorrow", "due_date": "2026-10-14"})
	mkItem(t, c, M{"title": "overdue", "due_date": "2026-10-12"})
	mkItem(t, c, M{"title": "inbox"})
	done := mkItem(t, c, M{"title": "done today", "estimate_minutes": 15})
	c.Call("PATCH", "/items/"+done.ID, M{"status": "done"}, http.StatusOK, nil)
	ids["planned"], ids["suggested"], ids["suggestion"] = planned.ID, suggested.ID, sug.ID
	return c, ids
}

func TestToday(t *testing.T) {
	me, ids := seedDay(t)
	var today api.Today
	me.Call("GET", "/today", nil, http.StatusOK, &today)
	if today.Date != day {
		t.Fatalf("date %s", today.Date)
	}
	want := []string{"important today", "planned today", "suggested", "carried over", "tonight", "done today"}
	if !equal(titles(today.Items), want) {
		t.Fatalf("items %v, want %v", titles(today.Items), want)
	}
	if !equal(titles(today.Overdue), []string{"overdue"}) {
		t.Fatalf("overdue %v", titles(today.Overdue))
	}
	var events []string
	for _, e := range today.Events {
		events = append(events, e.Title)
	}
	if !equal(events, []string{"holiday", "early call", "review", "sync"}) {
		t.Fatalf("events %v", events)
	}
	// 540 working minutes minus 30 (early call inside the window), 120 (review
	// and sync merged), 60 (block) and 60 (pending suggestion).
	if today.FreeMinutes != 270 {
		t.Fatalf("free_minutes %d, want 270", today.FreeMinutes)
	}
	// "important today" 30 + "carried over" 20; the others have a block, a
	// suggestion, are for the evening or are done.
	if today.UnplannedMinutes != 50 {
		t.Fatalf("unplanned_minutes %d, want 50", today.UnplannedMinutes)
	}
	for _, it := range today.Items {
		switch it.ID {
		case ids["planned"]:
			if it.Block == nil {
				t.Fatal("planned item has no block")
			}
		case ids["suggested"]:
			if it.Suggestion == nil || it.Suggestion.ID != ids["suggestion"] || it.Suggestion.Actor.Name != "Claude Code" || it.Suggestion.Reason != "free hour" {
				t.Fatalf("suggestion %+v", it.Suggestion)
			}
		}
	}

	var counts struct {
		Counts api.Counts `json:"counts"`
	}
	me.Call("GET", "/counts", nil, http.StatusOK, &counts)
	// inbox: everything open without a project (9); today: 5 planned and open plus 1 overdue.
	if counts.Counts != (api.Counts{Inbox: 9, Today: 6, Pending: 1}) {
		t.Fatalf("counts %+v", counts.Counts)
	}
}

func TestFree(t *testing.T) {
	me, _ := seedDay(t)
	slots := func(query string) []api.Slot {
		t.Helper()
		var resp struct {
			Slots []api.Slot `json:"slots"`
		}
		me.Call("GET", "/free?"+query, nil, http.StatusOK, &resp)
		return resp.Slots
	}
	got := slots("date=" + day + "&duration=60")
	want := []api.Slot{
		{Start: "2026-10-13T04:00:00Z", End: "2026-10-13T06:00:00Z"}, // 12:00-14:00
		{Start: "2026-10-13T07:00:00Z", End: "2026-10-13T08:00:00Z"}, // 15:00-16:00
		{Start: "2026-10-13T09:00:00Z", End: "2026-10-13T10:00:00Z"}, // 17:00-18:00
	}
	if len(got) != len(want) {
		t.Fatalf("slots %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slot %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got := slots("date=" + day + "&duration=90"); len(got) != 1 || got[0] != want[0] {
		t.Fatalf("90-minute slots %+v", got)
	}
	if got := slots("date=" + day + "&duration=30"); len(got) != 4 || got[0].Start != "2026-10-13T01:30:00Z" {
		t.Fatalf("30-minute slots %+v", got)
	}
	// An empty day is one slot spanning the working hours, in the user's zone.
	if got := slots("date=2026-10-20&duration=60"); len(got) != 1 || got[0] != (api.Slot{Start: "2026-10-20T01:00:00Z", End: "2026-10-20T10:00:00Z"}) {
		t.Fatalf("empty day %+v", got)
	}
	wantCode(t, me, "GET", "/free?date=someday&duration=60", nil, 400, "invalid_request")
	wantCode(t, me, "GET", "/free?date="+day+"&duration=soon", nil, 400, "invalid_request")

	// Working hours and time zone are the user's.
	me.Call("PATCH", "/me", M{"work_start": "13:00", "work_end": "15:30"}, http.StatusOK, nil)
	if got := slots("date=" + day + "&duration=30"); len(got) != 2 || got[0] != (api.Slot{Start: "2026-10-13T05:00:00Z", End: "2026-10-13T06:00:00Z"}) ||
		got[1] != (api.Slot{Start: "2026-10-13T07:00:00Z", End: "2026-10-13T07:30:00Z"}) {
		t.Fatalf("narrow window %+v", got)
	}
	me.Call("PATCH", "/me", M{"timezone": "America/New_York", "work_start": "09:00", "work_end": "18:00"}, http.StatusOK, nil)
	// In New York the clock reads 22:00 on the 12th; that day's window is 13:00Z-22:00Z.
	var today api.Today
	me.Call("GET", "/today", nil, http.StatusOK, &today)
	if today.Date != "2026-10-12" {
		t.Fatalf("today in New York %s", today.Date)
	}
	if got := slots("date=2026-10-12&duration=60"); len(got) != 1 || got[0] != (api.Slot{Start: "2026-10-12T13:00:00Z", End: "2026-10-12T22:00:00Z"}) {
		t.Fatalf("New York slots %+v", got)
	}
	wantCode(t, me, "PATCH", "/me", M{"work_start": "19:00"}, 400, "invalid_request")
	wantCode(t, me, "PATCH", "/me", M{"timezone": "Nowhere"}, 400, "invalid_request")
}

func TestUpcomingOverviewMatrixHeat(t *testing.T) {
	me, _ := seedDay(t)
	var up struct {
		Days []api.Day `json:"days"`
	}
	me.Call("GET", "/upcoming?from=2026-10-14&to=2026-10-20", nil, http.StatusOK, &up)
	if len(up.Days) != 1 || up.Days[0].Date != "2026-10-14" {
		t.Fatalf("upcoming days %+v", up.Days)
	}
	if got := titles(up.Days[0].Items); !equal(got, []string{"tomorrow", "due tomorrow"}) {
		t.Fatalf("upcoming items %v", got)
	}
	if len(up.Days[0].Events) != 1 || up.Days[0].Events[0].Title != "late" {
		t.Fatalf("upcoming events %+v", up.Days[0].Events)
	}
	wantCode(t, me, "GET", "/upcoming?from=2026-10-14&to=2027-10-20", nil, 400, "invalid_request")
	wantCode(t, me, "GET", "/upcoming?from=2026-10-14&to=2026-10-01", nil, 400, "invalid_request")

	var heat struct {
		Days map[string]int `json:"days"`
	}
	me.Call("GET", "/calendar/heat?year=2026", nil, http.StatusOK, &heat)
	// 13th: early call, review, sync, holiday and one time block. 14th: "late".
	if len(heat.Days) != 2 || heat.Days["2026-10-13"] != 5 || heat.Days["2026-10-14"] != 1 {
		t.Fatalf("heat %+v", heat.Days)
	}
	wantCode(t, me, "GET", "/calendar/heat?year=soon", nil, 400, "invalid_request")

	var matrix struct {
		Quadrants map[string]api.Quadrant `json:"quadrants"`
	}
	me.Call("GET", "/matrix", nil, http.StatusOK, &matrix)
	q := matrix.Quadrants
	// Only "important today" is important, and it has no deadline.
	if q["do"].Total != 0 || q["plan"].Total != 1 || q["plan"].Unplanned != 1 {
		t.Fatalf("important quadrants %+v %+v", q["do"], q["plan"])
	}
	if !equal(titles(q["quick"].Items), []string{"due tomorrow", "overdue"}) || q["quick"].Unplanned != 2 {
		t.Fatalf("quick %+v", q["quick"])
	}
	// later: planned today, suggested, carried over, tonight, tomorrow, inbox.
	if q["later"].Total != 6 || len(q["later"].Items) != 6 || q["later"].Unplanned != 4 {
		t.Fatalf("later %+v", q["later"])
	}
	mkItem(t, me, M{"title": "seventh"})
	me.Call("GET", "/matrix", nil, http.StatusOK, &matrix)
	if l := matrix.Quadrants["later"]; l.Total != 7 || len(l.Items) != 6 {
		t.Fatalf("a quadrant shows at most six items: %+v", l)
	}

	project := mkProject(t, me, "Big")
	empty := mkProject(t, me, "Empty")
	for i := 0; i < 7; i++ {
		mkItem(t, me, M{"title": "task", "project_id": project.ID})
	}
	var overview struct {
		Projects []api.OverviewProject `json:"projects"`
	}
	me.Call("GET", "/overview", nil, http.StatusOK, &overview)
	if len(overview.Projects) != 1 || overview.Projects[0].ProjectID != project.ID || overview.Projects[0].Total != 7 || len(overview.Projects[0].Items) != 5 {
		t.Fatalf("overview %+v (empty project %s must be left out)", overview.Projects, empty.ID)
	}
}
