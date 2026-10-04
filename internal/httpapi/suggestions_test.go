package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Lewin671/keduly/internal/api"
)

type decided struct {
	Suggestion api.Suggestion `json:"suggestion"`
	Activity   api.Activity   `json:"activity"`
}

func findEvent(events []api.Event, id string) *api.Event {
	for i := range events {
		if events[i].ID == id {
			return &events[i]
		}
	}
	return nil
}

func pending(t *testing.T, c interface {
	Call(string, string, any, int, any)
}) []api.Suggestion {
	t.Helper()
	var resp struct {
		Suggestions []api.Suggestion `json:"suggestions"`
	}
	c.Call("GET", "/suggestions", nil, http.StatusOK, &resp)
	return resp.Suggestions
}

func TestSuggestionOverlaysAndAccept(t *testing.T) {
	_, me := setup(t)
	agent := me.NewToken("Claude Code", "agent", "write", true)
	project := mkProject(t, me, "Work")
	item := mkItem(t, me, M{"title": "写周报", "project_id": project.ID})
	dentist := mkEvent(t, me, M{"title": "牙医", "start": "2026-10-13T02:30:00Z", "end": "2026-10-13T03:30:00Z"})
	oneOnOne := mkEvent(t, me, M{"title": "1:1", "start": "2026-10-13T05:00:00Z", "end": "2026-10-13T05:30:00Z"})

	schedule := suggest(t, agent, M{"kind": "schedule_item", "item_id": item.ID, "start": "2026-10-13T06:00:00Z", "end": "2026-10-13T07:00:00Z",
		"reason": "今天 18:00 截止，这是下午第一个 1 小时空档。"})
	if schedule.Status != "pending" || schedule.Title != "写周报" || schedule.Actor.Kind != "agent" || schedule.Actor.Name != "Claude Code" ||
		schedule.Item == nil || schedule.DecidedAt != nil {
		t.Fatalf("schedule suggestion %+v", schedule)
	}
	createItem := suggest(t, agent, M{"kind": "create_item", "item": M{"title": "准备材料", "project_id": project.ID, "estimate_minutes": 30},
		"start": "2026-10-13T01:30:00Z", "end": "2026-10-13T02:00:00Z", "reason": "评审 10:00 开始"})
	createEvent := suggest(t, agent, M{"kind": "create_event", "event": M{"title": "复盘", "start": "2026-10-13T08:00:00Z", "end": "2026-10-13T09:00:00Z"}, "reason": "r"})
	move := suggest(t, agent, M{"kind": "move_event", "event_id": dentist.ID, "start": "2026-10-16T02:00:00Z", "end": "2026-10-16T03:00:00Z", "reason": "和客户演示冲突"})
	remove := suggest(t, agent, M{"kind": "delete_event", "event_id": oneOnOne.ID, "reason": "王敏本周请假"})

	// Validation.
	wantCode(t, agent, "POST", "/suggestions", M{"kind": "schedule_item", "item_id": item.ID, "start": "2026-10-13T06:00:00Z", "end": "2026-10-13T07:00:00Z"}, 400, "invalid_request")
	wantCode(t, agent, "POST", "/suggestions", M{"kind": "schedule_item", "item_id": item.ID, "reason": "r"}, 400, "invalid_request")
	wantCode(t, agent, "POST", "/suggestions", M{"kind": "create_item", "item": M{"title": ""}, "reason": "r"}, 400, "invalid_request")
	wantCode(t, agent, "POST", "/suggestions", M{"kind": "create_event", "event": M{"title": "x"}, "reason": "r"}, 400, "invalid_request")
	wantCode(t, agent, "POST", "/suggestions", M{"kind": "move_event", "event_id": "aaaaaaaaaaaaaaaa", "start": "2026-10-16T02:00:00Z", "end": "2026-10-16T03:00:00Z", "reason": "r"}, 404, "not_found")
	wantCode(t, agent, "POST", "/suggestions", M{"kind": "teleport", "reason": "r"}, 400, "invalid_request")

	if got := pending(t, me); len(got) != 5 {
		t.Fatalf("%d pending suggestions", len(got))
	}
	if got := getItem(t, me, item.ID); got.Suggestion == nil || got.Suggestion.ID != schedule.ID || got.Block != nil {
		t.Fatalf("item suggestion %+v", got.Suggestion)
	}

	events := listEvents(t, me, dayUTC, endUTC)
	if len(events) != 5 {
		t.Fatalf("%d entries on the day, want 2 real and 3 tentative", len(events))
	}
	tentative := func(id string) *api.Event {
		t.Helper()
		e := findEvent(events, id)
		if e == nil || e.Status != "tentative" || e.SuggestionID == nil || *e.SuggestionID != id {
			t.Fatalf("no tentative entry for suggestion %s: %+v", id, e)
		}
		return e
	}
	if e := tentative(schedule.ID); e.ItemID == nil || *e.ItemID != item.ID || e.Title != "写周报" || *e.ProjectID != project.ID || *e.Start != "2026-10-13T06:00:00Z" {
		t.Fatalf("tentative block %+v", e)
	}
	if e := tentative(createItem.ID); e.Title != "准备材料" || e.ItemID != nil || *e.ProjectID != project.ID {
		t.Fatalf("tentative new item %+v", e)
	}
	if e := tentative(createEvent.ID); e.Title != "复盘" {
		t.Fatalf("tentative new event %+v", e)
	}
	for _, id := range []string{dentist.ID, oneOnOne.ID} {
		if e := findEvent(events, id); e == nil || e.Status != "leaving" || e.SuggestionID == nil {
			t.Fatalf("event %s should be leaving: %+v", id, e)
		}
	}
	// The new position of the moved event is tentative on its own day.
	later := listEvents(t, me, "2026-10-15T16:00:00Z", "2026-10-16T16:00:00Z")
	if len(later) != 1 || later[0].Status != "tentative" || later[0].Title != "牙医" || *later[0].SuggestionID != move.ID {
		t.Fatalf("moved position %+v", later)
	}

	// An agent cannot decide; the user can.
	wantCode(t, agent, "POST", "/suggestions/"+schedule.ID+"/accept", nil, 403, "forbidden")
	wantCode(t, agent, "POST", "/suggestions/"+schedule.ID+"/reject", nil, 403, "forbidden")
	wantCode(t, agent, "POST", "/suggestions/accept-all", nil, 403, "forbidden")

	var got decided
	me.Call("POST", "/suggestions/"+schedule.ID+"/accept", nil, http.StatusOK, &got)
	if got.Suggestion.Status != "accepted" || got.Suggestion.DecidedAt == nil {
		t.Fatalf("accepted %+v", got.Suggestion)
	}
	if a := got.Activity; a.Action != "item.schedule" || a.Actor.Name != "Claude Code" || a.Reason == nil ||
		*a.Reason != "今天 18:00 截止，这是下午第一个 1 小时空档。" || a.Summary != "「写周报」排到 今天 14:00" || !a.Undoable {
		t.Fatalf("activity %+v reason %q", a, *a.Reason)
	}
	if it := getItem(t, me, item.ID); it.Block == nil || it.Block.Start != "2026-10-13T06:00:00Z" || it.Suggestion != nil || *it.PlannedDate != day {
		t.Fatalf("item after accept %+v", it)
	}
	wantCode(t, me, "POST", "/suggestions/"+schedule.ID+"/accept", nil, 409, "conflict")
	wantCode(t, me, "POST", "/suggestions/"+schedule.ID+"/reject", nil, 409, "conflict")

	me.Call("POST", "/suggestions/"+createEvent.ID+"/reject", nil, http.StatusOK, &got)
	if got.Suggestion.Status != "rejected" {
		t.Fatalf("rejected %+v", got.Suggestion)
	}

	var all struct {
		Accepted    int      `json:"accepted"`
		ActivityIDs []string `json:"activity_ids"`
	}
	me.Call("POST", "/suggestions/accept-all", nil, http.StatusOK, &all)
	if all.Accepted != 3 || len(all.ActivityIDs) != 3 {
		t.Fatalf("accept-all %+v", all)
	}
	if len(pending(t, me)) != 0 {
		t.Fatal("suggestions still pending")
	}
	wantCode(t, me, "GET", "/events/"+oneOnOne.ID, nil, 404, "not_found")
	var ev struct {
		Event api.Event `json:"event"`
	}
	me.Call("GET", "/events/"+dentist.ID, nil, http.StatusOK, &ev)
	if *ev.Event.Start != "2026-10-16T02:00:00Z" {
		t.Fatalf("moved event %+v", ev.Event)
	}
	var list itemList
	me.Call("GET", "/items?q=准备材料", nil, http.StatusOK, &list)
	if len(list.Items) != 1 || list.Items[0].Block == nil || list.Items[0].CreatedBy.Name != "Claude Code" || *list.Items[0].EstimateMinutes != 30 {
		t.Fatalf("created item %+v", list.Items)
	}
	for _, e := range listEvents(t, me, dayUTC, "2026-10-17T16:00:00Z") {
		if e.Status != "confirmed" {
			t.Fatalf("entry still %s: %+v", e.Status, e)
		}
	}

	// Undoing everything accept-all did, in one call.
	var undone struct {
		Activities []api.Activity `json:"activities"`
	}
	me.Call("POST", "/activity/undo", M{"ids": all.ActivityIDs}, http.StatusOK, &undone)
	if len(undone.Activities) != 3 {
		t.Fatalf("undone %+v", undone)
	}
	me.Call("GET", "/events/"+oneOnOne.ID, nil, http.StatusOK, nil)
	me.Call("GET", "/events/"+dentist.ID, nil, http.StatusOK, &ev)
	if *ev.Event.Start != "2026-10-13T02:30:00Z" {
		t.Fatalf("undo did not move the event back: %+v", ev.Event)
	}
	me.Call("GET", "/items?q=准备材料", nil, http.StatusOK, &list)
	if len(list.Items) != 0 {
		t.Fatal("undo did not remove the created item")
	}
	_ = remove
}

