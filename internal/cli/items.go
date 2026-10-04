package cli

import (
	"flag"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Lewin671/keduly/internal/api"
)

// itemFlags are the options that describe an item, shared by add, edit and suggest.
type itemFlags struct {
	fs                                    *flag.FlagSet
	title, project, when, due, estimate   *string
	notes                                 *string
	important, notImportant, evening, day *bool
}

func (a *app) itemFlags(fs *flag.FlagSet, editing bool) *itemFlags {
	f := &itemFlags{fs: fs}
	f.project = fs.String("project", "", `project name or ID; "none" for the inbox`)
	f.when = fs.String("when", "", `planned date; "none" clears it`)
	f.due = fs.String("due", "", `deadline: a date or "DATE HH:MM"; "none" clears it`)
	f.estimate = fs.String("estimate", "", `estimated duration, e.g. 30m; "none" clears it`)
	f.notes = fs.String("notes", "", "notes")
	f.important = fs.Bool("important", false, "mark as important")
	f.evening = fs.Bool("evening", false, "show under this evening")
	if editing {
		f.title = fs.String("title", "", "new title")
		f.notImportant = fs.Bool("no-important", false, "remove the important mark")
		f.day = fs.Bool("no-evening", false, "remove the evening mark")
	}
	return f
}

// body turns the flags that were given into API fields.
func (f *itemFlags) body(a *app) (map[string]any, error) {
	body := map[string]any{}
	none := func(s string) bool { return strings.EqualFold(s, "none") }
	if given(f.fs, "title") {
		body["title"] = *f.title
	}
	if given(f.fs, "notes") {
		body["notes"] = *f.notes
	}
	if given(f.fs, "project") {
		if none(*f.project) {
			body["project_id"] = nil
		} else {
			id, err := a.projectID(*f.project)
			if err != nil {
				return nil, err
			}
			body["project_id"] = id
		}
	}
	needZone := given(f.fs, "when") || given(f.fs, "due")
	if needZone {
		if _, err := a.zone(); err != nil {
			return nil, err
		}
	}
	if given(f.fs, "when") {
		if none(*f.when) {
			body["planned_date"] = nil
		} else {
			date, err := parseDate(*f.when, a.env.Now(), a.loc)
			if err != nil {
				return nil, err
			}
			body["planned_date"] = date
		}
	}
	if given(f.fs, "due") {
		body["due_date"], body["due_time"] = nil, nil
		if !none(*f.due) {
			if date, err := parseDate(*f.due, a.env.Now(), a.loc); err == nil {
				body["due_date"] = date
			} else {
				t, err := parseTime(*f.due, a.env.Now(), a.loc)
				if err != nil {
					return nil, err
				}
				body["due_date"], body["due_time"] = t.Format(dateLayout), t.Format("15:04")
			}
		}
	}
	if given(f.fs, "estimate") {
		if none(*f.estimate) {
			body["estimate_minutes"] = nil
		} else {
			minutes, err := parseMinutes(*f.estimate)
			if err != nil {
				return nil, err
			}
			body["estimate_minutes"] = minutes
		}
	}
	if given(f.fs, "important") {
		body["important"] = *f.important
	}
	if given(f.fs, "no-important") {
		body["important"] = false
	}
	if given(f.fs, "evening") {
		body["evening"] = *f.evening
	}
	if given(f.fs, "no-evening") {
		body["evening"] = false
	}
	return body, nil
}

type itemResponse struct {
	Item       api.Item        `json:"item"`
	Suggestion *api.Suggestion `json:"suggestion"`
}

// showItem prints the outcome of a command that returns one item.
func (a *app) showItem(verb string, resp itemResponse) error {
	if _, err := a.zone(); err != nil {
		return err
	}
	a.printf("%s%s\n%s\n", verb, a.dryNote(), a.itemLine(resp.Item, a.projectNames()))
	return nil
}

func (a *app) itemAdd(args []string) error {
	fs := a.flags("item add", true)
	f := a.itemFlags(fs, false)
	pos, err := parseN(fs, args, 1, "TITLE [flags]")
	if err != nil {
		return err
	}
	body, err := f.body(a)
	if err != nil {
		return err
	}
	body["title"] = pos[0]
	var resp itemResponse
	if printed, _, err := a.send(http.MethodPost, "/items", nil, body, &resp); err != nil || printed {
		return err
	}
	return a.showItem("已新建事项", resp)
}

