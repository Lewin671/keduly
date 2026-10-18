package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/store"
)

type habitRecord struct {
	Status      string    `json:"status"`
	CompletedAt *string   `json:"completed_at"`
	ScheduledAt *string   `json:"scheduled_at"`
	SkipReason  string    `json:"skip_reason"`
	Actor       api.Actor `json:"actor"`
}

func (op *Op) habitRules() ([]api.Habit, error) {
	rows, err := op.q.QueryContext(op.ctx, `SELECT r.body FROM habit_rules r JOIN habits h ON h.id=r.habit_id WHERE h.user_id=? ORDER BY r.habit_id,r.version`, op.User.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.Habit{}
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			return nil, err
		}
		var h api.Habit
		if err = json.Unmarshal([]byte(s), &h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
func (op *Op) Habits() ([]api.Habit, error) {
	rules, err := op.habitRules()
	if err != nil {
		return nil, err
	}
	projects, e := store.Projects.List(op.ctx, op.q, op.User.ID, "")
	if e != nil {
		return nil, e
	}
	valid := map[string]bool{}
	for _, p := range projects {
		valid[p.ID] = true
	}
	latest := map[string]api.Habit{}
	for _, h := range rules {
		if h.ProjectID != nil && !valid[*h.ProjectID] {
			h.ProjectID = nil
		}
		latest[h.ID] = h
	}
	out := []api.Habit{}
	for _, h := range latest {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (op *Op) SaveHabit(id string, f Fields) (*api.Habit, error) {
	all, err := op.Habits()
	if err != nil {
		return nil, err
	}
	var h api.Habit
	if id != "" {
		i := slices.IndexFunc(all, func(x api.Habit) bool { return x.ID == id })
		if i < 0 {
			return nil, NotFound("habit")
		}
		h = all[i]
	} else {
		if len(all) >= 100 {
			return nil, Invalid("at most 100 habits")
		}
		h = api.Habit{ID: NewID(), Weekdays: []int{1, 2, 3, 4, 5, 6, 7}, EffectiveDate: op.today(), Timezone: op.User.Timezone}
	}
	oldVersion := h.Version
	r := newReader(f)
	r.name("title", &h.Title, maxTitle)
	r.str("notes", &h.Notes, maxNotes)
	r.str("time", &h.Time, 5)
	if raw, ok := r.raw("project_id"); ok {
		if err = json.Unmarshal(raw, &h.ProjectID); err != nil {
			r.fail("project_id must be a string or null")
		}
	}
	if raw, ok := r.raw("weekdays"); ok {
		if err = json.Unmarshal(raw, &h.Weekdays); err != nil {
			r.fail("weekdays must be an array")
		}
	}
	if raw, ok := r.raw("duration_minutes"); ok {
		if err = json.Unmarshal(raw, &h.DurationMinutes); err != nil {
			r.fail("invalid duration_minutes")
		}
	}
	if raw, ok := r.raw("day_offset"); ok {
		if err = json.Unmarshal(raw, &h.DayOffset); err != nil {
			r.fail("invalid day_offset")
		}
	}
	if raw, ok := r.raw("reminder_minutes"); ok {
		if err = json.Unmarshal(raw, &h.ReminderMinutes); err != nil {
			r.fail("invalid reminder_minutes")
		}
	}
	if raw, ok := r.raw("paused"); ok {
		if err = json.Unmarshal(raw, &h.Paused); err != nil {
			r.fail("invalid paused")
		}
	}
	var version int
	if raw, ok := r.raw("version"); ok {
		if json.Unmarshal(raw, &version) != nil {
			r.fail("invalid version")
		}
	}
	if err = r.done(); err != nil {
		return nil, err
	}
	if id != "" && version != oldVersion {
		return nil, Conflict("habit changed; reload before editing")
	}
	if h.Title == "" || len(h.Weekdays) == 0 || len(h.Weekdays) > 7 {
		return nil, Invalid("title and weekdays are required")
	}
	seen := map[int]bool{}
	for _, d := range h.Weekdays {
		if d < 1 || d > 7 || seen[d] {
			return nil, Invalid("weekdays must be unique ISO weekdays 1–7")
		}
		seen[d] = true
	}
	if h.DurationMinutes < 0 || h.DurationMinutes > 720 || h.DayOffset < 0 || h.DayOffset > 1 {
		return nil, Invalid("invalid duration or next-day offset")
	}
	if h.Time != "" {
		if _, err = time.Parse("15:04", h.Time); err != nil {
			return nil, Invalid("time must be HH:MM")
		}
	} else if h.DurationMinutes != 0 || h.DayOffset != 0 || h.ReminderMinutes != nil {
		return nil, Invalid("duration, next-day offset and reminders require a time")
	}
	if h.ReminderMinutes != nil && (*h.ReminderMinutes < 0 || *h.ReminderMinutes > 1440) {
		return nil, Invalid("reminder_minutes must be 0–1440")
	}
	if h.ProjectID != nil {
		p, e := store.Projects.Get(op.ctx, op.q, op.User.ID, *h.ProjectID)
		if e != nil {
			return nil, e
		}
		if p == nil {
			return nil, NotFound("project")
		}
	}
	if id != "" {
		h.EffectiveDate = max(h.EffectiveDate, op.Now.In(op.Loc).AddDate(0, 0, 1).Format(dateLayout))
	}
	h.Version = oldVersion + 1
	h.Timezone = op.User.Timezone
	if id != "" {
		if err = op.checkHabitRuleRecords(h); err != nil {
			return nil, err
		}
	}
	if id == "" {
		if _, err = op.q.ExecContext(op.ctx, "INSERT INTO habits(id,user_id,created_at) VALUES(?,?,?)", h.ID, op.User.ID, op.now()); err != nil {
			return nil, err
		}
	}
	body, _ := json.Marshal(h)
	_, err = op.q.ExecContext(op.ctx, "INSERT INTO habit_rules(habit_id,version,effective_date,body) VALUES(?,?,?,?)", h.ID, h.Version, h.EffectiveDate, string(body))
	if err != nil {
		return nil, err
	}
	op.record("habit-rule", fmt.Sprintf("%s:%d", h.ID, h.Version), nil, body)
	if _, err = op.log("habit.rule", "修改了习惯"+quote(h.Title)); err != nil {
		return nil, err
	}
	return &h, nil
}

// wallInstant resolves a gap forward by minutes and an overlap to its earliest UTC instant.
func wallInstant(date, clock string, loc *time.Location) time.Time {
	wall, _ := time.Parse(dateLayout+" 15:04", date+" "+clock)
	guess := time.Date(wall.Year(), wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), 0, 0, loc)
	offsets := map[int]bool{}
	for _, delta := range []time.Duration{-48 * time.Hour, 0, 48 * time.Hour} {
		_, off := guess.Add(delta).Zone()
		offsets[off] = true
	}
	for n := 0; n <= 180; n++ {
		target := wall.Add(time.Duration(n) * time.Minute)
		var earliest time.Time
		for off := range offsets {
			candidate := target.Add(-time.Duration(off) * time.Second)
			if candidate.In(loc).Format(dateLayout+" 15:04") == target.Format(dateLayout+" 15:04") && (earliest.IsZero() || candidate.Before(earliest)) {
				earliest = candidate
			}
		}
		if !earliest.IsZero() {
			return earliest
		}
	}
	return guess
}
func parseHabitRange(from, to string, maxDays int) (time.Time, time.Time, error) {
	a, e := time.Parse(dateLayout, from)
	if e != nil {
		return a, a, Invalid("from must be YYYY-MM-DD")
	}
	b, e := time.Parse(dateLayout, to)
	if e != nil || b.Before(a) || b.Sub(a) > time.Duration(maxDays-1)*24*time.Hour {
		return a, b, Invalid("invalid date range (maximum %d days)", maxDays)
	}
	return a, b, nil
}
func (op *Op) HabitOccurrences(from, to string) ([]api.HabitOccurrence, error) {
	return op.habitOccurrences(from, to, 93)
}
func (op *Op) habitOccurrences(from, to string, maxDays int) ([]api.HabitOccurrence, error) {
	a, b, err := parseHabitRange(from, to, maxDays)
	if err != nil {
		return nil, err
	}
	rules, err := op.habitRules()
	if err != nil {
		return nil, err
	}
	group := map[string][]api.Habit{}
	for _, h := range rules {
		group[h.ID] = append(group[h.ID], h)
	}
	records := map[string]habitRecord{}
	rows, err := op.q.QueryContext(op.ctx, `SELECT r.habit_id,r.date,r.body FROM habit_records r JOIN habits h ON r.habit_id=h.id WHERE h.user_id=? AND r.date>=? AND r.date<=?`, op.User.ID, from, to)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, date, body string
		if err = rows.Scan(&id, &date, &body); err != nil {
			rows.Close()
			return nil, err
		}
		var record habitRecord
		if err = json.Unmarshal([]byte(body), &record); err != nil {
			rows.Close()
			return nil, err
		}
		records[id+date] = record
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	// Deleted projects are treated as unfiled without deleting historical rules or records.
	projects, err := store.Projects.List(op.ctx, op.q, op.User.ID, "")
	if err != nil {
		return nil, err
	}
	valid := map[string]bool{}
	for _, p := range projects {
		valid[p.ID] = true
	}
	out := []api.HabitOccurrence{}
	for day := a; !day.After(b); day = day.AddDate(0, 0, 1) {
		date := day.Format(dateLayout)
		for id, list := range group {
			var chosen *api.Habit
			for i := range list {
				if list[i].EffectiveDate <= date {
					chosen = &list[i]
				}
			}
			if chosen == nil || chosen.Paused {
				continue
			}
			h := *chosen
			weekday := int(day.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			if !slices.Contains(h.Weekdays, weekday) {
				continue
			}
			o := api.HabitOccurrence{HabitID: id, Date: date, PlanDate: date, Title: h.Title, Notes: h.Notes, ProjectID: h.ProjectID, Timezone: h.Timezone, Time: h.Time, DayOffset: h.DayOffset, DurationMinutes: h.DurationMinutes, Status: "open"}
			if o.ProjectID != nil && !valid[*o.ProjectID] {
				o.ProjectID = nil
			}
			physical := day.AddDate(0, 0, h.DayOffset).Format(dateLayout)
			loc := loadLocation(h.Timezone)
			if h.Time != "" {
				s := wallInstant(physical, h.Time, loc)
				start := store.FormatTime(s)
				end := store.FormatTime(s.Add(time.Duration(h.DurationMinutes) * time.Minute))
				o.ScheduledAt = &start
				o.EndAt = &end
			}
			record := records[id+date]
			if record.ScheduledAt != nil && (o.ScheduledAt == nil || *record.ScheduledAt != *o.ScheduledAt) {
				s := store.ParseTime(*record.ScheduledAt)
				o.ScheduledAt = record.ScheduledAt
				end := store.FormatTime(s.Add(time.Duration(h.DurationMinutes) * time.Minute))
				o.EndAt = &end
				physical = s.In(loc).Format(dateLayout)
				o.PlanDate = physical
			}
			today := op.Now.In(loc).Format(dateLayout)
			if physical < today {
				o.Status = "missed"
			} else if o.PlanDate > today {
				o.Status = "future"
			}
			if record.Status == "done" || record.Status == "skipped" {
				o.Status = record.Status
			}
			o.CompletedAt = record.CompletedAt
			o.SkipReason = record.SkipReason
			o.Actor = record.Actor
			if h.ReminderMinutes != nil && o.ScheduledAt != nil && (o.Status == "open" || o.Status == "future") {
				at := store.FormatTime(store.ParseTime(*o.ScheduledAt).Add(-time.Duration(*h.ReminderMinutes) * time.Minute))
				o.RemindAt = &at
			}
			out = append(out, o)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return habitBefore(out[i], out[j]) })
	return out, nil
}
func (op *Op) UpdateHabitOccurrence(id, date string, f Fields) (*api.HabitOccurrence, error) {
	occurrences, err := op.HabitOccurrences(date, date)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(occurrences, func(o api.HabitOccurrence) bool { return o.HabitID == id })
	if i < 0 {
		return nil, NotFound("habit occurrence")
	}
	o := occurrences[i]
	rec := habitRecord{Status: "open", CompletedAt: o.CompletedAt, SkipReason: o.SkipReason, Actor: op.ID.Actor}
	var body string
	err = op.q.QueryRowContext(op.ctx, "SELECT body FROM habit_records WHERE habit_id=? AND date=?", id, date).Scan(&body)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		if err = json.Unmarshal([]byte(body), &rec); err != nil {
			return nil, err
		}
	}
	r := newReader(f)
	r.str("status", &rec.Status, 20)
	r.str("skip_reason", &rec.SkipReason, 500)
	for key, dst := range map[string]**string{"completed_at": &rec.CompletedAt, "scheduled_at": &rec.ScheduledAt} {
		if raw, ok := r.raw(key); ok {
			if err = json.Unmarshal(raw, dst); err != nil {
				r.fail("invalid %s", key)
			}
		}
	}
	if err = r.done(); err != nil {
		return nil, err
	}
	if rec.Status != "open" && rec.Status != "done" && rec.Status != "skipped" {
		return nil, Invalid("invalid status")
	}
	if rec.ScheduledAt != nil {
		at, e := time.Parse(time.RFC3339, *rec.ScheduledAt)
		base, _ := time.Parse(dateLayout, date)
		if e != nil || at.In(loadLocation(o.Timezone)).Format(dateLayout) < date || at.In(loadLocation(o.Timezone)).Format(dateLayout) > base.AddDate(0, 0, 7).Format(dateLayout) {
			return nil, Invalid("move must stay within the occurrence date and following seven days")
		}
		v := store.FormatTime(at)
		rec.ScheduledAt = &v
		if o.Time != "" {
			d, _ := time.Parse(dateLayout, date)
			ruleAt := wallInstant(d.AddDate(0, 0, o.DayOffset).Format(dateLayout), o.Time, loadLocation(o.Timezone))
			if at.Equal(ruleAt) {
				rec.ScheduledAt = nil
			}
		}
	}
	planDate := o.Date
	if rec.ScheduledAt != nil {
		if at, e := time.Parse(time.RFC3339, *rec.ScheduledAt); e == nil {
			planDate = at.In(loadLocation(o.Timezone)).Format(dateLayout)
		}
	}
	if (rec.Status == "done" || rec.Status == "skipped") && planDate > op.Now.In(loadLocation(o.Timezone)).Format(dateLayout) {
		return nil, Invalid("cannot record a future habit")
	}
	if rec.Status == "done" {
		if rec.CompletedAt == nil {
			at := op.now()
			rec.CompletedAt = &at
		}
		at, e := time.Parse(time.RFC3339, *rec.CompletedAt)
		if e != nil || at.After(op.Now) || at.In(loadLocation(o.Timezone)).Format(dateLayout) < date {
			return nil, Invalid("completion must be between the occurrence date and now")
		}
		v := store.FormatTime(at)
		rec.CompletedAt = &v
	} else {
		rec.CompletedAt = nil
	}

	if rec.Status != "skipped" {
		rec.SkipReason = ""
	}
	rec.Actor = op.ID.Actor
	var before json.RawMessage
	if body != "" {
		before = json.RawMessage(body)
	}

	var data json.RawMessage
	if rec.Status == "open" && rec.ScheduledAt == nil {
		_, err = op.q.ExecContext(op.ctx, "DELETE FROM habit_records WHERE habit_id=? AND date=?", id, date)
	} else {
		data, _ = json.Marshal(rec)
		_, err = op.q.ExecContext(op.ctx, "INSERT INTO habit_records(habit_id,date,body) VALUES(?,?,?) ON CONFLICT(habit_id,date) DO UPDATE SET body=excluded.body", id, date, string(data))
	}
	if err != nil {
		return nil, err
	}
	if !sameState(before, data) {
		op.record("habit-record", id+":"+date, before, data)
		if _, err = op.log("habit.record", "更新了习惯记录"+quote(o.Title)+" · "+date); err != nil {
			return nil, err
		}
	}

	list, err := op.HabitOccurrences(date, date)
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		if v.HabitID == id {
			return &v, nil
		}
	}
	return nil, NotFound("habit occurrence")
}
func (op *Op) todayHabits() ([]api.HabitOccurrence, error) {
	yesterday := op.Now.In(op.Loc).AddDate(0, 0, -7).Format(dateLayout)
	all, err := op.HabitOccurrences(yesterday, op.today())
	if err != nil {
		return nil, err
	}
	out := []api.HabitOccurrence{}
	for _, o := range all {
		if o.PlanDate == op.today() || (o.DayOffset == 1 && o.PlanDate == o.Date && o.ScheduledAt != nil && store.ParseTime(*o.ScheduledAt).In(op.Loc).Format(dateLayout) == op.today()) {
			out = append(out, o)
		}
	}
	return out, nil
}
func (op *Op) habitEntries(from, to time.Time) ([]entry, error) {
	// Include explicitly moved occurrences and yesterday's next-day point.
	list, err := op.habitOccurrences(from.In(op.Loc).AddDate(0, 0, -8).Format(dateLayout), to.In(op.Loc).Format(dateLayout), 740)
	if err != nil {
		return nil, err
	}
	out := []entry{}
	for _, o := range list {
		if o.ScheduledAt == nil || o.Status == "skipped" {
			continue
		}
		s, e := store.ParseTime(*o.ScheduledAt), store.ParseTime(*o.EndAt)
		if !overlaps(s, e, from, to) {
			continue
		}
		id, date := o.HabitID, o.Date
		ev := api.Event{ID: fmt.Sprintf("habit:%s:%s", id, date), Title: o.Title, Notes: o.Notes, ProjectID: o.ProjectID, Start: o.ScheduledAt, End: o.EndAt, Status: "confirmed", Readonly: true, CreatedBy: api.Actor{Kind: "system", Name: ""}, UpdatedAt: op.now(), HabitID: &id, HabitDate: &date}
		out = append(out, entry{ev: ev, start: s, end: e, block: true})
	}
	return out, nil
}

// Changing the account zone creates future versions; old dates keep their wall-time meaning.
func (op *Op) moveHabitTimezone(zone string) error {
	all, err := op.Habits()
	if err != nil {
		return err
	}
	for _, h := range all {
		h.Timezone = zone
		h.Version++
		h.EffectiveDate = max(h.EffectiveDate, op.Now.In(loadLocation(zone)).AddDate(0, 0, 1).Format(dateLayout), op.Now.In(op.Loc).AddDate(0, 0, 1).Format(dateLayout))
		body, _ := json.Marshal(h)
		if _, err = op.q.ExecContext(op.ctx, "INSERT INTO habit_rules(habit_id,version,effective_date,body) VALUES(?,?,?,?)", h.ID, h.Version, h.EffectiveDate, string(body)); err != nil {
			return err
		}
	}
	return nil
}

// PlannedHabitOccurrences selects a display window without losing occurrences moved into it.
func (op *Op) PlannedHabitOccurrences(from, to string) ([]api.HabitOccurrence, error) {
	a, _, err := parseHabitRange(from, to, 93)
	if err != nil {
		return nil, err
	}
	rows, err := op.habitOccurrences(a.AddDate(0, 0, -7).Format(dateLayout), to, 100)
	if err != nil {
		return nil, err
	}
	out := []api.HabitOccurrence{}
	for _, o := range rows {
		if o.PlanDate >= from && o.PlanDate <= to {
			out = append(out, o)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return habitBefore(out[i], out[j]) })
	return out, nil
}

func habitBefore(a, b api.HabitOccurrence) bool {
	if a.PlanDate != b.PlanDate {
		return a.PlanDate < b.PlanDate
	}
	if a.ScheduledAt != nil && b.ScheduledAt != nil && *a.ScheduledAt != *b.ScheduledAt {
		return *a.ScheduledAt < *b.ScheduledAt
	}
	if (a.ScheduledAt == nil) != (b.ScheduledAt == nil) {
		return a.ScheduledAt != nil
	}
	if a.Date != b.Date {
		return a.Date < b.Date
	}
	return a.HabitID < b.HabitID
}

// Refuse a rule edit that would hide an existing record. The user can undo its
// recorded move first; no data is silently removed or made inaccessible.
func (op *Op) checkHabitRuleRecords(h api.Habit) error {
	boundary, _ := time.Parse(dateLayout, h.EffectiveDate)
	rows, err := op.q.QueryContext(op.ctx, "SELECT date,body FROM habit_records WHERE habit_id=? AND date>=?", h.ID, boundary.AddDate(0, 0, -7).Format(dateLayout))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var date, body string
		if err = rows.Scan(&date, &body); err != nil {
			return err
		}
		var record habitRecord
		if err = json.Unmarshal([]byte(body), &record); err != nil {
			return err
		}
		d, _ := time.Parse(dateLayout, date)
		weekday := int(d.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		invalid := date >= h.EffectiveDate && (h.Paused || !slices.Contains(h.Weekdays, weekday))
		if h.Paused && record.Status == "open" && record.ScheduledAt != nil && store.ParseTime(*record.ScheduledAt).In(loadLocation(h.Timezone)).Format(dateLayout) >= h.EffectiveDate {
			invalid = true
		}
		if invalid {
			return &Error{Status: 409, Code: "habit_records_depend", Message: "Undo the habit's future moves or records in Activity before changing this repeat rule"}
		}
	}
	return rows.Err()
}
