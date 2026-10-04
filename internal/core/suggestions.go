package core

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/store"
)

// suggest stores a pending suggestion made by the caller.
func (op *Op) suggest(s *store.Suggestion) (*api.Suggestion, error) {
	s.ID, s.UserID, s.Status = NewID(), op.User.ID, "pending"
	s.ActorKind, s.ActorName = op.ID.Actor.Kind, op.ID.Actor.Name
	s.Reason, s.CreatedAt = op.Reason, op.now()
	if t := op.ID.Token; t != nil {
		s.TokenID = &t.ID
	}
	if err := store.Suggestions.Put(op.ctx, op.q, s); err != nil {
		return nil, err
	}
	op.deco = nil
	return op.renderSuggestion(s)
}

func (op *Op) renderSuggestion(s *store.Suggestion) (*api.Suggestion, error) {
	out := &api.Suggestion{
		ID: s.ID, Status: s.Status, Kind: s.Kind,
		Actor:  api.Actor{Kind: s.ActorKind, Name: s.ActorName},
		Reason: s.Reason, Title: s.Title, ItemID: s.ItemID, EventID: s.EventID,
		Start: s.StartAt, End: s.EndAt, CreatedAt: s.CreatedAt, DecidedAt: s.DecidedAt,
	}
	switch {
	case s.Kind == "create_item":
		out.Item = json.RawMessage(*s.Payload)
	case s.Kind == "create_event":
		out.Event = json.RawMessage(*s.Payload)
	case s.ItemID != nil:
		it, err := store.Items.Get(op.ctx, op.q, op.User.ID, *s.ItemID)
		if err != nil {
			return nil, err
		}
		if it != nil {
			item, err := op.renderItem(it)
			if err != nil {
				return nil, err
			}
			out.Item, _ = json.Marshal(item)
		}
	case s.EventID != nil:
		ev, err := store.Events.Get(op.ctx, op.q, op.User.ID, *s.EventID)
		if err != nil {
			return nil, err
		}
		if ev != nil {
			event, err := op.renderEvent(ev)
			if err != nil {
				return nil, err
			}
			out.Event, _ = json.Marshal(event)
		}
	}
	return out, nil
}

func subFields(raw json.RawMessage, name string) (Fields, error) {
	var f Fields
	if raw == nil || json.Unmarshal(raw, &f) != nil || f == nil {
		return nil, Invalid("%s must be an object", name)
	}
	return f, nil
}

func copyFields(f Fields) Fields {
	out := Fields{}
	for k, v := range f {
		out[k] = v
	}
	return out
}

// CreateSuggestion validates a proposal and stores it as pending.
func (op *Op) CreateSuggestion(f Fields) (*api.Suggestion, error) {
	if op.Reason == "" {
		return nil, Invalid("reason is required")
	}
	r := newReader(f)
	s := &store.Suggestion{}
	r.str("kind", &s.Kind, 20)
	r.nullStr("item_id", &s.ItemID, anyID, "an item ID")
	r.nullStr("event_id", &s.EventID, anyID, "an event ID")
	r.instant("start", &s.StartAt)
	r.instant("end", &s.EndAt)
	rawItem, _ := r.raw("item")
	rawEvent, _ := r.raw("event")
	if err := r.done(); err != nil {
		return nil, err
	}
	var err error
	switch s.Kind {
	case "schedule_item":
		err = op.prepareItemSuggestion(s, true)
	case "delete_item":
		err = op.prepareItemSuggestion(s, false)
	case "create_item":
		err = op.prepareCreateItem(s, rawItem)
	case "create_event":
		err = op.prepareCreateEvent(s, rawEvent)
	case "move_event":
		err = op.prepareEventSuggestion(s, true)
	case "delete_event":
		err = op.prepareEventSuggestion(s, false)
	default:
		err = Invalid("kind must be schedule_item, create_item, create_event, move_event, delete_event or delete_item")
	}
	if err != nil {
		return nil, err
	}
	n, err := store.Suggestions.Count(op.ctx, op.q, op.User.ID, "status = 'pending'")
	if err != nil {
		return nil, err
	}
	if n >= 500 {
		return nil, Invalid("limit reached: at most 500 suggestions can be pending")
	}
	return op.suggest(s)
}

