package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/Lewin671/keduly/internal/api"
)

// itemFlags are the options that describe an item, shared by add, edit and suggest.
type itemFlags struct {
	fs                                    *flag.FlagSet
	title, project, when, due, estimate   *string
	heading, notes                        *string
	important, notImportant, evening, day *bool
}

func (a *app) itemFlags(fs *flag.FlagSet, editing bool) *itemFlags {
	f := &itemFlags{fs: fs}
	f.project = fs.String("project", "", `project name or ID; "none" for the inbox`)
	f.heading = fs.String("heading", "", `heading ID, PROJECT/NAME, or a name within the item's project; "none" clears it`)
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

// body turns the flags that were given into API fields. itemID is the item
// being edited, or empty for a new one.
func (f *itemFlags) body(a *app, itemID string) (map[string]any, error) {
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
	if given(f.fs, "heading") {
		body["heading_id"] = nil
		if !none(*f.heading) {
			h, err := f.headingIn(a, body, itemID)
			if err != nil {
				return nil, err
			}
			body["heading_id"] = h.ID
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

// headingIn resolves --heading. A bare name is looked up in the project given
// with --project, or else in the project the edited item is in.
func (f *itemFlags) headingIn(a *app, body map[string]any, itemID string) (*api.Heading, error) {
	project := ""
	switch id, set := body["project_id"]; {
	case set && id == nil:
		return nil, usagef("--heading needs a project: an inbox item has no headings")
	case set:
		project = id.(string)
	case itemID != "":
		var current itemResponse
		if err := a.get("/items/"+itemID, nil, &current); err != nil {
			return nil, err
		}
		if current.Item.ProjectID != nil {
			project = *current.Item.ProjectID
		}
	}
	return a.heading(*f.heading, project)
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
	pos, err := a.parseN(fs, args, 1, "TITLE [flags]")
	if err != nil {
		return err
	}
	body, err := f.body(a, "")
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

// maxAllRows is where `item list --all` stops following pages.
var maxAllRows = 2000

// itemPage is one page of GET /items, or several pages joined.
type itemPage struct {
	Items      []json.RawMessage `json:"items"`
	NextCursor *string           `json:"next_cursor"`
	Total      int               `json:"total"`
}

// listFilters are the flags of `item list` that only the plain list understands.
var listFilters = []string{"project", "heading", "query", "status", "quadrant", "cursor", "all"}

func (a *app) itemList(args []string) error {
	fs := a.flags("item list", false)
	view := fs.String("view", "", "today, inbox, upcoming, all, matrix or done (default: today, or all with any filter)")
	project := fs.String("project", "", "project name or ID")
	heading := fs.String("heading", "", `heading ID, PROJECT/NAME, or a name within --project; "none" for items without a heading`)
	query := fs.String("query", "", "text to find in titles and notes, ignoring case")
	status := fs.String("status", "", "open, done or any (default: open)")
	quadrant := fs.String("quadrant", "", "do (important, urgent), plan (important), quick (urgent) or later (neither)")
	limit := fs.Int("limit", 50, "items per page (1-200)")
	cursor := fs.String("cursor", "", "continue from the next_cursor of the previous page")
	all := fs.Bool("all", false, "follow every page to the end; stops at 2000 items")
	if _, err := a.parseN(fs, args, 0, "[--view V] [filters] [--limit N] [--cursor C] [--all]"); err != nil {
		return err
	}
	filtered := slices.ContainsFunc(listFilters, func(name string) bool { return given(fs, name) })
	if *view == "" {
		*view = "today"
		if filtered {
			*view = "all"
		}
	}
	q := url.Values{"limit": {strconv.Itoa(*limit)}}
	switch *view {
	case "today", "upcoming", "matrix":
		if filtered {
			return usagef("--view %s takes no filters and no paging; use --view all with them", *view)
		}
		return a.listView(*view)
	case "inbox":
		if *project != "" || *heading != "" {
			return usagef("--view inbox lists items without a project; drop --project and --heading")
		}
		q.Set("project_id", "none")
	case "done":
		if *status != "" && *status != "done" {
			return usagef("--view done lists done items; drop --status or use --view all")
		}
		q.Set("status", "done")
	case "all":
	default:
		return usagef("--view must be today, inbox, upcoming, all, matrix or done")
	}
	for name, value := range map[string]string{"status": *status, "quadrant": *quadrant, "q": *query, "cursor": *cursor} {
		if value != "" {
			q.Set(name, value)
		}
	}
	if err := a.listPlace(q, *project, *heading); err != nil {
		return err
	}
	page, err := a.itemPages(q, *all)
	if err != nil {
		return err
	}
	return a.showPage(page, *all)
}

// listView prints one of the views the server assembles in a single request.
func (a *app) listView(view string) error {
	switch view {
	case "upcoming":
		return a.listUpcoming()
	case "matrix":
		return a.listMatrix()
	}
	return a.listToday()
}

// listPlace turns --project and --heading into filters.
func (a *app) listPlace(q url.Values, project, heading string) error {
	projectID := ""
	if project != "" {
		var err error
		if projectID, err = a.projectID(project); err != nil {
			return err
		}
		q.Set("project_id", projectID)
	}
	switch {
	case strings.EqualFold(heading, "none"):
		q.Set("heading_id", "none")
	case heading != "":
		h, err := a.heading(heading, projectID)
		if err != nil {
			return err
		}
		q.Set("heading_id", h.ID)
	}
	return nil
}

// itemPages fetches one page of items, or with all every following page too,
// up to maxAllRows items.
func (a *app) itemPages(q url.Values, all bool) (*itemPage, error) {
	out := &itemPage{Items: []json.RawMessage{}}
	for {
		if all {
			q.Set("limit", strconv.Itoa(min(200, maxAllRows-len(out.Items))))
		}
		var page itemPage
		if err := a.get("/items", q, &page); err != nil {
			return nil, err
		}
		out.Items = append(out.Items, page.Items...)
		out.NextCursor, out.Total = page.NextCursor, page.Total
		if !all || page.NextCursor == nil || len(out.Items) >= maxAllRows {
			return out, nil
		}
		q.Set("cursor", *page.NextCursor)
	}
}

// showPage prints a page of items and, when more exist, how to continue.
func (a *app) showPage(page *itemPage, all bool) error {
	more := page.NextCursor != nil
	if a.json {
		if more && all {
			fmt.Fprintf(a.env.Stderr, "keduly: stopped at %d of %d items; continue with --cursor %s\n",
				len(page.Items), page.Total, *page.NextCursor)
		}
		return a.printJSON(page)
	}
	if _, err := a.zone(); err != nil {
		return err
	}
	projects := a.projectNames()
	for _, raw := range page.Items {
		var it api.Item
		if err := json.Unmarshal(raw, &it); err != nil {
			return fmt.Errorf("unexpected response: %v", err)
		}
		a.printf("%s\n", a.itemLine(it, projects))
	}
	switch {
	case len(page.Items) == 0:
		a.printf("（没有事项）\n")
	case more && all:
		a.printf("… 共 %d 件，已到 %d 件上限；继续请加 --cursor %s\n", page.Total, len(page.Items), *page.NextCursor)
	case more:
		a.printf("… 共 %d 件，本页 %d 件；下一页请加 --cursor %s（或用 --all 取完）\n", page.Total, len(page.Items), *page.NextCursor)
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
	pos, err := a.parseN(fs, args, 1, "ID")
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
	b, err := a.bootstrap()
	if err != nil {
		return err
	}
	it := resp.Item
	projects := map[string]string{}
	for _, p := range b.Projects {
		projects[p.ID] = p.Name
	}
	a.printf("%s\nID %s · 创建者 %s", a.itemLine(it, projects), it.ID, it.CreatedBy.Name)
	for _, h := range b.Headings {
		if it.HeadingID != nil && h.ID == *it.HeadingID {
			a.printf(" · 分节 %s", h.Name)
		}
	}
	if it.Evening {
		a.printf(" · 今晚")
	}
	a.printf("\n")
	if it.Notes != "" {
		a.printf("\n%s\n", it.Notes)
	}
	return nil
}

func (a *app) itemEdit(args []string) error {
	fs := a.flags("item edit", true)
	f := a.itemFlags(fs, true)
	pos, err := a.parseN(fs, args, 1, "ID [flags]")
	if err != nil {
		return err
	}
	id, err := a.itemID(pos[0])
	if err != nil {
		return err
	}
	body, err := f.body(a, id)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return usagef("nothing to change; give at least one flag")
	}
	return a.patchItem(id, body, "已修改")
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
		pos, err := a.parseN(fs, args, 1, "ID")
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
	pos, err := a.parseN(fs, args, 1, "ID")
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
	pos, err := a.parseN(fs, args, 1, "ID --start T (--end T | --duration 1h)")
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
	pos, err := a.parseN(fs, args, 1, "ID")
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
