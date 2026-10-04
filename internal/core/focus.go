package core

import (
	"sort"
	"time"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/store"
)

// The focus timer is a pomodoro timer. Its state is not stored: it follows from
// the user's latest session and the clock. A session carries the end it was
// planned to have, so a tomato completes without anything running.

const (
	focusWork = "work"
	focusRest = "rest"
	// A tomato given up sooner than this was a slip and leaves no record.
	focusSlip = time.Minute
	// focusSeconds is the length of a session in SQL.
	focusSeconds = "(strftime('%s', end_at) - strftime('%s', start_at))"
	// focusFull is true in SQL for a session that ran its planned length.
	focusFull = "(" + focusSeconds + " >= planned_minutes * 60)"
)

func roundMinutes(seconds int64) int { return int((seconds + 30) / 60) }

func sessionSeconds(s *store.FocusSession) int64 {
	return int64(store.ParseTime(s.EndAt).Sub(store.ParseTime(s.StartAt)).Seconds())
}

// tomato reports whether a work session ran its full length and has ended.
func (op *Op) tomato(s *store.FocusSession) bool {
	return s.Kind == focusWork && s.EndAt <= op.now() && sessionSeconds(s) >= int64(s.PlannedMinutes)*60
}

func (op *Op) latestSession() (*store.FocusSession, error) {
	rows, err := store.FocusSessions.Query(op.ctx, op.q,
		store.FocusSessions.SelectSQL()+" WHERE user_id = ? ORDER BY start_at DESC, rowid DESC LIMIT 1", op.User.ID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// itemFocus loads, per item, the tomatoes and seconds of its ended work sessions.
func (op *Op) itemFocus() (map[string]api.ItemFocus, error) {
	rows, err := op.q.QueryContext(op.ctx, `SELECT item_id, sum(`+focusFull+`), sum(`+focusSeconds+`)
		FROM focus_sessions WHERE user_id = ? AND kind = 'work' AND item_id IS NOT NULL AND end_at <= ?
		GROUP BY item_id`, op.User.ID, op.now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]api.ItemFocus{}
	for rows.Next() {
		var id string
		var tomatoes int
		var seconds int64
		if err := rows.Scan(&id, &tomatoes, &seconds); err != nil {
			return nil, err
		}
		out[id] = api.ItemFocus{Tomatoes: tomatoes, Minutes: roundMinutes(seconds)}
	}
	return out, rows.Err()
}

// renderSessions fills in each session's project and current title from its item.
func (op *Op) renderSessions(rows []*store.FocusSession) ([]api.FocusSession, error) {
	items := map[string]*store.Item{}
	out := make([]api.FocusSession, 0, len(rows))
	for _, s := range rows {
		js := api.FocusSession{
			ID: s.ID, Kind: s.Kind, ItemID: s.ItemID, Title: s.Title, Start: s.StartAt, End: s.EndAt,
			PlannedMinutes: s.PlannedMinutes,
			Completed:      s.EndAt <= op.now() && sessionSeconds(s) >= int64(s.PlannedMinutes)*60,
			CreatedBy:      api.Actor{Kind: s.CreatedByKind, Name: s.CreatedByName},
		}
		if s.ItemID != nil {
			it, seen := items[*s.ItemID]
			if !seen {
				var err error
				if it, err = store.Items.Get(op.ctx, op.q, op.User.ID, *s.ItemID); err != nil {
					return nil, err
				}
				items[*s.ItemID] = it
			}
			if it != nil {
				js.ProjectID, js.Title = it.ProjectID, it.Title
			}
		}
		out = append(out, js)
	}
	return out, nil
}

// workOn returns the work sessions that started within [from, to) and have ended.
func (op *Op) workOn(from, to time.Time) ([]*store.FocusSession, error) {
	return store.FocusSessions.List(op.ctx, op.q, op.User.ID,
		"kind = 'work' AND start_at >= ? AND start_at < ? AND end_at <= ?",
		store.FormatTime(from), store.FormatTime(to), op.now())
}

// Focus reports the state of the timer.
func (op *Op) Focus() (*api.Focus, error) {
	u := op.User
	out := &api.Focus{State: "idle", RoundSize: u.RoundSize, RestMinutes: u.RestMinutes, Now: op.now()}
	dayFrom, dayTo := op.localDayRange(op.today())
	today, err := op.workOn(dayFrom, dayTo)
	if err != nil {
		return nil, err
	}
	var seconds int64
	for _, s := range today {
		seconds += sessionSeconds(s)
		if op.tomato(s) {
			out.TomatoesToday++
		}
	}
	out.MinutesToday = roundMinutes(seconds)
	latest, err := op.latestSession()
	if err != nil {
		return nil, err
	}
	switch {
	case latest == nil:
	case latest.EndAt > op.now():
		out.State = latest.Kind
	case op.tomato(latest) && !latest.Answered && latest.StartAt >= store.FormatTime(dayFrom):
		out.State = "over"
	}
	if out.State != "idle" {
		rendered, err := op.renderSessions([]*store.FocusSession{latest})
		if err != nil {
			return nil, err
		}
		out.Session = &rendered[0]
	}
	// A completed round stays full, and earns the long rest, until that rest is over.
	roundComplete := out.TomatoesToday > 0 && out.TomatoesToday%u.RoundSize == 0
	out.RoundDone = out.TomatoesToday % u.RoundSize
	if roundComplete {
		out.RestMinutes = u.LongRestMinutes
		if out.State == "over" || out.State == focusRest {
			out.RoundDone = u.RoundSize
		}
	}
	return out, nil
}

// settleFocus ends whatever the timer is doing: it gives up the running
// tomato, ends the running rest, or answers a tomato that ran out.
func (op *Op) settleFocus() error {
	latest, err := op.latestSession()
	if err != nil || latest == nil || (latest.Answered && latest.EndAt <= op.now()) {
		return err
	}
	if latest.EndAt > op.now() {
		if latest.Kind == focusWork && op.Now.Sub(store.ParseTime(latest.StartAt)) < focusSlip {
			return store.FocusSessions.Delete(op.ctx, op.q, op.User.ID, latest.ID)
		}
		latest.EndAt = op.now()
	}
	latest.Answered = true
	return store.FocusSessions.Put(op.ctx, op.q, latest)
}

func (op *Op) startSession(kind string, minutes int, item *store.Item) error {
	s := &store.FocusSession{
		ID: NewID(), UserID: op.User.ID, Kind: kind, StartAt: op.now(),
		EndAt:          store.FormatTime(op.Now.Add(time.Duration(minutes) * time.Minute)),
		PlannedMinutes: minutes,
		CreatedByKind:  op.ID.Actor.Kind, CreatedByName: op.ID.Actor.Name,
	}
	if item != nil {
		s.ItemID, s.Title = &item.ID, item.Title
	}
	return store.FocusSessions.Put(op.ctx, op.q, s)
}

// StartFocus starts a tomato on an item, or free focus without one.
func (op *Op) StartFocus(f Fields) (*api.Focus, error) {
	var itemID *string
	r := newReader(f)
	r.nullStr("item_id", &itemID, anyID, "an item ID")
	if err := r.done(); err != nil {
		return nil, err
	}
	var item *store.Item
	if itemID != nil {
		var err error
		if item, err = store.Items.Get(op.ctx, op.q, op.User.ID, *itemID); err != nil {
			return nil, err
		}
		if item == nil || item.Status != "open" {
			return nil, Invalid("item_id must name one of your open items")
		}
	}
	if err := op.settleFocus(); err != nil {
		return nil, err
	}
	if err := op.startSession(focusWork, op.User.FocusMinutes, item); err != nil {
		return nil, err
	}
	return op.Focus()
}

// StopFocus gives up the tomato, skips the rest, or answers "nothing" to a tomato that ran out.
func (op *Op) StopFocus(f Fields) (*api.Focus, error) {
	if err := newReader(f).done(); err != nil {
		return nil, err
	}
	if err := op.settleFocus(); err != nil {
		return nil, err
	}
	return op.Focus()
}

// RestFocus starts the rest that follows a tomato.
func (op *Op) RestFocus(f Fields) (*api.Focus, error) {
	if err := newReader(f).done(); err != nil {
		return nil, err
	}
	now, err := op.Focus()
	if err != nil {
		return nil, err
	}
	if now.State == focusWork {
		return nil, Conflict("a tomato is running: stop it before resting")
	}
	if err := op.settleFocus(); err != nil {
		return nil, err
	}
	if err := op.startSession(focusRest, now.RestMinutes, nil); err != nil {
		return nil, err
	}
	return op.Focus()
}

// stopFocusOn ends the timer when it is on the given item: completing an item
// leaves nothing to count down for.
func (op *Op) stopFocusOn(itemID string) error {
	latest, err := op.latestSession()
	if err != nil || latest == nil || latest.Kind != focusWork || latest.ItemID == nil || *latest.ItemID != itemID {
		return err
	}
	return op.settleFocus()
}

// FocusSessions lists the work sessions that started on the given local dates, the running one included.
func (op *Op) FocusSessions(from, to string) ([]api.FocusSession, error) {
	if !validDate(from) || !validDate(to) {
		return nil, Invalid("from and to must be dates (YYYY-MM-DD)")
	}
	start, _ := op.localDayRange(from)
	_, end := op.localDayRange(to)
	if end.Before(start) || end.Sub(start) > 63*24*time.Hour {
		return nil, Invalid("to must not be before from, and at most 62 days after it")
	}
	rows, err := store.FocusSessions.List(op.ctx, op.q, op.User.ID,
		"kind = 'work' AND start_at >= ? AND start_at < ?", store.FormatTime(start), store.FormatTime(end))
	if err != nil {
		return nil, err
	}
	return op.renderSessions(rows)
}

// focusWeekSeconds is the focus time on a project's items since Monday.
func (op *Op) focusWeekSeconds(projectID string) (int64, error) {
	today := dayStart(op.today(), op.Loc)
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	var seconds int64
	err := op.q.QueryRowContext(op.ctx, `SELECT coalesce(sum(strftime('%s', f.end_at) - strftime('%s', f.start_at)), 0)
		FROM focus_sessions f JOIN items i ON i.id = f.item_id
		WHERE f.user_id = ? AND i.project_id = ? AND f.kind = 'work' AND f.start_at >= ? AND f.end_at <= ?`,
		op.User.ID, projectID, store.FormatTime(monday), op.now()).Scan(&seconds)
	return seconds, err
}

// focusTally sums sessions per project; the empty key gathers what has none.
type focusTally struct {
	tomatoes int
	seconds  int64
}

func tallyProjects(by map[string]*focusTally) []api.FocusProject {
	out := make([]api.FocusProject, 0, len(by))
	for id, t := range by {
		p := api.FocusProject{Tomatoes: t.tomatoes, Minutes: roundMinutes(t.seconds)}
		if id != "" {
			id := id
			p.ProjectID = &id
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Minutes != out[j].Minutes {
			return out[i].Minutes > out[j].Minutes
		}
		return out[i].ProjectID != nil && (out[j].ProjectID == nil || *out[i].ProjectID < *out[j].ProjectID)
	})
	return out
}

// FocusStats summarises the last seven days.
func (op *Op) FocusStats() (*api.FocusStats, error) {
	const days = 7
	today := dayStart(op.today(), op.Loc)
	from := today.AddDate(0, 0, -(days - 1))
	rows, err := op.workOn(from, today.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	sessions, err := op.renderSessions(rows)
	if err != nil {
		return nil, err
	}
	out := &api.FocusStats{}
	perDay := make([]map[string]*focusTally, days)
	total := map[string]*focusTally{}
	var allSeconds int64
	add := func(by map[string]*focusTally, key string, full bool, seconds int64) {
		t := by[key]
		if t == nil {
			t = &focusTally{}
			by[key] = t
		}
		t.seconds += seconds
		if full {
			t.tomatoes++
		}
	}
	for i := range perDay {
		perDay[i] = map[string]*focusTally{}
	}
	for i, s := range sessions {
		local := store.ParseTime(s.Start).In(op.Loc)
		day := int(dayStart(local.Format(dateLayout), op.Loc).Sub(from).Hours()+12) / 24
		if day < 0 || day >= days {
			continue
		}
		key := ""
		if s.ProjectID != nil {
			key = *s.ProjectID
		}
		seconds := sessionSeconds(rows[i])
		add(perDay[day], key, s.Completed, seconds)
		add(total, key, s.Completed, seconds)
		allSeconds += seconds
		if s.Completed {
			out.Tomatoes++
		}
	}
	out.Minutes = roundMinutes(allSeconds)
	for i, by := range perDay {
		d := api.FocusDay{Date: from.AddDate(0, 0, i).Format(dateLayout), Projects: tallyProjects(by)}
		var seconds int64
		for _, t := range by {
			d.Tomatoes += t.tomatoes
			seconds += t.seconds
		}
		d.Minutes = roundMinutes(seconds)
		out.Days = append(out.Days, d)
	}
	out.Projects = tallyProjects(total)
	out.Streak, err = op.focusStreak()
	return out, err
}

// focusStreak counts the consecutive days up to today that have a tomato.
// A today without one yet does not break the run.
func (op *Op) focusStreak() (int, error) {
	rows, err := op.q.QueryContext(op.ctx, `SELECT start_at FROM focus_sessions
		WHERE user_id = ? AND kind = 'work' AND end_at <= ? AND `+focusFull+` ORDER BY start_at DESC`,
		op.User.ID, op.now())
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	streak := 0
	expect := dayStart(op.today(), op.Loc)
	for rows.Next() {
		var startAt string
		if err := rows.Scan(&startAt); err != nil {
			return 0, err
		}
		day := dayStart(store.ParseTime(startAt).In(op.Loc).Format(dateLayout), op.Loc)
		switch {
		case day.Equal(expect):
			streak++
			expect = expect.AddDate(0, 0, -1)
		case streak == 0 && day.Equal(expect.AddDate(0, 0, -1)):
			streak++
			expect = expect.AddDate(0, 0, -2)
		case day.Before(expect):
			return streak, nil
		}
	}
	return streak, rows.Err()
}
