package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/Lewin671/keduly/internal/api"
)

type itemList struct {
	Items      []api.Item `json:"items"`
	NextCursor *string    `json:"next_cursor"`
	Total      int        `json:"total"`
}

func TestItemCRUD(t *testing.T) {
	_, me := setup(t)
	project := mkProject(t, me, "Keduly")
	var heading struct {
		Heading api.Heading `json:"heading"`
	}
	me.Call("POST", "/projects/"+project.ID+"/headings", M{"name": "Sync"}, http.StatusCreated, &heading)

	item := mkItem(t, me, M{"title": " Implement CalDAV ", "heading_id": heading.Heading.ID, "estimate_minutes": 90,
		"planned_date": day, "due_date": "2026-10-14", "due_time": "18:00", "important": true, "notes": "RFC 4791"})
	if item.Title != "Implement CalDAV" || item.ProjectID == nil || *item.ProjectID != project.ID || item.Status != "open" ||
		*item.EstimateMinutes != 90 || item.Block != nil || item.Suggestion != nil || item.CompletedAt != nil || item.CreatedBy.Kind != "user" {
		t.Fatalf("unexpected item %+v", item)
	}

	var resp struct {
		Item api.Item `json:"item"`
	}
	me.Call("PATCH", "/items/"+item.ID, M{"status": "done"}, http.StatusOK, &resp)
	if resp.Item.Status != "done" || resp.Item.CompletedAt == nil || *resp.Item.CompletedAt != "2026-10-13T02:00:00Z" {
		t.Fatalf("completing: %+v", resp.Item)
	}
	me.Call("PATCH", "/items/"+item.ID, M{"status": "open", "due_date": nil, "due_time": nil, "estimate_minutes": nil}, http.StatusOK, &resp)
	if resp.Item.Status != "open" || resp.Item.CompletedAt != nil || resp.Item.DueDate != nil || resp.Item.EstimateMinutes != nil {
		t.Fatalf("reopening: %+v", resp.Item)
	}
	// Moving to the inbox drops the heading, which belongs to the project.
	me.Call("PATCH", "/items/"+item.ID, M{"project_id": nil}, http.StatusOK, &resp)
	if resp.Item.ProjectID != nil || resp.Item.HeadingID != nil {
		t.Fatalf("moving to the inbox: %+v", resp.Item)
	}
	wantCode(t, me, "PATCH", "/items/"+item.ID, M{"status": "archived"}, 400, "invalid_request")

	me.Call("DELETE", "/items/"+item.ID, nil, http.StatusNoContent, nil)
	wantCode(t, me, "GET", "/items/"+item.ID, nil, 404, "not_found")
}

func TestItemFiltersAndPagination(t *testing.T) {
	_, me := setup(t)
	project := mkProject(t, me, "Work")
	mkItem(t, me, M{"title": "do", "important": true, "due_date": day})
	mkItem(t, me, M{"title": "do tomorrow", "important": true, "due_date": "2026-10-14"})
	mkItem(t, me, M{"title": "plan", "important": true, "due_date": "2026-10-15"})
	mkItem(t, me, M{"title": "plan no date", "important": true, "project_id": project.ID})
	mkItem(t, me, M{"title": "quick", "due_date": "2026-10-01", "notes": "Call the BANK"})
	mkItem(t, me, M{"title": "later", "project_id": project.ID})
	done := mkItem(t, me, M{"title": "finished first", "important": true, "due_date": day})
	done2 := mkItem(t, me, M{"title": "finished second"})
	me.Call("PATCH", "/items/"+done.ID, M{"status": "done"}, http.StatusOK, nil)
	me.S.SetNow(me.S.Service.Now().Add(60e9))
	me.Call("PATCH", "/items/"+done2.ID, M{"status": "done"}, http.StatusOK, nil)

	check := func(query string, want ...string) {
		t.Helper()
		var list itemList
		me.Call("GET", "/items"+query, nil, http.StatusOK, &list)
		if !equal(titles(list.Items), want) || list.Total != len(want) {
			t.Fatalf("GET /items%s = %v (total %d), want %v", query, titles(list.Items), list.Total, want)
		}
	}
	check("", "do", "do tomorrow", "plan", "plan no date", "quick", "later")
	check("?status=done", "finished second", "finished first")
	check("?quadrant=do", "do", "do tomorrow")
	check("?quadrant=plan", "plan", "plan no date")
	check("?quadrant=quick", "quick")
	check("?quadrant=later", "later")
	check("?project_id=none", "do", "do tomorrow", "plan", "quick")
	check("?project_id="+project.ID, "plan no date", "later")
	check("?project_id="+project.ID+"&quadrant=later", "later")
	check("?q=bank", "quick")
	check("?q=PLAN", "plan", "plan no date")
	check("?heading_id=none&status=done", "finished second", "finished first")
	wantCode(t, me, "GET", "/items?quadrant=never", nil, 400, "invalid_request")
	wantCode(t, me, "GET", "/items?status=maybe", nil, 400, "invalid_request")
	wantCode(t, me, "GET", "/items?limit=0", nil, 400, "invalid_request")

	var page itemList
	me.Call("GET", "/items?status=any&limit=3", nil, http.StatusOK, &page)
	if len(page.Items) != 3 || page.Total != 8 || page.NextCursor == nil {
		t.Fatalf("first page %+v", page)
	}
	seen := titles(page.Items)
	for page.NextCursor != nil {
		cursor := *page.NextCursor
		page = itemList{}
		me.Call("GET", "/items?status=any&limit=3&cursor="+cursor, nil, http.StatusOK, &page)
		seen = append(seen, titles(page.Items)...)
	}
	want := []string{"do", "do tomorrow", "plan", "plan no date", "quick", "later", "finished second", "finished first"}
	if !equal(seen, want) {
		t.Fatalf("paged through %v, want %v", seen, want)
	}
}