func requireSlot(s *store.Suggestion) error {
	if s.StartAt == nil || s.EndAt == nil {
		return Invalid("start and end are required")
	}
	return checkSlot(*s.StartAt, *s.EndAt)
}

func (op *Op) prepareItemSuggestion(s *store.Suggestion, slot bool) error {
	if s.ItemID == nil {
		return Invalid("item_id is required")
	}
	it, err := op.item(*s.ItemID)
	if err != nil {
		return err
	}
	s.Title, s.EventID = it.Title, nil
	if !slot {
		s.StartAt, s.EndAt = nil, nil
		return nil
	}
	return requireSlot(s)
}

func (op *Op) prepareEventSuggestion(s *store.Suggestion, slot bool) error {
	if s.EventID == nil {
		return Invalid("event_id is required")
	}
	ev, err := op.event(*s.EventID)
	if err != nil {
		return err
	}
	if ev.RRule != nil {
		return errRecurring
	}
	s.Title, s.ItemID = ev.Title, nil
	if !slot {
		s.StartAt, s.EndAt = nil, nil
		return nil
	}
	if ev.AllDay {
		return Invalid("all-day events cannot be moved by a suggestion")
	}
	if s.StartAt != nil && s.EndAt == nil {
		length := store.ParseTime(*ev.EndAt).Sub(store.ParseTime(*ev.StartAt))
		end := store.FormatTime(store.ParseTime(*s.StartAt).Add(length))
		s.EndAt = &end
	}
	return requireSlot(s)
}

func (op *Op) prepareCreateItem(s *store.Suggestion, raw json.RawMessage) error {
	f, err := subFields(raw, "item")
	if err != nil {
		return err
	}
	draft := &store.Item{Status: "open"}
	if err := op.applyItemFields(draft, copyFields(f), true); err != nil {
		return err
	}
	payload := string(raw)
	s.Payload, s.Title, s.ItemID, s.EventID = &payload, draft.Title, nil, nil
	if s.StartAt == nil && s.EndAt == nil {
		return nil
	}
	return requireSlot(s)
}

func (op *Op) prepareCreateEvent(s *store.Suggestion, raw json.RawMessage) error {
	f, err := subFields(raw, "event")
	if err != nil {
		return err
	}
	draft := &store.Event{}
	if err := op.applyEventFields(draft, copyFields(f), true); err != nil {
		return err
	}
	payload := string(raw)
	s.Payload, s.Title, s.ItemID, s.EventID = &payload, draft.Title, nil, nil
	s.StartAt, s.EndAt = draft.StartAt, draft.EndAt
	return nil
}

func (op *Op) ListSuggestions(status string) ([]api.Suggestion, error) {
	where, args := "status = ?", []any{status}
	switch status {
	case "":
		args = []any{"pending"}
	case "pending", "accepted", "rejected":
	case "any":
		where, args = "", nil
	default:
		return nil, Invalid("status must be pending, accepted, rejected or any")
	}
	rows, err := store.Suggestions.List(op.ctx, op.q, op.User.ID, where, args...)
	if err != nil {
		return nil, err
	}
	out := make([]api.Suggestion, 0, len(rows))
	for _, s := range rows {
		one, err := op.renderSuggestion(s)
		if err != nil {
			return nil, err
		}
		out = append(out, *one)
	}
	return out, nil
}

func (op *Op) pendingSuggestion(id string) (*store.Suggestion, error) {
	s, err := store.Suggestions.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, NotFound("suggestion")
	}
	if s.Status != "pending" {
		return nil, Conflict("the suggestion is already %s", s.Status)
	}
	return s, nil
}

func (op *Op) decide(s *store.Suggestion, status string) error {
	now := op.now()
	s.Status, s.DecidedAt = status, &now
	op.deco = nil
	return store.Suggestions.Put(op.ctx, op.q, s)
}

func (op *Op) RejectSuggestion(id string) (*api.Suggestion, error) {
	s, err := op.pendingSuggestion(id)
	if err != nil {
		return nil, err
	}
	if err := op.decide(s, "rejected"); err != nil {
		return nil, err
	}
	return op.renderSuggestion(s)
}

