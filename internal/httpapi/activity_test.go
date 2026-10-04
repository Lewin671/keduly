package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/Lewin671/keduly/internal/api"
)

func undo(t *testing.T, c interface {
	Call(string, string, any, int, any)
}, id string) api.Activity {
	t.Helper()
	var resp struct {
		Activity api.Activity `json:"activity"`
	}
	c.Call("POST", "/activity/"+id+"/undo", nil, http.StatusOK, &resp)
	return resp.Activity
}

func redo(t *testing.T, c interface {
	Call(string, string, any, int, any)
}, id string) api.Activity {
	t.Helper()
	var resp struct {
		Activity api.Activity `json:"activity"`
	}
	c.Call("POST", "/activity/"+id+"/redo", nil, http.StatusOK, &resp)
	return resp.Activity
}

func TestActivityLog(t *testing.T) {
	_, me := setup(t)
	agent := me.NewToken("Codex", "agent", "write", false)
	item := mkItem(t, agent, M{"title": "调研 FullCalendar 授权", "reason": "follow-up from the review"})
	me.Call("PATCH", "/items/"+item.ID, M{"important": true}, http.StatusOK, nil)
	me.Call("PATCH", "/items/"+item.ID, M{"status": "done"}, http.StatusOK, nil)
	agent.Call("DELETE", "/items/"+item.ID, nil, http.StatusNoContent, nil)

	log := activities(t, me)
	want := []struct{ action, summary, actor string }{
		{"item.delete", "删除事项「调研 FullCalendar 授权」", "agent"},
		{"item.complete", "完成了「调研 FullCalendar 授权」", "user"},
		{"item.update", "将「调研 FullCalendar 授权」标为重要", "user"},
		{"item.create", "新建事项「调研 FullCalendar 授权」", "agent"},
	}
	if len(log) != len(want) {
		t.Fatalf("%d entries: %+v", len(log), log)
	}
	for i, w := range want {
		if log[i].Action != w.action || log[i].Summary != w.summary || log[i].Actor.Kind != w.actor || !log[i].Undoable || log[i].Undone {
			t.Fatalf("entry %d = %+v, want %+v", i, log[i], w)
		}
	}
	if log[3].Reason == nil || *log[3].Reason != "follow-up from the review" || log[3].Actor.Name != "Codex" || log[1].Reason != nil {
		t.Fatalf("reason and actor: %+v", log[3])
	}

	// The reason may also travel in a header.
	req, _ := http.NewRequest("DELETE", me.S.URL+"/api/v1/items/"+mkItem(t, agent, M{"title": "x"}).ID, nil)
	req.Header.Set("Authorization", "Bearer "+agent.Token)
	req.Header.Set("X-Keduly-Reason", "not needed")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete with a reason header: %v %v", err, resp.StatusCode)
	}
	resp.Body.Close()
	if got := activities(t, me)[0]; got.Reason == nil || *got.Reason != "not needed" {
		t.Fatalf("header reason %+v", got)
	}

	var page struct {
		Activities []api.Activity `json:"activities"`
		NextCursor *string        `json:"next_cursor"`
	}
	me.Call("GET", "/activity?limit=4", nil, http.StatusOK, &page)
	if len(page.Activities) != 4 || page.NextCursor == nil {
		t.Fatalf("first page %+v", page)
	}
	me.Call("GET", "/activity?limit=4&cursor="+*page.NextCursor, nil, http.StatusOK, &page)
	if len(page.Activities) != 2 || page.NextCursor != nil || page.Activities[1].Action != "item.create" {
		t.Fatalf("second page %+v", page)
	}
}