func TestDeleteBecomesSuggestion(t *testing.T) {
	_, me := setup(t)
	careful := me.NewToken("Careful", "agent", "write", true)
	trusted := me.NewToken("Trusted", "agent", "write", false)
	item := mkItem(t, me, M{"title": "keep me"})
	event := mkEvent(t, me, M{"title": "1:1 与王敏", "start": "2026-10-13T05:00:00Z", "end": "2026-10-13T05:30:00Z"})

	var resp struct {
		Suggestion api.Suggestion `json:"suggestion"`
	}
	careful.Call("DELETE", "/items/"+item.ID, M{"reason": "duplicate"}, http.StatusAccepted, &resp)
	if resp.Suggestion.Kind != "delete_item" || resp.Suggestion.Status != "pending" || *resp.Suggestion.ItemID != item.ID || resp.Suggestion.Reason != "duplicate" {
		t.Fatalf("delete suggestion %+v", resp.Suggestion)
	}
	getItem(t, me, item.ID)
	careful.Call("DELETE", "/events/"+event.ID, nil, http.StatusAccepted, &resp)
	if resp.Suggestion.Kind != "delete_event" || resp.Suggestion.Title != "1:1 与王敏" {
		t.Fatalf("delete suggestion %+v", resp.Suggestion)
	}
	if e := findEvent(listEvents(t, me, dayUTC, endUTC), event.ID); e == nil || e.Status != "leaving" {
		t.Fatalf("event should be leaving: %+v", e)
	}

	var got decided
	me.Call("POST", "/suggestions/"+resp.Suggestion.ID+"/accept", nil, http.StatusOK, &got)
	if got.Activity.Action != "event.delete" || got.Activity.Actor.Name != "Careful" {
		t.Fatalf("activity %+v", got.Activity)
	}
	wantCode(t, me, "GET", "/events/"+event.ID, nil, 404, "not_found")

	// Without confirm_delete a token deletes directly, and so does the user.
	trusted.Call("DELETE", "/items/"+item.ID, nil, http.StatusNoContent, nil)
	wantCode(t, me, "GET", "/items/"+item.ID, nil, 404, "not_found")

	// The suggestion to delete the item can no longer be applied.
	first := pending(t, me)
	if len(first) != 1 || first[0].Kind != "delete_item" || string(first[0].Item) != "null" {
		t.Fatalf("pending %+v", first)
	}
	wantCode(t, me, "POST", "/suggestions/"+first[0].ID+"/accept", nil, 409, "conflict")
	var any struct {
		Suggestions []api.Suggestion `json:"suggestions"`
	}
	me.Call("GET", "/suggestions?status=rejected", nil, http.StatusOK, &any)
	if len(any.Suggestions) != 1 || any.Suggestions[0].ID != first[0].ID {
		t.Fatalf("a suggestion that cannot be applied should end up rejected: %+v", any.Suggestions)
	}
	me.Call("GET", "/suggestions?status=any", nil, http.StatusOK, &any)
	if len(any.Suggestions) != 2 {
		t.Fatalf("all suggestions %+v", any.Suggestions)
	}
}

