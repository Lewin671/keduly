package core

import (
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"github.com/Lewin671/keduly/internal/store"
)

// Calendar is a CalDAV collection: the inbox or one project.
type Calendar struct {
	Key   string // "inbox" or the project ID
	Name  string
	Color string
	CTag  int64
}

const inboxName = "Keduly"

func (op *Op) calendarOf(key, name, color string) (Calendar, error) {
	ctag, err := store.CTag(op.ctx, op.q, op.User.ID, key)
	return Calendar{Key: key, Name: name, Color: color, CTag: ctag}, err
}

// Calendars lists the inbox calendar and one calendar per active project.
func (op *Op) Calendars() ([]Calendar, error) {
	inbox, err := op.calendarOf(inboxKey, inboxName, "")
	if err != nil {
		return nil, err
	}
	out := []Calendar{inbox}
	projects, err := store.Projects.List(op.ctx, op.q, op.User.ID, "archived = 0")
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		c, err := op.calendarOf(p.ID, p.Name, p.Color)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// Calendar returns nil when the key names no calendar of this user.
func (op *Op) Calendar(key string) (*Calendar, error) {
	if key == inboxKey {
		c, err := op.calendarOf(inboxKey, inboxName, "")
		return &c, err
	}
	p, err := store.Projects.Get(op.ctx, op.q, op.User.ID, key)
	if err != nil || p == nil {
		return nil, err
	}
	c, err := op.calendarOf(p.ID, p.Name, p.Color)
	return &c, err
}

func calendarWhere(key string) (string, []any) {
	if key == inboxKey {
		return "project_id IS NULL", nil
	}
	return "project_id = ?", []any{key}
}

func (op *Op) CalendarEvents(key string) ([]*store.Event, error) {
	where, args := calendarWhere(key)
	return store.Events.List(op.ctx, op.q, op.User.ID, where, args...)
}

// DavEvent finds a calendar object by resource name; nil when absent.
func (op *Op) DavEvent(key, name string) (*store.Event, error) {
	where, args := calendarWhere(key)
	rows, err := store.Events.List(op.ctx, op.q, op.User.ID, where+" AND dav_name = ?", append(args, name)...)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// EventOverlaps reports whether any occurrence of the event touches [from, to).
// A zero bound is open.
func (op *Op) EventOverlaps(ev *store.Event, from, to time.Time) bool {
	if from.IsZero() {
		from = time.Unix(0, 0)
	}
	if to.IsZero() {
		to = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if ev.RRule != nil {
		return len(expand(ev, from, to, op.Loc)) > 0
	}
	start, end := eventSpan(ev, op.Loc)
	return overlaps(start, end, from, to)
}

// PutDavEvent creates or replaces the calendar object at key/name from
// client-supplied iCalendar data, keeping the client's text.
func (op *Op) PutDavEvent(key, name string, cal *ical.Calendar) (*store.Event, error) {
	if name == "" || len(name) > 255 || strings.ContainsAny(name, "/\\") {
		return nil, Invalid("the resource name is not valid")
	}
	calendar, err := op.Calendar(key)
	if err != nil {
		return nil, err
	}
	if calendar == nil {
		return nil, NotFound("calendar")
	}
	ev, err := op.DavEvent(key, name)
	if err != nil {
		return nil, err
	}
	var before *store.Event
	if ev == nil {
		if ev, err = op.newEvent(); err != nil {
			return nil, err
		}
		ev.DavName = name
		if key != inboxKey {
			ev.ProjectID = &key
		}
	} else {
		copy := *ev
		before = &copy
	}
	fallbackUID := ev.UID
	if err := applyICal(ev, cal, op.Loc); err != nil {
		return nil, err
	}
	if ev.UID == "" {
		ev.UID = fallbackUID
		masterEvent(cal).Props.SetText(ical.PropUID, ev.UID)
	}
	if ev.ItemID != nil && ev.AllDay {
		return nil, Invalid("a time block cannot become an all-day event")
	}
	if ev.ICal, err = EncodeICal(cal, op.Now); err != nil {
		return nil, Invalid("the calendar object cannot be stored: %v", err)
	}
	if err := op.saveEvent(ev, true); err != nil {
		return nil, err
	}
	if err := op.syncBlockItem(ev); err != nil {
		return nil, err
	}
	action, summary := "event.create", "新建日程"+quote(ev.Title)
	if before != nil {
		action, summary = "event.update", "修改了日程"+quote(ev.Title)
		if !sameSlot(before, ev) {
			summary = op.movedSummary(ev.Title, before, ev)
		} else if before.Title != ev.Title {
			summary = "日程" + quote(before.Title) + "改名为" + quote(ev.Title)
		}
	}
	_, err = op.log(action, summary)
	return ev, err
}

func (op *Op) DeleteDavEvent(ev *store.Event) error {
	summary, err := op.deleteEvent(ev)
	if err != nil {
		return err
	}
	_, err = op.log("event.delete", summary)
	return err
}