// WithdrawSuggestion removes a pending suggestion, as its proposer taking it
// back. A token reaches only the suggestions it made itself: any other one
// looks missing to it.
func (op *Op) WithdrawSuggestion(id string) error {
	s, err := store.Suggestions.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return err
	}
	if s == nil || (op.ID.Token != nil && !eqStr(s.TokenID, &op.ID.Token.ID)) {
		return NotFound("suggestion")
	}
	if s.Status != "pending" {
		return Conflict("the suggestion is already %s", s.Status)
	}
	op.deco = nil
	return store.Suggestions.Delete(op.ctx, op.q, op.User.ID, id)
}

// AcceptSuggestion applies a pending suggestion. One that can no longer be
// applied is marked rejected and reported as a conflict.
func (op *Op) AcceptSuggestion(id string) (*api.Suggestion, *api.Activity, error) {
	s, err := op.pendingSuggestion(id)
	if err != nil {
		return nil, nil, err
	}
	activity, err := op.tryAccept(s)
	if err != nil {
		return nil, nil, err
	}
	out, err := op.renderSuggestion(s)
	if err != nil {
		return nil, nil, err
	}
	a := activityJSON(activity)
	return out, &a, nil
}

// AcceptAll accepts every pending suggestion, oldest first, skipping (and
// rejecting) the ones that can no longer be applied.
func (op *Op) AcceptAll() (int, []string, error) {
	rows, err := store.Suggestions.List(op.ctx, op.q, op.User.ID, "status = 'pending'")
	if err != nil {
		return 0, nil, err
	}
	ids := []string{}
	for _, s := range rows {
		activity, err := op.tryAccept(s)
		var keep *committed
		if errors.As(err, &keep) {
			continue
		}
		if err != nil {
			return 0, nil, err
		}
		ids = append(ids, activity.ID)
	}
	return len(ids), ids, nil
}

// tryAccept applies a suggestion inside a savepoint so that a failed attempt
// leaves nothing behind except the suggestion being marked rejected.
func (op *Op) tryAccept(s *store.Suggestion) (*store.Activity, error) {
	if _, err := op.q.ExecContext(op.ctx, "SAVEPOINT suggestion"); err != nil {
		return nil, err
	}
	caller, reason := op.ID, op.Reason
	proposer := *caller
	proposer.Actor = api.Actor{Kind: s.ActorKind, Name: s.ActorName}
	op.ID, op.Reason = &proposer, s.Reason
	activity, err := op.apply(s)
	op.ID, op.Reason = caller, reason

	var apiErr *Error
	if errors.As(err, &apiErr) {
		if _, rerr := op.q.ExecContext(op.ctx, "ROLLBACK TO suggestion"); rerr != nil {
			return nil, rerr
		}
		op.changes, op.deco = nil, nil
		if derr := op.decide(s, "rejected"); derr != nil {
			return nil, derr
		}
		return nil, &committed{Conflict("the suggestion can no longer be applied: %s", apiErr.Message)}
	}
	if err != nil {
		return nil, err
	}
	if _, err := op.q.ExecContext(op.ctx, "RELEASE suggestion"); err != nil {
		return nil, err
	}
	return activity, op.decide(s, "accepted")
}

// apply carries out a suggestion and logs it under the proposer's name.
func (op *Op) apply(s *store.Suggestion) (*store.Activity, error) {
	var action, summary string
	var err error
	switch s.Kind {
	case "schedule_item", "delete_item":
		var it *store.Item
		if it, err = op.item(*s.ItemID); err != nil {
			return nil, err
		}
		if s.Kind == "schedule_item" {
			action = "item.schedule"
			summary, err = op.scheduleItem(it, *s.StartAt, *s.EndAt)
		} else {
			action = "item.delete"
			summary, err = op.deleteItem(it)
		}
	case "create_item":
		action = "item.create"
		summary, err = op.applyCreateItem(s)
	case "create_event":
		action = "event.create"
		var f Fields
		if f, err = subFields(json.RawMessage(*s.Payload), "event"); err == nil {
			_, summary, err = op.createEvent(f)
		}
	case "move_event", "delete_event":
		var ev *store.Event
		if ev, err = op.event(*s.EventID); err != nil {
			return nil, err
		}
		if s.Kind == "delete_event" {
			action = "event.delete"
			summary, err = op.deleteEvent(ev)
		} else {
			action = "event.update"
			slot := Fields{"start": quoteJSON(*s.StartAt), "end": quoteJSON(*s.EndAt)}
			summary, err = op.updateEvent(ev, slot)
		}
	}
	if err != nil {
		return nil, err
	}
	return op.log(action, summary)
}