func TestUndoRedo(t *testing.T) {
	_, me := setup(t)
	item := mkItem(t, me, M{"title": "draft", "due_date": "2026-10-20"})
	created := activities(t, me)[0]

	// Update.
	me.Call("PATCH", "/items/"+item.ID, M{"title": "final", "important": true}, http.StatusOK, nil)
	updated := activities(t, me)[0]
	before := len(activities(t, me))
	if got := undo(t, me, updated.ID); !got.Undone {
		t.Fatalf("undo result %+v", got)
	}
	if got := getItem(t, me, item.ID); got.Title != "draft" || got.Important {
		t.Fatalf("after undo %+v", got)
	}
	if len(activities(t, me)) != before {
		t.Fatal("undo must not add a log entry")
	}
	wantCode(t, me, "POST", "/activity/"+updated.ID+"/undo", nil, 409, "conflict")
	if got := redo(t, me, updated.ID); got.Undone {
		t.Fatalf("redo result %+v", got)
	}
	if got := getItem(t, me, item.ID); got.Title != "final" || !got.Important {
		t.Fatalf("after redo %+v", got)
	}
	wantCode(t, me, "POST", "/activity/"+updated.ID+"/redo", nil, 409, "conflict")

	// Completion.
	me.Call("PATCH", "/items/"+item.ID, M{"status": "done"}, http.StatusOK, nil)
	completed := activities(t, me)[0]
	undo(t, me, completed.ID)
	if got := getItem(t, me, item.ID); got.Status != "open" || got.CompletedAt != nil {
		t.Fatalf("after undoing completion %+v", got)
	}
	redo(t, me, completed.ID)
	if got := getItem(t, me, item.ID); got.Status != "done" || got.CompletedAt == nil {
		t.Fatalf("after redoing completion %+v", got)
	}

	// An older change cannot be undone once the item has changed again.
	wantCode(t, me, "POST", "/activity/"+updated.ID+"/undo", nil, 409, "conflict")
	wantCode(t, me, "POST", "/activity/"+created.ID+"/undo", nil, 409, "conflict")

	// Delete, with the item's time block.
	me.Call("POST", "/items/"+item.ID+"/schedule", M{"start": "2026-10-13T06:00:00Z", "end": "2026-10-13T07:00:00Z"}, http.StatusOK, nil)
	blockID := getItem(t, me, item.ID).Block.EventID
	me.Call("DELETE", "/items/"+item.ID, nil, http.StatusNoContent, nil)
	deleted := activities(t, me)[0]
	wantCode(t, me, "GET", "/events/"+blockID, nil, 404, "not_found")
	undo(t, me, deleted.ID)
	if got := getItem(t, me, item.ID); got.Title != "final" || got.Block == nil || got.Block.EventID != blockID {
		t.Fatalf("after undoing delete %+v", got)
	}
	redo(t, me, deleted.ID)
	wantCode(t, me, "GET", "/items/"+item.ID, nil, 404, "not_found")
	wantCode(t, me, "GET", "/events/"+blockID, nil, 404, "not_found")

	// Create.
	other := mkItem(t, me, M{"title": "oops"})
	createdOther := activities(t, me)[0]
	undo(t, me, createdOther.ID)
	wantCode(t, me, "GET", "/items/"+other.ID, nil, 404, "not_found")
	redo(t, me, createdOther.ID)
	if got := getItem(t, me, other.ID); got.Title != "oops" {
		t.Fatalf("after redoing create %+v", got)
	}

	// Events.
	event := mkEvent(t, me, M{"title": "健身", "start": "2026-10-13T10:00:00Z", "end": "2026-10-13T11:00:00Z"})
	me.Call("PATCH", "/events/"+event.ID, M{"start": "2026-10-13T10:30:00Z"}, http.StatusOK, nil)
	moved := activities(t, me)[0]
	if moved.Summary != "「健身」从 18:00 改到 18:30" {
		t.Fatalf("summary %q", moved.Summary)
	}
	undo(t, me, moved.ID)
	var ev struct {
		Event api.Event `json:"event"`
	}
	me.Call("GET", "/events/"+event.ID, nil, http.StatusOK, &ev)
	if *ev.Event.Start != "2026-10-13T10:00:00Z" || *ev.Event.End != "2026-10-13T11:00:00Z" {
		t.Fatalf("after undoing the move %+v", ev.Event)
	}

	wantCode(t, me, "POST", "/activity/aaaaaaaaaaaaaaaa/undo", nil, 404, "not_found")
	wantCode(t, me, "POST", "/activity/undo", M{"ids": []string{}}, 400, "invalid_request")
}

