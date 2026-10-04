package core

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/store"
)

// change is the before and after state of one row touched by a write.
// Before is null for a created row, After is null for a deleted one.
type change struct {
	Kind   string          `json:"kind"`
	ID     string          `json:"id"`
	Before json.RawMessage `json:"before"`
	After  json.RawMessage `json:"after"`
}

const (
	kindArea    = "area"
	kindProject = "project"
	kindHeading = "heading"
	kindItem    = "item"
	kindEvent   = "event"
)

func snapshot(row any) json.RawMessage {
	if row == nil || reflect.ValueOf(row).IsNil() {
		return nil
	}
	raw, err := json.Marshal(row)
	if err != nil {
		panic(err)
	}
	return raw
}

// record notes a row change, folding repeated changes to the same row into one.
func (op *Op) record(kind, id string, before, after json.RawMessage) {
	for i := range op.changes {
		c := &op.changes[i]
		if c.Kind == kind && c.ID == id {
			c.After = after
			if c.Before == nil && c.After == nil {
				op.changes = append(op.changes[:i], op.changes[i+1:]...)
			}
			return
		}
	}
	op.changes = append(op.changes, change{Kind: kind, ID: id, Before: before, After: after})
}

func save[T any](op *Op, kind string, t store.Table[T], row *T) error {
	t.SetOwner(row, op.User.ID)
	id := t.ID(row)
	before, err := t.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return err
	}
	if err := t.Put(op.ctx, op.q, row); err != nil {
		return err
	}
	op.record(kind, id, snapshot(before), snapshot(row))
	return nil
}

func remove[T any](op *Op, kind string, t store.Table[T], id string) error {
	before, err := t.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil || before == nil {
		return err
	}
	if err := t.Delete(op.ctx, op.q, op.User.ID, id); err != nil {
		return err
	}
	op.record(kind, id, snapshot(before), nil)
	return nil
}

// log writes the activity entry for everything recorded so far in this Op.
func (op *Op) log(action, summary string) (*store.Activity, error) {
	changes, err := json.Marshal(op.changes)
	if err != nil {
		return nil, err
	}
	a := &store.Activity{
		ID: NewID(), UserID: op.User.ID,
		ActorKind: op.ID.Actor.Kind, ActorName: op.ID.Actor.Name,
		Action: action, Summary: summary,
		Undoable: len(op.changes) > 0, Changes: string(changes), CreatedAt: op.now(),
	}
	if op.Reason != "" {
		a.Reason = &op.Reason
	}
	op.changes = nil
	return a, store.Activities.Put(op.ctx, op.q, a)
}

func activityJSON(a *store.Activity) api.Activity {
	return api.Activity{
		ID: a.ID, Actor: api.Actor{Kind: a.ActorKind, Name: a.ActorName},
		Action: a.Action, Summary: a.Summary, Reason: a.Reason,
		Undoable: a.Undoable, Undone: a.Undone, CreatedAt: a.CreatedAt,
	}
}

// Page is the pagination input shared by list endpoints.
type Page struct {
	Limit  int
	Offset int
}

// ParsePage validates limit and cursor query values.
func ParsePage(limit, cursor string, def int) (Page, error) {
	p := Page{Limit: def}
	if limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil || n < 1 {
			return p, Invalid("limit must be a positive integer")
		}
		p.Limit = min(n, 200)
	}
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		n, err2 := strconv.Atoi(string(raw))
		if err != nil || err2 != nil || n < 0 {
			return p, Invalid("cursor is not valid")
		}
		p.Offset = n
	}
	return p, nil
}

// next returns the cursor for the following page, or nil on the last one.
func (p Page) next(total int) *string {
	if p.Offset+p.Limit >= total {
		return nil
	}
	c := base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(p.Offset + p.Limit)))
	return &c
}

func (op *Op) ListActivity(p Page) ([]api.Activity, *string, error) {
	total, err := store.Activities.Count(op.ctx, op.q, op.User.ID, "")
	if err != nil {
		return nil, nil, err
	}
	rows, err := store.Activities.Query(op.ctx, op.q,
		store.Activities.SelectSQL()+" WHERE user_id = ? ORDER BY seq DESC LIMIT ? OFFSET ?",
		op.User.ID, p.Limit, p.Offset)
	if err != nil {
		return nil, nil, err
	}
	out := make([]api.Activity, 0, len(rows))
	for _, a := range rows {
		out = append(out, activityJSON(a))
	}
	return out, p.next(total), nil
}

// Undo restores the rows of an activity entry to their state before the change.
func (op *Op) Undo(id string) (*api.Activity, error) { return op.flip(id, true) }

// Redo applies an undone change again.
func (op *Op) Redo(id string) (*api.Activity, error) { return op.flip(id, false) }

