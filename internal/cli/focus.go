package cli

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Lewin671/keduly/internal/api"
)

type focusResponse struct {
	Focus api.Focus `json:"focus"`
}

func sessionTitle(s api.FocusSession) string {
	if s.Title == "" {
		return "自由专注"
	}
	return "「" + s.Title + "」"
}

// focusTotals is the line about today that every timer command ends with.
func focusTotals(f api.Focus) string {
	return fmt.Sprintf("今天 %d 个番茄 · %s · 本轮 %d/%d", f.TomatoesToday, minutesLabel(f.MinutesToday), f.RoundDone, f.RoundSize)
}

// focusLines describes the timer: what it is doing, how long is left, and today's totals.
func (a *app) focusLines(f api.Focus) string {
	if f.Session == nil {
		return "计时器空闲\n" + focusTotals(f) + "\n"
	}
	s := *f.Session
	now, _ := time.Parse(time.RFC3339, f.Now)
	end := localTime(s.End, a.loc)
	left := end.Sub(now).Round(time.Second)
	var head string
	switch f.State {
	case "work":
		head = fmt.Sprintf("专注中：%s · 还剩 %s，到 %s", sessionTitle(s), left, end.Format("15:04"))
	case "rest":
		head = fmt.Sprintf("休息中 · 还剩 %s，到 %s", left, end.Format("15:04"))
	default:
		head = fmt.Sprintf("番茄到点了：%s，%s 结束 · 接下来可以 `keduly focus rest`（休息 %s）、再 `keduly focus start`，或 `keduly focus stop`",
			sessionTitle(s), end.Format("15:04"), minutesLabel(f.RestMinutes))
	}
	return head + "\n" + focusTotals(f) + "\n"
}

// focusCommand runs one of the timer's writes and prints the state it leads to.
func (a *app) focusCommand(path, verb string, body map[string]any) error {
	var resp focusResponse
	if printed, _, err := a.send(http.MethodPost, path, nil, body, &resp); err != nil || printed {
		return err
	}
	if _, err := a.zone(); err != nil {
		return err
	}
	a.printf("%s%s\n%s", verb, a.dryNote(), a.focusLines(resp.Focus))
	return nil
}

func (a *app) focusStatus(args []string) error {
	fs := a.flags("focus status", false)
	if _, err := a.parseN(fs, args, 0, ""); err != nil {
		return err
	}
	var resp focusResponse
	if printed, _, err := a.send(http.MethodGet, "/focus", nil, nil, &resp); err != nil || printed {
		return err
	}
	if _, err := a.zone(); err != nil {
		return err
	}
	a.printf("%s", a.focusLines(resp.Focus))
	return nil
}

func (a *app) focusStart(args []string) error {
	fs := a.flags("focus start", true)
	project := fs.String("project", "", "for free focus: project name or ID")
	title := fs.String("title", "", "for free focus: what the time goes to")
	pos, err := a.parse(fs, args, "[ITEM_ID | --title T --project P]")
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return usagef("usage: keduly focus start [ITEM_ID]")
	}
	body := map[string]any{}
	if len(pos) == 1 {
		if body["item_id"], err = a.itemID(pos[0]); err != nil {
			return err
		}
	}
	if given(fs, "project") {
		if body["project_id"], err = a.projectID(*project); err != nil {
			return err
		}
	}
	if given(fs, "title") {
		body["title"] = *title
	}
	return a.focusCommand("/focus/start", "已开始一个番茄", body)
}

func (a *app) focusStop(args []string) error {
	fs := a.flags("focus stop", true)
	if _, err := a.parseN(fs, args, 0, ""); err != nil {
		return err
	}
	return a.focusCommand("/focus/stop", "已停止计时", nil)
}

func (a *app) focusRest(args []string) error {
	fs := a.flags("focus rest", true)
	if _, err := a.parseN(fs, args, 0, ""); err != nil {
		return err
	}
	return a.focusCommand("/focus/rest", "已开始休息", nil)
}