func TestUndoAcceptedSuggestion(t *testing.T) {
	_, me := setup(t)
	agent := me.NewToken("Claude Code", "agent", "write", true)
	sug := suggest(t, agent, M{"kind": "create_item", "item": M{"title": "准备评审材料"},
		"start": "2026-10-13T01:30:00Z", "end": "2026-10-13T02:00:00Z", "reason": "评审 10:00 开始，材料还没准备。"})
	var got decided
	me.Call("POST", "/suggestions/"+sug.ID+"/accept", nil, http.StatusOK, &got)
	if got.Activity.Summary != "新建事项「准备评审材料」，排到 今天 09:30" || got.Activity.Action != "item.create" {
		t.Fatalf("activity %+v", got.Activity)
	}
	if n := len(listEvents(t, me, dayUTC, endUTC)); n != 1 {
		t.Fatalf("%d events after accept", n)
	}
	// A write token may undo, too.
	undo(t, agent, got.Activity.ID)
	var list itemList
	me.Call("GET", "/items?status=any", nil, http.StatusOK, &list)
	if list.Total != 0 || len(listEvents(t, me, dayUTC, endUTC)) != 0 {
		t.Fatalf("undo left %d items behind", list.Total)
	}
	redo(t, me, got.Activity.ID)
	me.Call("GET", "/items?status=any", nil, http.StatusOK, &list)
	if list.Total != 1 || list.Items[0].Block == nil {
		t.Fatalf("redo restored %+v", list.Items)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	_, me := setup(t)
	item := mkItem(t, me, M{"title": "real"})
	event := mkEvent(t, me, M{"title": "real event", "start": "2026-10-13T05:00:00Z", "end": "2026-10-13T06:00:00Z"})
	careful := me.NewToken("Careful", "agent", "write", true)
	start := revision(t, me)
	logged := len(activities(t, me))

	var created struct {
		Item   api.Item `json:"item"`
		DryRun bool     `json:"dry_run"`
	}
	me.Call("POST", "/items?dry_run=1", M{"title": "imagined"}, http.StatusCreated, &created)
	if !created.DryRun || created.Item.Title != "imagined" || created.Item.ID == "" {
		t.Fatalf("dry-run create %+v", created)
	}
	wantCode(t, me, "GET", "/items/"+created.Item.ID, nil, 404, "not_found")
	wantCode(t, me, "POST", "/items?dry_run=1", M{"title": ""}, 400, "invalid_request")

	me.Call("PATCH", "/items/"+item.ID+"?dry_run=1", M{"title": "changed"}, http.StatusOK, &created)
	if !created.DryRun || created.Item.Title != "changed" {
		t.Fatalf("dry-run patch %+v", created)
	}
	me.Call("POST", "/items/"+item.ID+"/schedule?dry_run=1", M{"start": "2026-10-13T06:00:00Z", "end": "2026-10-13T07:00:00Z"}, http.StatusOK, &created)
	if !created.DryRun || created.Item.Block == nil {
		t.Fatalf("dry-run schedule %+v", created)
	}

	var dry struct {
		DryRun     bool            `json:"dry_run"`
		Suggestion *api.Suggestion `json:"suggestion"`
		Event      *api.Event      `json:"event"`
	}
	me.Call("DELETE", "/items/"+item.ID+"?dry_run=1", nil, http.StatusOK, &dry)
	if !dry.DryRun {
		t.Fatalf("dry-run delete %+v", dry)
	}
	me.Call("PATCH", "/events/"+event.ID+"?dry_run=1", M{"title": "moved"}, http.StatusOK, &dry)
	me.Call("DELETE", "/events/"+event.ID+"?dry_run=1", nil, http.StatusOK, &dry)
	me.Call("POST", "/events?dry_run=1", M{"title": "imagined", "start": "2026-10-13T08:00:00Z", "end": "2026-10-13T09:00:00Z"}, http.StatusCreated, &dry)
	if !dry.DryRun || dry.Event == nil {
		t.Fatalf("dry-run event %+v", dry)
	}
	me.Call("POST", "/suggestions?dry_run=1", M{"kind": "delete_event", "event_id": event.ID, "reason": "r"}, http.StatusCreated, &dry)
	if !dry.DryRun || dry.Suggestion == nil {
		t.Fatalf("dry-run suggestion %+v", dry)
	}
	careful.Call("DELETE", "/items/"+item.ID+"?dry_run=1", nil, http.StatusAccepted, &dry)
	if !dry.DryRun || dry.Suggestion == nil {
		t.Fatalf("dry-run delete by a careful token %+v", dry)
	}

	if got := getItem(t, me, item.ID); got.Title != "real" || got.Block != nil {
		t.Fatalf("a dry run changed the item: %+v", got)
	}
	var list itemList
	me.Call("GET", "/items?status=any", nil, http.StatusOK, &list)
	if list.Total != 1 || len(listEvents(t, me, dayUTC, endUTC)) != 1 || len(pending(t, me)) != 0 {
		t.Fatal("a dry run wrote something")
	}
	if revision(t, me) != start || len(activities(t, me)) != logged {
		t.Fatal("a dry run bumped the revision or logged activity")
	}
}

func TestRevisionBumpsOnEveryWrite(t *testing.T) {
	_, me := setup(t)
	agent := me.NewToken("Agent", "agent", "write", true)
	rev := revision(t, me)
	step := func(what string) {
		t.Helper()
		next := revision(t, me)
		if next <= rev {
			t.Fatalf("revision did not grow after %s: %d -> %d", what, rev, next)
		}
		rev = next
	}
	item := mkItem(t, me, M{"title": "x"})
	step("creating an item")
	me.Call("PATCH", "/items/"+item.ID, M{"title": "y"}, http.StatusOK, nil)
	step("patching an item")
	sug := suggest(t, agent, M{"kind": "delete_item", "item_id": item.ID, "reason": "r"})
	step("a suggestion")
	me.Call("POST", "/suggestions/"+sug.ID+"/reject", nil, http.StatusOK, nil)
	step("rejecting a suggestion")
	undo(t, me, activities(t, me)[0].ID)
	step("an undo")
	me.Call("GET", "/items", nil, http.StatusOK, nil)
	if revision(t, me) != rev {
		t.Fatal("a read bumped the revision")
	}
}