// UndoMany undoes several entries, newest first, all or nothing.
func (op *Op) UndoMany(ids []string) ([]api.Activity, error) {
	if len(ids) == 0 || len(ids) > 200 {
		return nil, Invalid("ids must list between 1 and 200 activity IDs")
	}
	marks := make([]string, len(ids))
	args := []any{op.User.ID}
	for i, id := range ids {
		marks[i] = "?"
		args = append(args, id)
	}
	rows, err := op.q.QueryContext(op.ctx, "SELECT id FROM activities WHERE user_id = ? AND id IN ("+
		strings.Join(marks, ",")+") ORDER BY seq DESC", args...)
	if err != nil {
		return nil, err
	}
	var ordered []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ordered = append(ordered, id)
	}
	rows.Close()
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if len(ordered) != len(seen) {
		return nil, NotFound("activity")
	}
	out := []api.Activity{}
	for _, id := range ordered {
		done, err := op.flip(id, true)
		if err != nil {
			return nil, err
		}
		out = append(out, *done)
	}
	return out, nil
}

func (op *Op) flip(id string, undo bool) (*api.Activity, error) {
	a, err := store.Activities.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, NotFound("activity")
	}
	switch {
	case !a.Undoable:
		return nil, Conflict("this change cannot be undone")
	case undo && a.Undone:
		return nil, Conflict("this change is already undone")
	case !undo && !a.Undone:
		return nil, Conflict("this change is not undone")
	}
	var changes []change
	if err := json.Unmarshal([]byte(a.Changes), &changes); err != nil {
		return nil, err
	}
	for i := range changes {
		if isNull(changes[i].Before) {
			changes[i].Before = nil
		}
		if isNull(changes[i].After) {
			changes[i].After = nil
		}
	}
	if undo {
		for i := len(changes) - 1; i >= 0; i-- {
			c := changes[i]
			if err := op.restore(c.Kind, c.ID, c.After, c.Before); err != nil {
				return nil, err
			}
		}
	} else {
		for _, c := range changes {
			if err := op.restore(c.Kind, c.ID, c.Before, c.After); err != nil {
				return nil, err
			}
		}
	}
	if err := store.SetActivityUndone(op.ctx, op.q, op.User.ID, id, undo); err != nil {
		return nil, err
	}
	a.Undone = undo
	out := activityJSON(a)
	return &out, nil
}

// restore moves one row from the expected state to the target state, refusing
// when the row has changed since.
func (op *Op) restore(kind, id string, expected, target json.RawMessage) error {
	switch kind {
	case kindArea:
		return restoreRow(op, store.Areas, id, expected, target, nil)
	case kindProject:
		return restoreRow(op, store.Projects, id, expected, target, []string{"headings", "items", "events"})
	case kindHeading:
		return restoreRow(op, store.Headings, id, expected, target, nil)
	case kindItem:
		return restoreRow(op, store.Items, id, expected, target, []string{"events"})
	case kindEvent:
		return op.restoreEvent(id, expected, target)
	}
	return Conflict("this change cannot be undone")
}

func restoreRow[T any](op *Op, t store.Table[T], id string, expected, target json.RawMessage, dependents []string) error {
	current, err := t.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return err
	}
	if !sameState(expected, snapshot(current)) {
		return Conflict("the affected data has changed since; undo it by hand")
	}
	if target == nil {
		// Deleting a row that other rows now hang off would silently take them along.
		for _, table := range dependents {
			var n int
			query := "SELECT count(*) FROM " + table + " WHERE user_id = ? AND " + t.Name[:len(t.Name)-1] + "_id = ?"
			if err := op.q.QueryRowContext(op.ctx, query, op.User.ID, id).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return Conflict("the affected data has changed since; undo it by hand")
			}
		}
		return t.Delete(op.ctx, op.q, op.User.ID, id)
	}
	row := new(T)
	if err := json.Unmarshal(target, row); err != nil {
		return err
	}
	t.SetOwner(row, op.User.ID)
	return t.Put(op.ctx, op.q, row)
}

func (op *Op) restoreEvent(id string, expected, target json.RawMessage) error {
	current, err := store.Events.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return err
	}
	if current != nil {
		if err := store.BumpCTag(op.ctx, op.q, op.User.ID, calendarKey(current.ProjectID)); err != nil {
			return err
		}
	}
	if err := restoreRow(op, store.Events, id, expected, target, nil); err != nil {
		return err
	}
	if target == nil {
		return nil
	}
	restored, err := store.Events.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return err
	}
	// CalDAV clients must see the restored object as changed.
	restored.ETag = NewID()
	if err := store.Events.Put(op.ctx, op.q, restored); err != nil {
		return err
	}
	return store.BumpCTag(op.ctx, op.q, op.User.ID, calendarKey(restored.ProjectID))
}

// sameState compares two snapshots, ignoring fields that change on every save.
func sameState(a, b json.RawMessage) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	var ma, mb map[string]any
	if json.Unmarshal(a, &ma) != nil || json.Unmarshal(b, &mb) != nil {
		return false
	}
	for _, volatile := range []string{"updated_at", "etag", "ical"} {
		delete(ma, volatile)
		delete(mb, volatile)
	}
	return reflect.DeepEqual(ma, mb)
}
