package core

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/store"
)

const (
	maxEventSpan = 31 * 24 * time.Hour
	inboxKey     = "inbox"
)

func calendarKey(projectID *string) string {
	if projectID == nil {
		return inboxKey
	}
	return *projectID
}

// saveEvent stores an event, records the change and tells CalDAV clients.
// keepICal is set when the iCalendar text was just supplied by a client.
func (op *Op) saveEvent(ev *store.Event, keepICal bool) error {
	before, err := store.Events.Get(op.ctx, op.q, op.User.ID, ev.ID)
	if err != nil {
		return err
	}
	ev.UpdatedAt = op.now()
	if !keepICal {
		if err := refreshICal(ev, op.Loc, op.Now); err != nil {
			return err
		}
	}
	ev.ETag = NewID()
	if err := save(op, kindEvent, store.Events, ev); err != nil {
		return err
	}
	op.deco = nil
	if before != nil && calendarKey(before.ProjectID) != calendarKey(ev.ProjectID) {
		if err := store.BumpCTag(op.ctx, op.q, op.User.ID, calendarKey(before.ProjectID)); err != nil {
			return err
		}
	}
	return store.BumpCTag(op.ctx, op.q, op.User.ID, calendarKey(ev.ProjectID))
}

func (op *Op) removeEvent(ev *store.Event) error {
	if err := remove(op, kindEvent, store.Events, ev.ID); err != nil {
		return err
	}
	op.deco = nil
	return store.BumpCTag(op.ctx, op.q, op.User.ID, calendarKey(ev.ProjectID))
}

func (op *Op) event(id string) (*store.Event, error) {
	ev, err := store.Events.Get(op.ctx, op.q, op.User.ID, id)
	if err != nil {
		return nil, err
	}
	if ev == nil {
		return nil, NotFound("event")
	}
	return ev, nil
}

func eventJSON(ev *store.Event) api.Event {
	return api.Event{
		ID: ev.ID, ProjectID: ev.ProjectID, ItemID: ev.ItemID,
		Title: ev.Title, Notes: ev.Notes, Location: ev.Location,
		AllDay: ev.AllDay, Start: ev.StartAt, End: ev.EndAt, StartDate: ev.StartDate, EndDate: ev.EndDate,
		RRule: ev.RRule, Recurring: ev.RRule != nil, Status: "confirmed", Readonly: ev.RRule != nil,
		CreatedBy: api.Actor{Kind: ev.CreatedByKind, Name: ev.CreatedByName}, UpdatedAt: ev.UpdatedAt,
	}
}

// renderEvent is eventJSON plus the state of the item a time block belongs to.
func (op *Op) renderEvent(ev *store.Event) (*api.Event, error) {
	out := eventJSON(ev)
	if ev.ItemID != nil {
		it, err := store.Items.Get(op.ctx, op.q, op.User.ID, *ev.ItemID)
		if err != nil {
			return nil, err
		}
		if it != nil {
			done := it.Status == "done"
			out.ItemDone = &done
		}
	}
	return &out, nil
}

func (op *Op) GetEvent(id string) (*api.Event, error) {
	ev, err := op.event(id)
	if err != nil {
		return nil, err
	}
	return op.renderEvent(ev)
}

