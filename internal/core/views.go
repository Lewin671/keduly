package core

import (
	"sort"
	"strconv"
	"time"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/store"
)

func userJSON(u *store.User) api.User {
	return api.User{ID: u.ID, Email: u.Email, Name: u.Name, Timezone: u.Timezone, TimezoneAuto: u.TimezoneAuto,
		WorkStart: u.WorkStart, WorkEnd: u.WorkEnd, CreatedAt: u.CreatedAt}
}

func (op *Op) Me() api.User { return userJSON(op.User) }

func (op *Op) Revision() (int64, error) {
	return store.Revision(op.ctx, op.q, op.User.ID)
}

func (op *Op) Counts() (api.Counts, error) {
	var c api.Counts
	today := op.today()
	var err error
	if c.Inbox, err = store.Items.Count(op.ctx, op.q, op.User.ID, "status = 'open' AND project_id IS NULL"); err != nil {
		return c, err
	}
	if c.Today, err = store.Items.Count(op.ctx, op.q, op.User.ID,
		"status = 'open' AND (planned_date <= ? OR due_date < ?)", today, today); err != nil {
		return c, err
	}
	c.Pending, err = store.Suggestions.Count(op.ctx, op.q, op.User.ID, "status = 'pending'")
	return c, err
}

func (op *Op) Bootstrap() (*api.Bootstrap, error) {
	out := &api.Bootstrap{User: op.Me(), Areas: []api.Area{}, Headings: []api.Heading{}}
	areas, err := store.Areas.List(op.ctx, op.q, op.User.ID, "")
	if err != nil {
		return nil, err
	}
	for _, a := range areas {
		out.Areas = append(out.Areas, areaJSON(a))
	}
	if out.Projects, err = op.Projects(); err != nil {
		return nil, err
	}
	headings, err := store.Headings.List(op.ctx, op.q, op.User.ID, "")
	if err != nil {
		return nil, err
	}
	for _, h := range headings {
		out.Headings = append(out.Headings, headingJSON(h))
	}
	if out.Counts, err = op.Counts(); err != nil {
		return nil, err
	}
	out.Revision, err = op.Revision()
	return out, err
}

// ParseDate validates a date query value; empty means def.
func ParseDate(name, value, def string) (string, error) {
	if value == "" {
		value = def
	}
	if !validDate(value) {
		return "", Invalid("%s must be a date (YYYY-MM-DD)", name)
	}
	return value, nil
}

// TodayDate is the current date in the user's time zone.
func (op *Op) TodayDate() string { return op.today() }

// realEvents keeps what the list views show: events that exist and are not time blocks.
func realEvents(e entry) bool {
	return !e.isBlock() && e.ev.Status != "tentative"
}

func (op *Op) Today() (*api.Today, error) {
	today := op.today()
	from, to := op.localDayRange(today)
	rows, err := store.Items.Query(op.ctx, op.q, store.Items.SelectSQL()+` WHERE user_id = ? AND (
			(status = 'open' AND planned_date <= ?) OR
			(status = 'done' AND completed_at >= ? AND completed_at < ?))
		ORDER BY important DESC, status DESC, position, created_at, rowid`,
		op.User.ID, today, store.FormatTime(from), store.FormatTime(to))
	if err != nil {
		return nil, err
	}
	overdue, err := op.queryItems("status = 'open' AND due_date < ? AND (planned_date IS NULL OR planned_date > ?)",
		[]any{today, today}, 0, 0)
	if err != nil {
		return nil, err
	}
	out := &api.Today{Date: today}
	if out.Items, err = op.renderItems(rows); err != nil {
		return nil, err
	}
	if out.Overdue, err = op.renderItems(overdue); err != nil {
		return nil, err
	}
	for _, it := range out.Items {
		if it.Status == "open" && !it.Evening && it.Block == nil && it.Suggestion == nil && it.EstimateMinutes != nil {
			out.UnplannedMinutes += *it.EstimateMinutes
		}
	}
	entries, err := op.entries(from, to, true)
	if err != nil {
		return nil, err
	}
	out.Events = eventsOf(entries, realEvents)
	sort.SliceStable(out.Events, func(i, j int) bool { return out.Events[i].AllDay && !out.Events[j].AllDay })
	for _, gap := range op.gaps(today, entries) {
		out.FreeMinutes += int(gap[1].Sub(gap[0]).Minutes())
	}
	return out, nil
}

// workWindow returns the user's working hours on a date as instants.
func (op *Op) workWindow(date string) (time.Time, time.Time) {
	at := func(clock string) time.Time {
		d := dayStart(date, op.Loc)
		h, _ := strconv.Atoi(clock[:2])
		m, _ := strconv.Atoi(clock[3:])
		return time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, op.Loc)
	}
	return at(op.User.WorkStart), at(op.User.WorkEnd)
}