func (a *app) itemList(args []string) error {
	fs := a.flags("item list", false)
	view := fs.String("view", "", "today, inbox, upcoming, all, matrix or done (default: today, or all with --project)")
	project := fs.String("project", "", "project name or ID")
	limit := fs.Int("limit", 50, "maximum number of items (1-200)")
	if _, err := parseN(fs, args, 0, "[--view V] [--project P] [--limit N]"); err != nil {
		return err
	}
	if *view == "" {
		*view = "today"
		if *project != "" {
			*view = "all"
		}
	}
	q := url.Values{"limit": {strconv.Itoa(*limit)}}
	if *project != "" {
		id, err := a.projectID(*project)
		if err != nil {
			return err
		}
		q.Set("project_id", id)
	}
	switch *view {
	case "today":
		return a.listToday()
	case "upcoming":
		return a.listUpcoming()
	case "matrix":
		return a.listMatrix()
	case "inbox":
		q.Set("project_id", "none")
	case "done":
		q.Set("status", "done")
	case "all":
	default:
		return usagef("--view must be today, inbox, upcoming, all, matrix or done")
	}
	var resp struct {
		Items []api.Item `json:"items"`
		Total int        `json:"total"`
	}
	if printed, _, err := a.send(http.MethodGet, "/items", q, nil, &resp); err != nil || printed {
		return err
	}
	if _, err := a.zone(); err != nil {
		return err
	}
	a.printItems(resp.Items, a.projectNames())
	if len(resp.Items) == 0 {
		a.printf("（没有事项）\n")
	} else if resp.Total > len(resp.Items) {
		a.printf("… 共 %d 件，显示前 %d 件\n", resp.Total, len(resp.Items))
	}
	return nil
}

func (a *app) listToday() error {
	var today api.Today
	if printed, _, err := a.send(http.MethodGet, "/today", nil, nil, &today); err != nil || printed {
		return err
	}
	if _, err := a.zone(); err != nil {
		return err
	}
	projects := a.projectNames()
	a.printf("%s · 空闲 %s · 未安排 %s\n", dateLabel(today.Date), minutesLabel(today.FreeMinutes), minutesLabel(today.UnplannedMinutes))
	a.printItems(today.Items, projects)
	if len(today.Items) == 0 {
		a.printf("（今天没有事项）\n")
	}
	if len(today.Overdue) > 0 {
		a.printf("已逾期：\n")
		a.printItems(today.Overdue, projects)
	}
	return nil
}

func (a *app) listUpcoming() error {
	loc, err := a.zone()
	if err != nil {
		return err
	}
	now := a.env.Now().In(loc)
	q := url.Values{"from": {now.AddDate(0, 0, 1).Format(dateLayout)}, "to": {now.AddDate(0, 0, 14).Format(dateLayout)}}
	var resp struct {
		Days []api.Day `json:"days"`
	}
	if printed, _, err := a.send(http.MethodGet, "/upcoming", q, nil, &resp); err != nil || printed {
		return err
	}
	projects := a.projectNames()
	shown := false
	for _, day := range resp.Days {
		if len(day.Items) == 0 {
			continue
		}
		shown = true
		a.printf("%s\n", dateLabel(day.Date))
		for _, it := range day.Items {
			a.printf("  %s\n", a.itemLine(it, projects))
		}
	}
	if !shown {
		a.printf("（未来 14 天没有事项）\n")
	}
	return nil
}

func (a *app) listMatrix() error {
	var resp struct {
		Quadrants map[string]api.Quadrant `json:"quadrants"`
	}
	if printed, _, err := a.send(http.MethodGet, "/matrix", nil, nil, &resp); err != nil || printed {
		return err
	}
	if _, err := a.zone(); err != nil {
		return err
	}
	projects := a.projectNames()
	labels := [][2]string{{"do", "重要且紧急"}, {"plan", "重要不紧急"}, {"quick", "紧急不重要"}, {"later", "不重要不紧急"}}
	for _, l := range labels {
		q := resp.Quadrants[l[0]]
		a.printf("%s（%d 件，%d 件未安排）\n", l[1], q.Total, q.Unplanned)
		for _, it := range q.Items {
			a.printf("  %s\n", a.itemLine(it, projects))
		}
	}
	return nil
}