// applyEventFields validates input and copies it onto the row.
func (op *Op) applyEventFields(ev *store.Event, f Fields, creating bool) error {
	r := newReader(f)
	oldStart, oldEnd := ev.StartAt, ev.EndAt
	if creating && !r.has("title") {
		r.fail("title is required")
	}
	r.name("title", &ev.Title, maxTitle)
	r.str("notes", &ev.Notes, maxNotes)
	r.str("location", &ev.Location, maxTitle)
	r.nullStr("project_id", &ev.ProjectID, anyID, "a project ID")
	r.boolean("all_day", &ev.AllDay)
	r.instant("start", &ev.StartAt)
	r.instant("end", &ev.EndAt)
	r.date("start_date", &ev.StartDate)
	r.date("end_date", &ev.EndDate)
	if err := r.done(); err != nil {
		return err
	}
	if ev.ProjectID != nil && r.has("project_id") {
		if p, err := store.Projects.Get(op.ctx, op.q, op.User.ID, *ev.ProjectID); err != nil {
			return err
		} else if p == nil {
			return Invalid("project_id does not name one of your projects")
		}
	}
	if ev.AllDay {
		if ev.ItemID != nil {
			return Invalid("all_day cannot be set on a time block")
		}
		if ev.StartDate == nil || ev.EndDate == nil {
			return Invalid("start_date and end_date are required when all_day is true")
		}
		if *ev.EndDate < *ev.StartDate {
			return Invalid("end_date must not be before start_date")
		}
		if dayStart(*ev.EndDate, time.UTC).Sub(dayStart(*ev.StartDate, time.UTC)) > 366*24*time.Hour {
			return Invalid("an all-day event may span at most 366 days")
		}
		ev.StartAt, ev.EndAt = nil, nil
		return nil
	}
	// Moving the start alone keeps the duration.
	if !creating && r.has("start") && !r.has("end") && ev.StartAt != nil && oldStart != nil && oldEnd != nil {
		length := store.ParseTime(*oldEnd).Sub(store.ParseTime(*oldStart))
		end := store.FormatTime(store.ParseTime(*ev.StartAt).Add(length))
		ev.EndAt = &end
	}
	if ev.StartAt == nil || ev.EndAt == nil {
		return Invalid("start and end are required")
	}
	if err := checkSlot(*ev.StartAt, *ev.EndAt); err != nil {
		return err
	}
	ev.StartDate, ev.EndDate = nil, nil
	return nil
}

func checkSlot(start, end string) error {
	s, e := store.ParseTime(start), store.ParseTime(end)
	if !e.After(s) {
		return Invalid("end must be after start")
	}
	if e.Sub(s) > maxEventSpan {
		return Invalid("an event may last at most 31 days")
	}
	return nil
}

func (op *Op) newEvent() (*store.Event, error) {
	n, err := store.Events.Count(op.ctx, op.q, op.User.ID, "")
	if err != nil {
		return nil, err
	}
	if err := op.checkLimit(n, op.svc.Limits.Events, "events"); err != nil {
		return nil, err
	}
	id := NewID()
	return &store.Event{
		ID: id, UserID: op.User.ID, UID: id + "@keduly", DavName: id + ".ics",
		CreatedByKind: op.ID.Actor.Kind, CreatedByName: op.ID.Actor.Name, CreatedAt: op.now(),
	}, nil
}

// createEvent creates an event without logging; it returns the summary to log.
func (op *Op) createEvent(f Fields) (*store.Event, string, error) {
	ev, err := op.newEvent()
	if err != nil {
		return nil, "", err
	}
	if err := op.applyEventFields(ev, f, true); err != nil {
		return nil, "", err
	}
	if err := op.saveEvent(ev, false); err != nil {
		return nil, "", err
	}
	return ev, "新建日程" + quote(ev.Title), nil
}

func (op *Op) CreateEvent(f Fields) (*api.Event, error) {
	ev, summary, err := op.createEvent(f)
	if err != nil {
		return nil, err
	}
	if _, err := op.log("event.create", summary); err != nil {
		return nil, err
	}
	return op.renderEvent(ev)
}

var errRecurring = Invalid("recurring events are read-only here; change them from a CalDAV client")

// updateEvent changes an event without logging; it returns the summary to log.
func (op *Op) updateEvent(ev *store.Event, f Fields) (string, error) {
	if ev.RRule != nil {
		return "", errRecurring
	}
	before := *ev
	if err := op.applyEventFields(ev, f, false); err != nil {
		return "", err
	}
	if err := op.saveEvent(ev, false); err != nil {
		return "", err
	}
	if err := op.syncBlockItem(ev); err != nil {
		return "", err
	}
	if !sameSlot(&before, ev) {
		return op.movedSummary(ev.Title, &before, ev), nil
	}
	if before.Title != ev.Title {
		return "日程" + quote(before.Title) + "改名为" + quote(ev.Title), nil
	}
	return "修改了日程" + quote(ev.Title), nil
}