func TestProjectsAreasHeadings(t *testing.T) {
	_, me := setup(t)
	var area struct {
		Area api.Area `json:"area"`
	}
	me.Call("POST", "/areas", M{"name": "工作"}, http.StatusCreated, &area)
	first := mkProject(t, me, "A")
	second := mkProject(t, me, "B")
	if first.Color != "blue" || second.Color != "indigo" {
		t.Fatalf("default colours %s, %s", first.Color, second.Color)
	}
	wantCode(t, me, "POST", "/projects", M{"name": "C", "color": "beige"}, 400, "invalid_request")
	var patched struct {
		Project api.Project `json:"project"`
	}
	me.Call("PATCH", "/projects/"+first.ID, M{"area_id": area.Area.ID, "color": "teal", "archived": true}, http.StatusOK, &patched)
	if patched.Project.AreaID == nil || patched.Project.Color != "teal" || !patched.Project.Archived {
		t.Fatalf("patched %+v", patched.Project)
	}

	var heading struct {
		Heading api.Heading `json:"heading"`
	}
	me.Call("POST", "/projects/"+first.ID+"/headings", M{"name": "Sync"}, http.StatusCreated, &heading)
	item := mkItem(t, me, M{"title": "in heading", "heading_id": heading.Heading.ID})
	mkItem(t, me, M{"title": "done one", "project_id": first.ID})
	event := mkEvent(t, me, M{"title": "review", "project_id": first.ID, "start": "2026-10-13T05:00:00Z", "end": "2026-10-13T06:00:00Z"})
	mkEvent(t, me, M{"title": "already over", "project_id": first.ID, "start": "2026-10-12T05:00:00Z", "end": "2026-10-12T06:00:00Z"})
	me.Call("POST", "/items/"+item.ID+"/schedule", M{"start": "2026-10-13T07:00:00Z", "end": "2026-10-13T08:00:00Z"}, http.StatusOK, nil)

	var detail api.ProjectDetail
	me.Call("GET", "/projects/"+first.ID, nil, http.StatusOK, &detail)
	if detail.Project.OpenCount != 2 || len(detail.Headings) != 1 || detail.UnplannedCount != 1 ||
		len(detail.UpcomingEvents) != 1 || detail.UpcomingEvents[0].ID != event.ID {
		t.Fatalf("detail %+v", detail)
	}

	me.Call("DELETE", "/headings/"+heading.Heading.ID, nil, http.StatusNoContent, nil)
	if got := getItem(t, me, item.ID); got.HeadingID != nil || got.ProjectID == nil {
		t.Fatalf("item after deleting its heading: %+v", got)
	}
	me.Call("DELETE", "/areas/"+area.Area.ID, nil, http.StatusNoContent, nil)
	var boot api.Bootstrap
	me.Call("GET", "/bootstrap", nil, http.StatusOK, &boot)
	if len(boot.Areas) != 0 || len(boot.Projects) != 2 || boot.Projects[0].AreaID != nil || boot.User.Email == "" || boot.Revision == 0 {
		t.Fatalf("bootstrap %+v", boot)
	}

	// Deleting a project takes its items and events along, and undo brings them back.
	me.Call("DELETE", "/projects/"+first.ID, nil, http.StatusNoContent, nil)
	wantCode(t, me, "GET", "/items/"+item.ID, nil, 404, "not_found")
	wantCode(t, me, "GET", "/events/"+event.ID, nil, 404, "not_found")
	me.Call("POST", "/activity/"+activities(t, me)[0].ID+"/undo", nil, http.StatusOK, nil)
	if got := getItem(t, me, item.ID); got.Block == nil {
		t.Fatalf("restored item lost its block: %+v", got)
	}
	me.Call("GET", "/events/"+event.ID, nil, http.StatusOK, nil)
}

