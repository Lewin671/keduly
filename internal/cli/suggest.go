package cli

import (
	"maps"
	"net/http"
	"net/url"
	"slices"

	"github.com/Lewin671/keduly/internal/api"
)

type suggestionResponse struct {
	Suggestion api.Suggestion `json:"suggestion"`
}

// propose posts a suggestion and prints it.
func (a *app) propose(body map[string]any) error {
	if a.reason == "" {
		return usagef(`--reason is required: say why, e.g. --reason "due today, first free hour"`)
	}
	var resp suggestionResponse
	if printed, _, err := a.send(http.MethodPost, "/suggestions", nil, body, &resp); err != nil || printed {
		return err
	}
	if _, err := a.zone(); err != nil {
		return err
	}
	a.printf("已提出建议，等待用户确认%s\n%s\n", a.dryNote(), a.suggestionLine(resp.Suggestion))
	return nil
}

func (a *app) suggestSchedule(args []string) error {
	fs := a.flags("suggest schedule", true)
	start := fs.String("start", "", "start time")
	end := fs.String("end", "", "end time")
	duration := fs.String("duration", "", "length, e.g. 1h")
	pos, err := a.parseN(fs, args, 1, `ITEM_ID --start T --duration 1h --reason "..."`)
	if err != nil {
		return err
	}
	from, to, err := a.slot(*start, *end, *duration)
	if err != nil {
		return err
	}
	id, err := a.itemID(pos[0])
	if err != nil {
		return err
	}
	return a.propose(map[string]any{"kind": "schedule_item", "item_id": id, "start": stamp(from), "end": stamp(to)})
}

func (a *app) suggestAddItem(args []string) error {
	fs := a.flags("suggest add-item", true)
	f := a.itemFlags(fs, false)
	start := fs.String("start", "", "start of the proposed time block")
	end := fs.String("end", "", "end of the proposed time block")
	duration := fs.String("duration", "", "length of the proposed time block")
	pos, err := a.parseN(fs, args, 1, `TITLE [item flags] [--start T --duration D] --reason "..."`)
	if err != nil {
		return err
	}
	item, err := f.body(a, "")
	if err != nil {
		return err
	}
	item["title"] = pos[0]
	body := map[string]any{"kind": "create_item", "item": item}
	if *start != "" {
		from, to, err := a.slot(*start, *end, *duration)
		if err != nil {
			return err
		}
		body["start"], body["end"] = stamp(from), stamp(to)
	}
	return a.propose(body)
}

func (a *app) suggestAddEvent(args []string) error {
	fs := a.flags("suggest add-event", true)
	start := fs.String("start", "", "start time (a date with --all-day)")
	end := fs.String("end", "", "end time (a date with --all-day)")
	duration := fs.String("duration", "", "length, e.g. 1h")
	allDay := fs.Bool("all-day", false, "an all-day event")
	project := fs.String("project", "", "project name or ID")
	notes := fs.String("notes", "", "notes")
	location := fs.String("location", "", "location")
	pos, err := a.parseN(fs, args, 1, `TITLE --start T --duration D --reason "..."`)
	if err != nil {
		return err
	}
	event, err := a.eventBody(pos[0], *start, *end, *duration, *project, *notes, *location, *allDay)
	if err != nil {
		return err
	}
	return a.propose(map[string]any{"kind": "create_event", "event": event})
}

func (a *app) suggestMove(args []string) error {
	fs := a.flags("suggest move", true)
	start := fs.String("start", "", "new start time")
	end := fs.String("end", "", "new end time")
	duration := fs.String("duration", "", "new length; the current one is kept when omitted")
	pos, err := a.parseN(fs, args, 1, `EVENT_ID --start T [--duration D] --reason "..."`)
	if err != nil {
		return err
	}
	if *start == "" {
		return usagef("--start is required")
	}
	id, err := a.eventID(pos[0])
	if err != nil {
		return err
	}
	body := map[string]any{"kind": "move_event", "event_id": id}
	if err := a.movedSlot(id, *start, *end, *duration, body); err != nil {
		return err
	}
	return a.propose(body)
}

// suggestDelete proposes deleting an item or an event: what `rm` does on its
// own when the token's deletions need confirmation.
func (a *app) suggestDelete(what string) func([]string) error {
	return func(args []string) error {
		fs := a.flags("suggest delete-"+what, true)
		pos, err := a.parseN(fs, args, 1, `ID --reason "..."`)
		if err != nil {
			return err
		}
		resolve := a.itemID
		if what == "event" {
			resolve = a.eventID
		}
		id, err := resolve(pos[0])
		if err != nil {
			return err
		}
		return a.propose(map[string]any{"kind": "delete_" + what, what + "_id": id})
	}
}

// suggestWithdraw takes back a pending suggestion that this token made.
func (a *app) suggestWithdraw(args []string) error {
	fs := a.flags("suggest withdraw", true)
	pos, err := a.parseN(fs, args, 1, "SUGGESTION_ID")
	if err != nil {
		return err
	}
	var list struct {
		Suggestions []api.Suggestion `json:"suggestions"`
	}
	if err := a.get("/suggestions", url.Values{"status": {"pending"}}, &list); err != nil {
		return err
	}
	titles := map[string]string{}
	for _, s := range list.Suggestions {
		titles[s.ID] = s.Title
	}
	id := pos[0]
	if len(id) != idLength {
		if id, err = pick("pending suggestion", id, slices.Collect(maps.Keys(titles))); err != nil {
			return err
		}
	}
	if printed, _, err := a.send(http.MethodDelete, "/suggestions/"+id, nil, nil, nil); err != nil || printed {
		return err
	}
	a.printf("已撤回建议 %s「%s」%s\n", short(id), titles[id], a.dryNote())
	return nil
}

func (a *app) suggestList(args []string) error {
	fs := a.flags("suggest list", false)
	status := fs.String("status", "pending", "pending, accepted, rejected or any")
	if _, err := a.parseN(fs, args, 0, "[--status pending|accepted|rejected|any]"); err != nil {
		return err
	}
	var resp struct {
		Suggestions []api.Suggestion `json:"suggestions"`
	}
	q := map[string][]string{"status": {*status}}
	if printed, _, err := a.send(http.MethodGet, "/suggestions", q, nil, &resp); err != nil || printed {
		return err
	}
	if _, err := a.zone(); err != nil {
		return err
	}
	for _, s := range resp.Suggestions {
		a.printf("%s\n", a.suggestionLine(s))
	}
	if len(resp.Suggestions) == 0 {
		a.printf("（没有建议）\n")
	}
	return nil
}