// syncBlockItem keeps an item's planned date on the day of its time block.
func (op *Op) syncBlockItem(ev *store.Event) error {
	if ev.ItemID == nil || ev.AllDay || ev.StartAt == nil {
		return nil
	}
	it, err := store.Items.Get(op.ctx, op.q, op.User.ID, *ev.ItemID)
	if err != nil || it == nil {
		return err
	}
	day := store.ParseTime(*ev.StartAt).In(op.Loc).Format(dateLayout)
	if it.PlannedDate != nil && *it.PlannedDate == day {
		return nil
	}
	it.PlannedDate = &day
	it.UpdatedAt = op.now()
	return save(op, kindItem, store.Items, it)
}

func (op *Op) UpdateEvent(id string, f Fields) (*api.Event, error) {
	ev, err := op.event(id)
	if err != nil {
		return nil, err
	}
	summary, err := op.updateEvent(ev, f)
	if err != nil {
		return nil, err
	}
	if _, err := op.log("event.update", summary); err != nil {
		return nil, err
	}
	return op.renderEvent(ev)
}

func (op *Op) deleteEvent(ev *store.Event) (string, error) {
	if err := op.removeEvent(ev); err != nil {
		return "", err
	}
	return "删除日程" + quote(ev.Title), nil
}

// needsConfirmation reports whether the caller's deletions become suggestions.
func (op *Op) needsConfirmation() bool {
	return op.ID.Token != nil && op.ID.Token.ConfirmDelete
}

// DeleteEvent deletes the event, or returns a delete suggestion when the
// caller's token requires confirmation.
func (op *Op) DeleteEvent(id string) (*api.Suggestion, error) {
	ev, err := op.event(id)
	if err != nil {
		return nil, err
	}
	if ev.RRule != nil {
		return nil, errRecurring
	}
	if op.needsConfirmation() {
		return op.suggest(&store.Suggestion{Kind: "delete_event", EventID: &ev.ID, Title: ev.Title})
	}
	summary, err := op.deleteEvent(ev)
	if err != nil {
		return nil, err
	}
	_, err = op.log("event.delete", summary)
	return nil, err
}

// entry is one thing on the timeline with its position as instants.
type entry struct {
	ev         api.Event
	start, end time.Time
	block      bool // a tentative time block for an item that does not exist yet
}

func (e entry) isBlock() bool { return e.ev.ItemID != nil || e.block }

func overlaps(start, end, from, to time.Time) bool {
	return start.Before(to) && (end.After(from) || (end.Equal(start) && !start.Before(from)))
}

// entries returns everything that overlaps [from, to), sorted by start.
// With overlays, pending suggestions add tentative entries and mark the
// events they would move or delete as leaving.
func (op *Op) entries(from, to time.Time, overlays bool) ([]entry, error) {
	fromDate := from.In(op.Loc).AddDate(0, 0, -1).Format(dateLayout)
	toDate := to.In(op.Loc).AddDate(0, 0, 1).Format(dateLayout)
	rows, err := store.Events.List(op.ctx, op.q, op.User.ID,
		`rrule IS NOT NULL
		 OR (all_day = 0 AND start_at < ? AND end_at >= ?)
		 OR (all_day = 1 AND start_date <= ? AND end_date >= ?)`,
		store.FormatTime(to), store.FormatTime(from), toDate, fromDate)
	if err != nil {
		return nil, err
	}
	var out []entry
	for _, ev := range rows {
		if ev.RRule == nil {
			start, end := eventSpan(ev, op.Loc)
			if overlaps(start, end, from, to) {
				out = append(out, entry{ev: eventJSON(ev), start: start, end: end})
			}
			continue
		}
		for _, occ := range expand(ev, from, to, op.Loc) {
			out = append(out, op.instance(ev, occ))
		}
	}
	if overlays {
		if out, err = op.overlay(out, from, to); err != nil {
			return nil, err
		}
	}
	if err := op.fillItemDone(out); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].start.Equal(out[j].start) {
			return out[i].start.Before(out[j].start)
		}
		return out[i].ev.AllDay && !out[j].ev.AllDay
	})
	return out, nil
}