func TestScheduleAndBlock(t *testing.T) {
	_, me := setup(t)
	project := mkProject(t, me, "Work")
	item := mkItem(t, me, M{"title": "Write the report", "project_id": project.ID})

	var resp struct {
		Item api.Item `json:"item"`
	}
	// 16:30 UTC is 00:30 the next day in Asia/Shanghai.
	me.Call("POST", "/items/"+item.ID+"/schedule", M{"start": "2026-10-13T16:30:00Z", "end": "2026-10-13T17:30:00Z"}, http.StatusOK, &resp)
	block := resp.Item.Block
	if block == nil || block.Start != "2026-10-13T16:30:00Z" || block.End != "2026-10-13T17:30:00Z" {
		t.Fatalf("block %+v", block)
	}
	if resp.Item.PlannedDate == nil || *resp.Item.PlannedDate != "2026-10-14" {
		t.Fatalf("planned_date should follow the block in the user's time zone: %v", resp.Item.PlannedDate)
	}

	var ev struct {
		Event api.Event `json:"event"`
	}
	me.Call("GET", "/events/"+block.EventID, nil, http.StatusOK, &ev)
	if ev.Event.ItemID == nil || *ev.Event.ItemID != item.ID || ev.Event.Title != "Write the report" ||
		ev.Event.ItemDone == nil || *ev.Event.ItemDone || *ev.Event.ProjectID != project.ID {
		t.Fatalf("block event %+v", ev.Event)
	}

	// Scheduling again moves the same block.
	me.Call("POST", "/items/"+item.ID+"/schedule", M{"start": "2026-10-13T06:00:00Z", "end": "2026-10-13T07:00:00Z"}, http.StatusOK, &resp)
	if resp.Item.Block.EventID != block.EventID || *resp.Item.PlannedDate != day {
		t.Fatalf("rescheduled %+v", resp.Item)
	}
	if got := activities(t, me)[0]; got.Action != "item.schedule" || got.Summary != "「Write the report」从 明天 00:30 改到 今天 14:00" {
		t.Fatalf("activity %+v", got)
	}

	// Moving the block as an event moves the item's planned date; renaming the item renames the block.
	me.Call("PATCH", "/events/"+block.EventID, M{"start": "2026-10-15T06:00:00Z"}, http.StatusOK, &ev)
	if *ev.Event.End != "2026-10-15T07:00:00Z" {
		t.Fatalf("moving the start alone should keep the length: %+v", ev.Event)
	}
	if got := getItem(t, me, item.ID); *got.PlannedDate != "2026-10-15" {
		t.Fatalf("planned_date %v", *got.PlannedDate)
	}
	me.Call("PATCH", "/items/"+item.ID, M{"title": "Write the annual report", "status": "done"}, http.StatusOK, nil)
	me.Call("GET", "/events/"+block.EventID, nil, http.StatusOK, &ev)
	if ev.Event.Title != "Write the annual report" || !*ev.Event.ItemDone {
		t.Fatalf("block after item change %+v", ev.Event)
	}

	wantCode(t, me, "POST", "/items/"+item.ID+"/schedule", M{"start": "2026-10-13T06:00:00Z", "end": "2026-10-13T06:00:00Z"}, 400, "invalid_request")
	wantCode(t, me, "POST", "/items/"+item.ID+"/schedule", M{"start": "tomorrow"}, 400, "invalid_request")

	me.Call("DELETE", "/items/"+item.ID+"/schedule", nil, http.StatusOK, &resp)
	if resp.Item.Block != nil {
		t.Fatalf("block should be gone: %+v", resp.Item.Block)
	}
	wantCode(t, me, "GET", "/events/"+block.EventID, nil, 404, "not_found")
}

