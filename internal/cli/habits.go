package cli

import (
	"encoding/json"
	"github.com/Lewin671/keduly/internal/api"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (a *app) habitRead(args []string, history bool) error {
	fs := a.flags("habit list", false)
	if history {
		fs = a.flags("habit log", false)
	}
	from := fs.String("from", "", "first date")
	to := fs.String("to", "", "last date")
	pos, err := a.parse(fs, args, "[HABIT_ID] [--from D --to D]")
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return usagef("expected at most one habit ID")
	}
	path := "/habits"
	q := url.Values{}
	if history {
		if _, err = a.zone(); err != nil {
			return err
		}
		d := a.env.Now().In(a.loc).Format("2006-01-02")
		if *from == "" {
			*from = d
		}
		if *to == "" {
			*to = d
		}
		path = "/habit-occurrences"
		q.Set("from", *from)
		q.Set("to", *to)
		q.Set("basis", "plan")
		if len(pos) > 0 {
			id, e := a.habitID(pos[0])
			if e != nil {
				return e
			}
			pos[0] = id
			q.Set("habit_id", id)
		}
	}
	var response map[string]json.RawMessage
	if printed, _, e := a.send(http.MethodGet, path, q, nil, &response); e != nil || printed {
		return e
	}
	if history {
		var rows []api.HabitOccurrence
		_ = json.Unmarshal(response["occurrences"], &rows)
		for _, o := range rows {
			if len(pos) > 0 && !strings.HasPrefix(o.HabitID, pos[0]) {
				continue
			}
			a.printf("%s %s %s %s", o.HabitID, o.PlanDate, o.Status, o.Title)
			if o.Date != o.PlanDate {
				a.printf("（记录日期 %s）", o.Date)
			}
			a.printf("\n")
		}
	} else {
		var rows []api.Habit
		_ = json.Unmarshal(response["habits"], &rows)
		for _, h := range rows {
			a.printf("%s  %s  %s  paused=%t version=%d\n", h.ID, h.Title, h.Time, h.Paused, h.Version)
		}
	}
	return nil
}
func (a *app) habitWrite(args []string, cmd string) error {
	fs := a.flags("habit "+cmd, true)
	title := fs.String("title", "", "title")
	notes := fs.String("notes", "", "notes; empty clears")
	clock := fs.String("time", "", "HH:MM or none")
	days := fs.String("weekdays", "", "comma-separated ISO weekdays 1–7")
	duration := fs.Int("duration", 0, "duration minutes")
	offset := fs.Int("day-offset", 0, "0 or 1")
	reminder := fs.Int("reminder", -1, "minutes before; -1 disables")
	project := fs.String("project", "", "project name or ID, none clears")
	date := fs.String("date", "", "logical occurrence date")
	at := fs.String("at", "", "actual completion RFC3339")
	move := fs.String("to", "", "new scheduled RFC3339 instant")
	pos, err := a.parseN(fs, args, 1, "TITLE|HABIT_ID [flags]")
	if err != nil {
		return err
	}
	body := map[string]any{}
	path := "/habits"
	method := http.MethodPost
	if cmd == "add" {
		body["title"] = pos[0]
	} else {
		var r struct {
			Habits []api.Habit `json:"habits"`
		}
		data, _, e := a.request(http.MethodGet, "/habits", nil, nil)
		if e != nil {
			return e
		}
		if e = json.Unmarshal(data, &r); e != nil {
			return e
		}
		var found *api.Habit
		for _, h := range r.Habits {
			if h.ID == pos[0] || (len(pos[0]) >= 4 && strings.HasPrefix(h.ID, pos[0])) {
				if found != nil {
					return usagef("ambiguous habit ID")
				}
				x := h
				found = &x
			}
		}
		if found == nil {
			return usagef("habit not found")
		}
		path += "/" + found.ID
		method = http.MethodPatch
		body["version"] = found.Version
		switch cmd {
		case "pause", "resume":
			body["paused"] = cmd == "pause"
		case "done", "skip", "undo", "move":
			delete(body, "version")
			if _, err = a.zone(); err != nil {
				return err
			}
			if *date == "" {
				*date = a.env.Now().In(a.loc).Format("2006-01-02")
			}
			path += "/occurrences/" + url.PathEscape(*date)
			if cmd == "move" {
				if *move == "" {
					return usagef("--to RFC3339 is required")
				}
				body["scheduled_at"] = *move
			} else {
				status := map[string]string{"done": "done", "skip": "skipped", "undo": "open"}[cmd]
				body["status"] = status
				if *at != "" {
					body["completed_at"] = *at
				}
			}
		}
	}
	if given(fs, "notes") {
		body["notes"] = *notes
	}
	if given(fs, "title") {
		body["title"] = *title
	}
	if given(fs, "time") {
		v := *clock
		if v == "none" {
			body["duration_minutes"] = 0
			body["day_offset"] = 0
			body["reminder_minutes"] = nil
			v = ""
		}
		body["time"] = v
	}
	if given(fs, "weekdays") {
		list := []int{}
		for _, s := range strings.Split(*days, ",") {
			n, e := strconv.Atoi(strings.TrimSpace(s))
			if e != nil {
				return usagef("invalid weekdays")
			}
			list = append(list, n)
		}
		body["weekdays"] = list
	}
	if given(fs, "duration") {
		body["duration_minutes"] = *duration
	}
	if given(fs, "day-offset") {
		body["day_offset"] = *offset
	}
	if given(fs, "reminder") {
		if *reminder < 0 {
			body["reminder_minutes"] = nil
		} else {
			body["reminder_minutes"] = *reminder
		}
	}
	if given(fs, "project") {
		var id any
		var e error
		if *project != "none" {
			id, e = a.projectID(*project)
		}
		if e != nil {
			return e
		}
		body["project_id"] = id
	}
	var r map[string]json.RawMessage
	if printed, _, e := a.send(method, path, nil, body, &r); e != nil || printed {
		return e
	}
	data, _ := json.MarshalIndent(r, "", "  ")
	a.printf("%s\n", data)
	return nil
}

func (a *app) printHabit(h api.HabitOccurrence) {
	at := h.Time
	if h.ScheduledAt != nil {
		at = localTime(*h.ScheduledAt, a.loc).Format("2006-01-02 15:04")
	}
	a.printf("  ↻ %s %s [%s]\n", at, h.Title, h.Status)
}

func (a *app) habitID(value string) (string, error) {
	var r struct {
		Habits []api.Habit `json:"habits"`
	}
	data, _, err := a.request(http.MethodGet, "/habits", nil, nil)
	if err != nil {
		return "", err
	}
	if err = json.Unmarshal(data, &r); err != nil {
		return "", err
	}
	id := ""
	for _, h := range r.Habits {
		if h.ID == value || (len(value) >= 4 && strings.HasPrefix(h.ID, value)) {
			if id != "" {
				return "", usagef("ambiguous habit ID")
			}
			id = h.ID
		}
	}
	if id == "" {
		return "", usagef("habit not found")
	}
	return id, nil
}