func (op *Op) instance(ev *store.Event, occ occurrence) entry {
	e := eventJSON(ev)
	original := store.FormatTime(occ.Original)
	e.Instance = &original
	e.Readonly = true
	if occ.Title != "" {
		e.Title = occ.Title
	}
	if ev.AllDay {
		sd := occ.Start.In(op.Loc).Format(dateLayout)
		ed := occ.End.In(op.Loc).AddDate(0, 0, -1).Format(dateLayout)
		e.StartDate, e.EndDate = &sd, &ed
	} else {
		s, en := store.FormatTime(occ.Start), store.FormatTime(occ.End)
		e.Start, e.End = &s, &en
	}
	return entry{ev: e, start: occ.Start, end: occ.End}
}

func (op *Op) fillItemDone(entries []entry) error {
	var ids []string
	for _, e := range entries {
		if e.ev.ItemID != nil {
			ids = append(ids, *e.ev.ItemID)
		}
	}
	items, err := op.itemsByID(ids)
	if err != nil {
		return err
	}
	for i := range entries {
		if id := entries[i].ev.ItemID; id != nil {
			if it := items[*id]; it != nil {
				done := it.Status == "done"
				entries[i].ev.ItemDone = &done
			}
		}
	}
	return nil
}

func (op *Op) itemsByID(ids []string) (map[string]*store.Item, error) {
	out := map[string]*store.Item{}
	for len(ids) > 0 {
		n := min(len(ids), 400)
		args := make([]any, n)
		for i, id := range ids[:n] {
			args[i] = id
		}
		rows, err := store.Items.List(op.ctx, op.q, op.User.ID,
			"id IN (?"+strings.Repeat(",?", n-1)+")", args...)
		if err != nil {
			return nil, err
		}
		for _, it := range rows {
			out[it.ID] = it
		}
		ids = ids[n:]
	}
	return out, nil
}

// ParseRange validates the from and to query values of GET /events.
func ParseRange(from, to string) (time.Time, time.Time, error) {
	f, err1 := time.Parse(time.RFC3339, from)
	t, err2 := time.Parse(time.RFC3339, to)
	if err1 != nil || err2 != nil {
		return f, t, Invalid("from and to must be RFC 3339 timestamps")
	}
	if !t.After(f) {
		return f, t, Invalid("to must be after from")
	}
	if t.Sub(f) > 400*24*time.Hour {
		return f, t, Invalid("from and to may be at most 400 days apart")
	}
	return f, t, nil
}

func (op *Op) ListEvents(from, to time.Time) ([]api.Event, error) {
	entries, err := op.entries(from, to, true)
	if err != nil {
		return nil, err
	}
	return eventsOf(entries, func(entry) bool { return true }), nil
}

func eventsOf(entries []entry, keep func(entry) bool) []api.Event {
	out := []api.Event{}
	for _, e := range entries {
		if keep(e) {
			out = append(out, e.ev)
		}
	}
	return out
}

// Heat counts events and time blocks per local day of a year.
func (op *Op) Heat(year string) (map[string]int, error) {
	y, err := strconv.Atoi(year)
	if err != nil || y < 1970 || y > 9999 {
		return nil, Invalid("year must be a four-digit year")
	}
	from := time.Date(y, 1, 1, 0, 0, 0, 0, op.Loc)
	to := time.Date(y+1, 1, 1, 0, 0, 0, 0, op.Loc)
	entries, err := op.entries(from, to, false)
	if err != nil {
		return nil, err
	}
	days := map[string]int{}
	for _, e := range entries {
		for _, d := range op.daysOf(e, from, to) {
			days[d]++
		}
	}
	return days, nil
}

// daysOf lists the local dates an entry touches within [from, to).
func (op *Op) daysOf(e entry, from, to time.Time) []string {
	start, last := e.start, e.end.Add(-time.Nanosecond)
	if last.Before(start) {
		last = start
	}
	if start.Before(from) {
		start = from
	}
	if !last.Before(to) {
		last = to.Add(-time.Nanosecond)
	}
	var out []string
	lastDay := last.In(op.Loc).Format(dateLayout)
	for d := start.In(op.Loc); ; d = d.AddDate(0, 0, 1) {
		day := d.Format(dateLayout)
		if day > lastDay {
			break
		}
		out = append(out, day)
	}
	return out
}