func (a *app) itemShow(args []string) error {
	fs := a.flags("item show", false)
	pos, err := parseN(fs, args, 1, "ID")
	if err != nil {
		return err
	}
	id, err := a.itemID(pos[0])
	if err != nil {
		return err
	}
	var resp itemResponse
	if printed, _, err := a.send(http.MethodGet, "/items/"+id, nil, nil, &resp); err != nil || printed {
		return err
	}
	if _, err := a.zone(); err != nil {
		return err
	}
	it := resp.Item
	a.printf("%s\nID %s · 创建者 %s\n", a.itemLine(it, a.projectNames()), it.ID, it.CreatedBy.Name)
	if it.Notes != "" {
		a.printf("\n%s\n", it.Notes)
	}
	return nil
}

func (a *app) itemEdit(args []string) error {
	fs := a.flags("item edit", true)
	f := a.itemFlags(fs, true)
	pos, err := parseN(fs, args, 1, "ID [flags]")
	if err != nil {
		return err
	}
	body, err := f.body(a)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return usagef("nothing to change; give at least one flag")
	}
	return a.patchItem(pos[0], body, "已修改")
}

func (a *app) patchItem(ref string, body map[string]any, verb string) error {
	id, err := a.itemID(ref)
	if err != nil {
		return err
	}
	var resp itemResponse
	if printed, _, err := a.send(http.MethodPatch, "/items/"+id, nil, body, &resp); err != nil || printed {
		return err
	}
	return a.showItem(verb, resp)
}

func (a *app) itemStatus(status string) func([]string) error {
	return func(args []string) error {
		verb := "已完成"
		if status == "open" {
			verb = "已重新打开"
		}
		fs := a.flags("item "+map[string]string{"done": "done", "open": "reopen"}[status], true)
		pos, err := parseN(fs, args, 1, "ID")
		if err != nil {
			return err
		}
		return a.patchItem(pos[0], map[string]any{"status": status}, verb)
	}
}

// removed reports a delete, which a token that needs confirmation only proposes.
func (a *app) removed(what, title string, status int, s *api.Suggestion) {
	if status == http.StatusAccepted && s != nil {
		a.printf("已提出删除%s「%s」的建议 %s，等待用户确认%s\n", what, s.Title, short(s.ID), a.dryNote())
		return
	}
	a.printf("已删除%s %s%s\n", what, title, a.dryNote())
}

func (a *app) itemRemove(args []string) error {
	fs := a.flags("item rm", true)
	pos, err := parseN(fs, args, 1, "ID")
	if err != nil {
		return err
	}
	id, err := a.itemID(pos[0])
	if err != nil {
		return err
	}
	var resp itemResponse
	printed, status, err := a.send(http.MethodDelete, "/items/"+id, nil, nil, &resp)
	if err != nil || printed {
		return err
	}
	if _, err := a.zone(); err != nil {
		return err
	}
	a.removed("事项", short(id), status, resp.Suggestion)
	return nil
}

func (a *app) itemSchedule(args []string) error {
	fs := a.flags("item schedule", true)
	start := fs.String("start", "", "start time")
	end := fs.String("end", "", "end time")
	duration := fs.String("duration", "", "length, e.g. 1h")
	pos, err := parseN(fs, args, 1, "ID --start T (--end T | --duration 1h)")
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
	var resp itemResponse
	body := map[string]any{"start": stamp(from), "end": stamp(to)}
	if printed, _, err := a.send(http.MethodPost, "/items/"+id+"/schedule", nil, body, &resp); err != nil || printed {
		return err
	}
	return a.showItem("已安排时间", resp)
}

func (a *app) itemUnschedule(args []string) error {
	fs := a.flags("item unschedule", true)
	pos, err := parseN(fs, args, 1, "ID")
	if err != nil {
		return err
	}
	id, err := a.itemID(pos[0])
	if err != nil {
		return err
	}
	var resp itemResponse
	if printed, _, err := a.send(http.MethodDelete, "/items/"+id+"/schedule", nil, nil, &resp); err != nil || printed {
		return err
	}
	return a.showItem("已取消时间安排", resp)
}
