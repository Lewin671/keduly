package cli

import (
	"flag"
	"net/http"
	"time"

	"github.com/Lewin671/keduly/internal/api"
)

type eventResponse struct {
	Event      api.Event       `json:"event"`
	Suggestion *api.Suggestion `json:"suggestion"`
}

func (a *app) showEvent(verb string, e api.Event) error {
	if _, err := a.zone(); err != nil {
		return err
	}
	a.printf("%s%s\n%s  %s\n", verb, a.dryNote(), dateLabel(a.eventDay(e)), a.eventLine(e, a.projectNames()))
	return nil
}

// eventBody builds the fields of a new event from the command line.
func (a *app) eventBody(title, start, end, duration, project, notes, location string, allDay bool) (map[string]any, error) {
	body := map[string]any{"title": title}
	if allDay {
		loc, err := a.zone()
		if err != nil {
			return nil, err
		}
		if start == "" || duration != "" {
			return nil, usagef("--all-day takes --start DATE and optionally --end DATE")
		}
		first, err := parseDate(start, a.env.Now(), loc)
		if err != nil {
			return nil, err
		}
		last := first
		if end != "" {
			if last, err = parseDate(end, a.env.Now(), loc); err != nil {
				return nil, err
			}
		}
		body["all_day"], body["start_date"], body["end_date"] = true, first, last
	} else {
		from, to, err := a.slot(start, end, duration)
		if err != nil {
			return nil, err
		}
		body["start"], body["end"] = stamp(from), stamp(to)
	}
	if project != "" {
		id, err := a.projectID(project)
		if err != nil {
			return nil, err
		}
		body["project_id"] = id
	}
	if notes != "" {
		body["notes"] = notes
	}
	if location != "" {
		body["location"] = location
	}
	return body, nil
}

func (a *app) eventAdd(args []string) error {
	fs := a.flags("event add", true)
	start := fs.String("start", "", "start time (a date with --all-day)")
	end := fs.String("end", "", "end time (with --all-day: the last day, default the first)")
	duration := fs.String("duration", "", "length, e.g. 1h")
	allDay := fs.Bool("all-day", false, "an all-day event")
	project := fs.String("project", "", "project name or ID")
	notes := fs.String("notes", "", "notes")
	location := fs.String("location", "", "location")
	pos, err := a.parseN(fs, args, 1, "TITLE (--start T (--end T | --duration 1h) | --all-day --start D [--end D]) [--project P] [--notes TEXT] [--location L]")
	if err != nil {
		return err
	}
	body, err := a.eventBody(pos[0], *start, *end, *duration, *project, *notes, *location, *allDay)
	if err != nil {
		return err
	}
	var resp eventResponse
	if printed, _, err := a.send(http.MethodPost, "/events", nil, body, &resp); err != nil || printed {
		return err
	}
	return a.showEvent("已新建日程", resp.Event)
}

func (a *app) eventList(args []string) error {
	fs := a.flags("event list", false)
	from := fs.String("from", "today", "first day")
	to := fs.String("to", "", "last day (default: 7 days after --from)")
	if _, err := a.parseN(fs, args, 0, "[--from D] [--to D]"); err != nil {
		return err
	}
	loc, err := a.zone()
	if err != nil {
		return err
	}
	first, err := parseDate(*from, a.env.Now(), loc)
	if err != nil {
		return err
	}
	start, _ := time.ParseInLocation(dateLayout, first, loc)
	end := start.AddDate(0, 0, 8)
	if *to != "" {
		last, err := parseDate(*to, a.env.Now(), loc)
		if err != nil {
			return err
		}
		lastDay, _ := time.ParseInLocation(dateLayout, last, loc)
		end = lastDay.AddDate(0, 0, 1)
	}
	var resp struct {
		Events []api.Event `json:"events"`
	}
	q := map[string][]string{"from": {stamp(start)}, "to": {stamp(end)}}
	if printed, _, err := a.send(http.MethodGet, "/events", q, nil, &resp); err != nil || printed {
		return err
	}
	projects := a.projectNames()
	day := ""
	for _, e := range resp.Events {
		if d := a.eventDay(e); d != day {
			day = d
			a.printf("%s\n", dateLabel(day))
		}
		a.printf("  %s\n", a.eventLine(e, projects))
	}
	if len(resp.Events) == 0 {
		a.printf("（没有日程）\n")
	}
	return nil
}

// eventEdit serves both `event edit` and `event move`, which differ in name only.
func (a *app) eventEdit(name string) func([]string) error {
	return func(args []string) error { return a.editEvent(a.flags("event "+name, true), args) }
}