func TestAcceptAllSkipsWhatCannotBeApplied(t *testing.T) {
	_, me := setup(t)
	agent := me.NewToken("Agent", "agent", "write", false)
	gone := mkItem(t, me, M{"title": "gone"})
	kept := mkItem(t, me, M{"title": "kept"})
	slot := M{"start": "2026-10-13T06:00:00Z", "end": "2026-10-13T07:00:00Z", "reason": "r"}
	for _, id := range []string{gone.ID, kept.ID} {
		body := M{"kind": "schedule_item", "item_id": id}
		for k, v := range slot {
			body[k] = v
		}
		suggest(t, agent, body)
	}
	me.Call("DELETE", "/items/"+gone.ID, nil, http.StatusNoContent, nil)
	var all struct {
		Accepted    int      `json:"accepted"`
		ActivityIDs []string `json:"activity_ids"`
	}
	me.Call("POST", "/suggestions/accept-all", nil, http.StatusOK, &all)
	if all.Accepted != 1 || len(all.ActivityIDs) != 1 {
		t.Fatalf("accept-all %+v", all)
	}
	if getItem(t, me, kept.ID).Block == nil {
		t.Fatal("the applicable suggestion was not applied")
	}
	var rejected struct {
		Suggestions []api.Suggestion `json:"suggestions"`
	}
	me.Call("GET", "/suggestions?status=rejected", nil, http.StatusOK, &rejected)
	if len(rejected.Suggestions) != 1 {
		t.Fatalf("rejected %+v", rejected.Suggestions)
	}
}

func TestSuggestionShapeIsStable(t *testing.T) {
	_, me := setup(t)
	item := mkItem(t, me, M{"title": "x"})
	_, data, _ := me.Do("POST", "/suggestions", M{"kind": "delete_item", "item_id": item.ID, "reason": "r"})
	var raw struct {
		Suggestion map[string]json.RawMessage `json:"suggestion"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"id", "status", "kind", "actor", "reason", "title", "item_id", "event_id", "start", "end", "item", "event", "created_at", "decided_at"} {
		if _, ok := raw.Suggestion[field]; !ok {
			t.Fatalf("suggestion lacks %q: %s", field, data)
		}
	}
	if string(raw.Suggestion["event"]) != "null" || string(raw.Suggestion["start"]) != "null" {
		t.Fatalf("absent values must be null: %s", data)
	}
}