func TestBlockPrefersTheNextOne(t *testing.T) {
	s, me := setup(t)
	item := mkItem(t, me, M{"title": "x"})
	me.Call("POST", "/items/"+item.ID+"/schedule", M{"start": "2026-10-13T03:00:00Z", "end": "2026-10-13T04:00:00Z"}, http.StatusOK, nil)
	first := getItem(t, me, item.ID).Block
	// Once the block is over it is still reported, as the most recent one.
	s.SetNow(s.Service.Now().Add(5 * 3600e9))
	if got := getItem(t, me, item.ID).Block; got == nil || got.EventID != first.EventID {
		t.Fatalf("past block %+v", got)
	}
}

func TestEvents(t *testing.T) {
	_, me := setup(t)
	timed := mkEvent(t, me, M{"title": "设计评审", "start": "2026-10-13T02:00:00+00:00", "end": "2026-10-13T11:30:00+08:00", "location": "3F"})
	if timed.AllDay || *timed.Start != "2026-10-13T02:00:00Z" || *timed.End != "2026-10-13T03:30:00Z" || timed.StartDate != nil ||
		timed.Status != "confirmed" || timed.Readonly || timed.Recurring || timed.RRule != nil || timed.ItemDone != nil {
		t.Fatalf("timed event %+v", timed)
	}
	allDay := mkEvent(t, me, M{"title": "出差", "all_day": true, "start_date": "2026-10-13", "end_date": "2026-10-14"})
	if !allDay.AllDay || allDay.Start != nil || *allDay.StartDate != "2026-10-13" || *allDay.EndDate != "2026-10-14" {
		t.Fatalf("all-day event %+v", allDay)
	}
	wantCode(t, me, "POST", "/events", M{"title": "x", "start": "2026-10-13T03:00:00Z", "end": "2026-10-13T02:00:00Z"}, 400, "invalid_request")
	wantCode(t, me, "POST", "/events", M{"title": "x", "all_day": true, "start_date": "2026-10-13"}, 400, "invalid_request")
	wantCode(t, me, "POST", "/events", M{"title": "x"}, 400, "invalid_request")
	wantCode(t, me, "GET", "/events?from=2026-10-13T00:00:00Z&to=2028-10-13T00:00:00Z", nil, 400, "invalid_request")
	wantCode(t, me, "GET", "/events", nil, 400, "invalid_request")

	// The second local day sees only the all-day event; ranges are half-open.
	got := listEvents(t, me, endUTC, "2026-10-14T16:00:00Z")
	if len(got) != 1 || got[0].ID != allDay.ID {
		t.Fatalf("second day %+v", got)
	}
	if got := listEvents(t, me, "2026-10-13T03:30:00Z", "2026-10-13T05:00:00Z"); len(got) != 1 {
		t.Fatalf("an event that ended at the range start should not be listed: %+v", got)
	}
	got = listEvents(t, me, dayUTC, endUTC)
	if len(got) != 2 || got[0].ID != allDay.ID || got[1].ID != timed.ID {
		t.Fatalf("first day %+v", got)
	}

	var resp struct {
		Event api.Event `json:"event"`
	}
	me.Call("PATCH", "/events/"+timed.ID, M{"start": "2026-10-15T05:00:00Z", "end": "2026-10-15T06:00:00Z"}, http.StatusOK, &resp)
	if act := activities(t, me)[0]; act.Action != "event.update" || act.Summary != "「设计评审」从 今天 10:00 改到 周四 13:00" {
		t.Fatalf("activity %+v", act)
	}
	me.Call("PATCH", "/events/"+timed.ID, M{"all_day": true, "start_date": "2026-10-16", "end_date": "2026-10-16"}, http.StatusOK, &resp)
	if !resp.Event.AllDay || resp.Event.Start != nil {
		t.Fatalf("to all-day %+v", resp.Event)
	}
	me.Call("DELETE", "/events/"+timed.ID, nil, http.StatusNoContent, nil)
	wantCode(t, me, "GET", "/events/"+timed.ID, nil, 404, "not_found")
}