func (a *app) editEvent(fs *flag.FlagSet, args []string) error {
	title := fs.String("title", "", "new title")
	start := fs.String("start", "", "new start time (a date for an all-day event); alone, it keeps the length")
	end := fs.String("end", "", "new end time (for an all-day event: the last day)")
	duration := fs.String("duration", "", "new length, e.g. 1h; timed events only")
	allDay := fs.Bool("all-day", false, "make it an all-day event, on the day it starts unless --start and --end give dates")
	timed := fs.Bool("timed", false, "make an all-day event a timed one; needs --start with --end or --duration")
	project := fs.String("project", "", `project name or ID; "none" for no project`)
	notes := fs.String("notes", "", "notes")
	location := fs.String("location", "", "location")
	pos, err := a.parseN(fs, args, 1, "ID [--title T] [--start T] [--end T | --duration 1h] [--all-day | --timed] [--project P] [--notes TEXT] [--location L]")
	if err != nil {
		return err
	}
	id, err := a.eventID(pos[0])
	if err != nil {
		return err
	}
	body := map[string]any{}
	if given(fs, "title") {
		body["title"] = *title
	}
	if given(fs, "notes") {
		body["notes"] = *notes
	}
	if given(fs, "location") {
		body["location"] = *location
	}
	if given(fs, "project") {
		body["project_id"] = nil
		if *project != "none" {
			if body["project_id"], err = a.projectID(*project); err != nil {
				return err
			}
		}
	}
	if *allDay && *timed {
		return usagef("give only one of --all-day and --timed")
	}
	if err := a.eventWhen(id, *start, *end, *duration, *allDay, *timed, body); err != nil {
		return err
	}
	if len(body) == 0 {
		return usagef("nothing to change; give at least one flag")
	}
	var resp eventResponse
	if printed, _, err := a.send(http.MethodPatch, "/events/"+id, nil, body, &resp); err != nil || printed {
		return err
	}
	return a.showEvent("已修改日程", resp.Event)
}

// eventWhen fills the fields that place an edited event in time: dates for an
// all-day event, instants for a timed one. allDay and timed switch between the two.
func (a *app) eventWhen(id, start, end, duration string, allDay, timed bool, body map[string]any) error {
	if start == "" && end == "" && duration == "" && !allDay && !timed {
		return nil
	}
	var current eventResponse
	if err := a.get("/events/"+id, nil, &current); err != nil {
		return err
	}
	switch ev := current.Event; {
	case allDay || (ev.AllDay && !timed):
		return a.allDaySpan(ev, start, end, duration, body)
	case ev.AllDay:
		from, to, err := a.slot(start, end, duration)
		if err != nil {
			return err
		}
		body["all_day"], body["start"], body["end"] = false, stamp(from), stamp(to)
		return nil
	}
	return a.movedSlot(id, start, end, duration, body)
}

// allDaySpan fills the dates of an event that is or becomes all-day. A new
// first day alone keeps the number of days; a timed event becomes one day.
func (a *app) allDaySpan(ev api.Event, start, end, duration string, body map[string]any) error {
	if duration != "" {
		return usagef("an all-day event takes --start DATE and --end DATE, not --duration; add --timed to give it a time")
	}
	loc, err := a.zone()
	if err != nil {
		return err
	}
	var first, last time.Time
	switch {
	case ev.AllDay && ev.StartDate != nil && ev.EndDate != nil:
		first, _ = time.Parse(dateLayout, *ev.StartDate)
		last, _ = time.Parse(dateLayout, *ev.EndDate)
	case ev.Start != nil:
		first, _ = time.Parse(dateLayout, localTime(*ev.Start, loc).Format(dateLayout))
		last = first
	}
	if start != "" {
		day, err := parseDate(start, a.env.Now(), loc)
		if err != nil {
			return err
		}
		length := last.Sub(first)
		first, _ = time.Parse(dateLayout, day)
		last = first.Add(length)
	}
	if end != "" {
		day, err := parseDate(end, a.env.Now(), loc)
		if err != nil {
			return err
		}
		last, _ = time.Parse(dateLayout, day)
	}
	body["all_day"], body["start_date"], body["end_date"] = true, first.Format(dateLayout), last.Format(dateLayout)
	return nil
}

// movedSlot fills start and end for a move. The start alone keeps the event's
// length; a duration alone keeps its start.
func (a *app) movedSlot(id, start, end, duration string, body map[string]any) error {
	if start == "" && end == "" && duration == "" {
		return nil
	}
	if end != "" && duration != "" {
		return usagef("give only one of --end and --duration")
	}
	loc, err := a.zone()
	if err != nil {
		return err
	}
	var from time.Time
	if start != "" {
		if from, err = parseTime(start, a.env.Now(), loc); err != nil {
			return err
		}
		body["start"] = stamp(from)
	} else {
		var current eventResponse
		if err := a.get("/events/"+id, nil, &current); err != nil {
			return err
		}
		if current.Event.Start == nil {
			return usagef("an all-day event has no start time to keep; this command moves timed events")
		}
		from = localTime(*current.Event.Start, loc)
	}
	switch {
	case end != "":
		to, err := parseTime(end, a.env.Now(), loc)
		if err != nil {
			return err
		}
		body["end"] = stamp(to)
	case duration != "":
		minutes, err := parseMinutes(duration)
		if err != nil {
			return err
		}
		body["start"] = stamp(from)
		body["end"] = stamp(from.Add(time.Duration(minutes) * time.Minute))
	}
	return nil
}

func (a *app) eventRemove(args []string) error {
	fs := a.flags("event rm", true)
	pos, err := a.parseN(fs, args, 1, "ID")
	if err != nil {
		return err
	}
	id, err := a.eventID(pos[0])
	if err != nil {
		return err
	}
	var resp eventResponse
	printed, status, err := a.send(http.MethodDelete, "/events/"+id, nil, nil, &resp)
	if err != nil || printed {
		return err
	}
	a.removed("日程", short(id), status, resp.Suggestion)
	return nil
}