func (op *Op) applyCreateItem(s *store.Suggestion) (string, error) {
	f, err := subFields(json.RawMessage(*s.Payload), "item")
	if err != nil {
		return "", err
	}
	it, summary, err := op.createItem(f)
	if err != nil || s.StartAt == nil {
		return summary, err
	}
	if _, err := op.scheduleItem(it, *s.StartAt, *s.EndAt); err != nil {
		return "", err
	}
	return summary + "，排到 " + op.whenLabel(store.ParseTime(*s.StartAt)), nil
}

func quoteJSON(s string) json.RawMessage {
	raw, _ := json.Marshal(s)
	return raw
}

// overlay adds tentative entries for pending suggestions and marks the events
// they would move or delete as leaving.
func (op *Op) overlay(entries []entry, from, to time.Time) ([]entry, error) {
	pending, err := store.Suggestions.List(op.ctx, op.q, op.User.ID, "status = 'pending'")
	if err != nil {
		return nil, err
	}
	for _, s := range pending {
		if s.Kind == "move_event" || s.Kind == "delete_event" {
			for i := range entries {
				e := &entries[i].ev
				if e.ID == *s.EventID && e.Status == "confirmed" {
					e.Status, e.SuggestionID = "leaving", &s.ID
				}
			}
		}
		tentative, err := op.tentative(s)
		if err != nil {
			return nil, err
		}
		if tentative != nil && overlaps(tentative.start, tentative.end, from, to) {
			entries = append(entries, *tentative)
		}
	}
	return entries, nil
}

// tentative builds the entry a pending suggestion would create, if any.
func (op *Op) tentative(s *store.Suggestion) (*entry, error) {
	var ev *store.Event
	block := false
	switch s.Kind {
	case "schedule_item":
		it, err := store.Items.Get(op.ctx, op.q, op.User.ID, *s.ItemID)
		if err != nil || it == nil {
			return nil, err
		}
		ev = &store.Event{ItemID: &it.ID, ProjectID: it.ProjectID, Title: it.Title, StartAt: s.StartAt, EndAt: s.EndAt}
		block = true
	case "create_item":
		if s.StartAt == nil {
			return nil, nil
		}
		draft := &store.Item{Status: "open"}
		f, err := subFields(json.RawMessage(*s.Payload), "item")
		if err != nil || op.applyItemFields(draft, f, true) != nil {
			return nil, nil
		}
		ev = &store.Event{ProjectID: draft.ProjectID, Title: draft.Title, StartAt: s.StartAt, EndAt: s.EndAt}
		block = true
	case "create_event":
		ev = &store.Event{}
		f, err := subFields(json.RawMessage(*s.Payload), "event")
		if err != nil || op.applyEventFields(ev, f, true) != nil {
			return nil, nil
		}
	case "move_event":
		orig, err := store.Events.Get(op.ctx, op.q, op.User.ID, *s.EventID)
		if err != nil || orig == nil {
			return nil, err
		}
		moved := *orig
		moved.StartAt, moved.EndAt = s.StartAt, s.EndAt
		ev = &moved
	default:
		return nil, nil
	}
	out := eventJSON(ev)
	out.ID, out.Status, out.SuggestionID, out.Readonly = s.ID, "tentative", &s.ID, true
	out.CreatedBy = api.Actor{Kind: s.ActorKind, Name: s.ActorName}
	out.UpdatedAt = s.CreatedAt
	start, end := eventSpan(ev, op.Loc)
	return &entry{ev: out, start: start, end: end, block: block}, nil
}