// gaps returns the stretches of the working day that no timed entry covers.
// All-day events do not block time; overlapping entries count once.
func (op *Op) gaps(date string, entries []entry) [][2]time.Time {
	start, end := op.workWindow(date)
	var busy [][2]time.Time
	for _, e := range entries {
		if e.ev.AllDay || !e.start.Before(end) || !e.end.After(start) {
			continue
		}
		busy = append(busy, [2]time.Time{e.start, e.end})
	}
	sort.Slice(busy, func(i, j int) bool { return busy[i][0].Before(busy[j][0]) })
	var out [][2]time.Time
	cursor := start
	for _, b := range busy {
		if b[0].After(cursor) {
			out = append(out, [2]time.Time{cursor, b[0]})
		}
		if b[1].After(cursor) {
			cursor = b[1]
		}
	}
	if cursor.Before(end) {
		out = append(out, [2]time.Time{cursor, end})
	}
	return out
}

// Free lists the gaps of at least minutes within the working hours of a date.
func (op *Op) Free(date, minutes string) ([]api.Slot, error) {
	date, err := ParseDate("date", date, op.today())
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(minutes)
	if minutes == "" {
		n, err = 30, nil
	}
	if err != nil || n < 1 || n > 24*60 {
		return nil, Invalid("duration must be a number of minutes between 1 and 1440")
	}
	from, to := op.localDayRange(date)
	entries, err := op.entries(from, to, true)
	if err != nil {
		return nil, err
	}
	slots := []api.Slot{}
	for _, gap := range op.gaps(date, entries) {
		if gap[1].Sub(gap[0]) >= time.Duration(n)*time.Minute {
			slots = append(slots, api.Slot{Start: store.FormatTime(gap[0]), End: store.FormatTime(gap[1])})
		}
	}
	return slots, nil
}

// Upcoming groups open items and events by day, leaving out empty days.
func (op *Op) Upcoming(fromDate, toDate string) ([]api.Day, error) {
	fromDate, err := ParseDate("from", fromDate, op.today())
	if err != nil {
		return nil, err
	}
	def := dayStart(fromDate, time.UTC).AddDate(0, 0, 30).Format(dateLayout)
	if toDate, err = ParseDate("to", toDate, def); err != nil {
		return nil, err
	}
	span := dayStart(toDate, time.UTC).Sub(dayStart(fromDate, time.UTC))
	if span < 0 || span > 62*24*time.Hour {
		return nil, Invalid("to must be on or after from and at most 62 days later")
	}
	days := map[string]*api.Day{}
	day := func(date string) *api.Day {
		if days[date] == nil {
			days[date] = &api.Day{Date: date, Events: []api.Event{}, Items: []api.Item{}}
		}
		return days[date]
	}
	rows, err := op.queryItems(`status = 'open' AND coalesce(planned_date, due_date) BETWEEN ? AND ?`,
		[]any{fromDate, toDate}, 0, 0)
	if err != nil {
		return nil, err
	}
	for _, it := range rows {
		item, err := op.renderItem(it)
		if err != nil {
			return nil, err
		}
		date := it.DueDate
		if it.PlannedDate != nil {
			date = it.PlannedDate
		}
		day(*date).Items = append(day(*date).Items, item)
	}
	from, to := dayStart(fromDate, op.Loc), dayStart(toDate, op.Loc).AddDate(0, 0, 1)
	entries, err := op.entries(from, to, true)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !realEvents(e) {
			continue
		}
		for _, date := range op.daysOf(e, from, to) {
			day(date).Events = append(day(date).Events, e.ev)
		}
	}
	out := make([]api.Day, 0, len(days))
	for _, d := range days {
		sort.SliceStable(d.Events, func(i, j int) bool { return d.Events[i].AllDay && !d.Events[j].AllDay })
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

// Overview lists every active project that has open items with its first five.
func (op *Op) Overview() ([]api.OverviewProject, error) {
	projects, err := store.Projects.List(op.ctx, op.q, op.User.ID, "archived = 0")
	if err != nil {
		return nil, err
	}
	rows, err := op.queryItems("status = 'open' AND project_id IS NOT NULL", nil, 0, 0)
	if err != nil {
		return nil, err
	}
	byProject := map[string][]*store.Item{}
	for _, it := range rows {
		byProject[*it.ProjectID] = append(byProject[*it.ProjectID], it)
	}
	out := []api.OverviewProject{}
	for _, p := range projects {
		open := byProject[p.ID]
		if len(open) == 0 {
			continue
		}
		items, err := op.renderItems(open[:min(5, len(open))])
		if err != nil {
			return nil, err
		}
		out = append(out, api.OverviewProject{ProjectID: p.ID, Total: len(open), Items: items})
	}
	return out, nil
}

// Matrix returns the four quadrants with their first six items.
func (op *Op) Matrix() (map[string]api.Quadrant, error) {
	out := map[string]api.Quadrant{}
	for _, name := range []string{"do", "plan", "quick", "later"} {
		where, args, _ := op.quadrantSQL(name)
		rows, err := op.queryItems(where, args, 0, 0)
		if err != nil {
			return nil, err
		}
		q := api.Quadrant{Total: len(rows)}
		for _, it := range rows {
			planned, err := op.planned(it.ID)
			if err != nil {
				return nil, err
			}
			if !planned {
				q.Unplanned++
			}
		}
		if q.Items, err = op.renderItems(rows[:min(6, len(rows))]); err != nil {
			return nil, err
		}
		out[name] = q
	}
	return out, nil
}