func (a *app) focusLog(args []string) error {
	fs := a.flags("focus log", false)
	from := fs.String("from", "today", "first `date`")
	to := fs.String("to", "", "last `date` (default: the first date)")
	if _, err := a.parseN(fs, args, 0, ""); err != nil {
		return err
	}
	loc, err := a.zone()
	if err != nil {
		return err
	}
	first, err := parseDate(*from, a.env.Now(), loc)
	if err != nil {
		return usagef("--from: %v", err)
	}
	last := first
	if *to != "" {
		if last, err = parseDate(*to, a.env.Now(), loc); err != nil {
			return usagef("--to: %v", err)
		}
	}
	var resp struct {
		Sessions []api.FocusSession `json:"sessions"`
	}
	if printed, _, err := a.send(http.MethodGet, "/focus/sessions", url.Values{"from": {first}, "to": {last}}, nil, &resp); err != nil || printed {
		return err
	}
	if len(resp.Sessions) == 0 {
		a.printf("（这段时间没有专注记录）\n")
		return nil
	}
	projects := a.projectNames()
	for _, s := range resp.Sessions {
		start, end := localTime(s.Start, loc), localTime(s.End, loc)
		mark := "未完成"
		switch {
		case s.Completed:
			mark = "番茄"
		case end.Sub(start) >= time.Duration(s.PlannedMinutes)*time.Minute:
			// Not given up and not completed: its planned end is still ahead.
			mark = "进行中"
		}
		line := fmt.Sprintf("%s  %s  %-4s %s  %s", short(s.ID), span(s.Start, s.End, loc), minutesLabel(int(end.Sub(start).Round(time.Minute).Minutes())), mark, sessionTitle(s))
		if s.ProjectID != nil && projects[*s.ProjectID] != "" {
			line += "  (" + projects[*s.ProjectID] + ")"
		}
		a.printf("%s\n", line)
	}
	return nil
}

// sessionID resolves a prefix among the sessions of the last 62 days, as far back as the log goes.
func (a *app) sessionID(prefix string) (string, error) {
	if len(prefix) == idLength {
		return prefix, nil
	}
	loc, err := a.zone()
	if err != nil {
		return "", err
	}
	now := a.env.Now().In(loc)
	var resp struct {
		Sessions []api.FocusSession `json:"sessions"`
	}
	q := url.Values{"from": {now.AddDate(0, 0, -62).Format("2006-01-02")}, "to": {now.Format("2006-01-02")}}
	if err := a.get("/focus/sessions", q, &resp); err != nil {
		return "", err
	}
	ids := make([]string, 0, len(resp.Sessions))
	for _, s := range resp.Sessions {
		ids = append(ids, s.ID)
	}
	return pick("focus session", prefix, ids)
}

func (a *app) focusEdit(args []string) error {
	fs := a.flags("focus edit", true)
	item := fs.String("item", "", `the item the time counts towards; "none" makes it free focus`)
	project := fs.String("project", "", `for free focus: project name or ID; "none" for no project`)
	title := fs.String("title", "", "for free focus: what the time went to; empty clears it")
	pos, err := a.parseN(fs, args, 1, "SESSION_ID [--item ITEM|none] [--project P|none] [--title T]")
	if err != nil {
		return err
	}
	body := map[string]any{}
	if given(fs, "item") {
		body["item_id"] = nil
		if !strings.EqualFold(*item, "none") {
			if body["item_id"], err = a.itemID(*item); err != nil {
				return err
			}
		}
	}
	if given(fs, "project") {
		body["project_id"] = nil
		if !strings.EqualFold(*project, "none") {
			if body["project_id"], err = a.projectID(*project); err != nil {
				return err
			}
		}
	}
	if given(fs, "title") {
		body["title"] = *title
	}
	if len(body) == 0 {
		return usagef("nothing to change: give --item, --project or --title")
	}
	id, err := a.sessionID(pos[0])
	if err != nil {
		return err
	}
	var resp struct {
		Session api.FocusSession `json:"session"`
	}
	if printed, _, err := a.send(http.MethodPatch, "/focus/sessions/"+id, nil, body, &resp); err != nil || printed {
		return err
	}
	loc, err := a.zone()
	if err != nil {
		return err
	}
	s := resp.Session
	line := fmt.Sprintf("已修改专注记录%s\n%s  %s  %s", a.dryNote(), short(s.ID), span(s.Start, s.End, loc), sessionTitle(s))
	if s.ProjectID != nil {
		if name := a.projectNames()[*s.ProjectID]; name != "" {
			line += "  (" + name + ")"
		}
	}
	a.printf("%s\n", line)
	return nil
}

func (a *app) focusStats(args []string) error {
	fs := a.flags("focus stats", false)
	if _, err := a.parseN(fs, args, 0, ""); err != nil {
		return err
	}
	var stats api.FocusStats
	if printed, _, err := a.send(http.MethodGet, "/focus/stats", nil, nil, &stats); err != nil || printed {
		return err
	}
	a.printf("最近 7 天：%d 个番茄 · %s · 连续 %d 天\n", stats.Tomatoes, minutesLabel(stats.Minutes), stats.Streak)
	for _, d := range stats.Days {
		a.printf("  %s  %2d 个番茄  %s\n", dateLabel(d.Date), d.Tomatoes, minutesLabel(d.Minutes))
	}
	if len(stats.Projects) > 0 {
		a.printf("\n时间花在哪：\n")
	}
	projects := a.projectNames()
	for _, p := range stats.Projects {
		name := "未归项目"
		if p.ProjectID != nil && projects[*p.ProjectID] != "" {
			name = projects[*p.ProjectID]
		}
		a.printf("  %2d 个番茄  %-6s %s\n", p.Tomatoes, minutesLabel(p.Minutes), name)
	}
	return nil
}
