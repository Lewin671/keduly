package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/Lewin671/keduly/internal/api"
)

func (a *app) login(args []string) error {
	fs := a.flags("login", false)
	server := fs.String("server", a.env.Getenv("KEDULY_SERVER"), "server URL, e.g. https://keduly.example.com")
	token := fs.String("token", "", "API token (kdl_...); prompted for when omitted")
	if _, err := parseN(fs, args, 0, "--server URL [--token TOKEN]"); err != nil {
		return err
	}
	if *server == "" {
		return usagef("--server is required")
	}
	if u, err := url.Parse(*server); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return usagef("--server must be an http or https URL")
	}
	if *token == "" {
		secret, err := a.promptToken()
		if err != nil {
			return err
		}
		*token = secret
	}
	a.cfg = config{Server: strings.TrimRight(*server, "/"), Token: strings.TrimSpace(*token)}
	me, err := a.account()
	if err != nil {
		return fmt.Errorf("the server did not accept the token: %v", err)
	}
	path, err := a.saveConfig()
	if err != nil {
		return err
	}
	if a.json {
		_, _, err := a.send(http.MethodGet, "/me", nil, nil, nil)
		return err
	}
	a.printf("已登录 %s（%s），凭证「%s」\n配置已保存到 %s\n", me.User.Name, me.User.Email, me.Actor.Name, path)
	return nil
}

// promptToken reads the token without echo on a terminal, or one line otherwise.
func (a *app) promptToken() (string, error) {
	fmt.Fprint(a.env.Stderr, "Token: ")
	if f, ok := a.env.Stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		secret, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(a.env.Stderr)
		return strings.TrimSpace(string(secret)), err
	}
	line, err := bufio.NewReader(a.env.Stdin).ReadString('\n')
	if line = strings.TrimSpace(line); line == "" {
		if err == nil {
			err = errors.New("no token given")
		}
		return "", fmt.Errorf("cannot read the token: %v", err)
	}
	return line, nil
}

func (a *app) whoami(args []string) error {
	fs := a.flags("whoami", false)
	if _, err := parseN(fs, args, 0, ""); err != nil {
		return err
	}
	var me meResponse
	if printed, _, err := a.send(http.MethodGet, "/me", nil, nil, &me); err != nil || printed {
		return err
	}
	a.printf("%s <%s>\n时区 %s · 工作时间 %s-%s\n凭证「%s」 · 服务器 %s\n", me.User.Name, me.User.Email,
		me.User.Timezone, me.User.WorkStart, me.User.WorkEnd, me.Actor.Name, a.cfg.Server)
	return nil
}

func (a *app) agenda(args []string) error {
	fs := a.flags("agenda", false)
	date := fs.String("date", "today", "first day")
	days := fs.Int("days", 1, "number of days (1-31)")
	if _, err := parseN(fs, args, 0, "[--date D] [--days N]"); err != nil {
		return err
	}
	if *days < 1 || *days > 31 {
		return usagef("--days must be between 1 and 31")
	}
	loc, err := a.zone()
	if err != nil {
		return err
	}
	first, err := parseDate(*date, a.env.Now(), loc)
	if err != nil {
		return err
	}
	start, _ := time.ParseInLocation(dateLayout, first, loc)
	end := start.AddDate(0, 0, *days)
	last := end.AddDate(0, 0, -1).Format(dateLayout)
	events, err := a.events(start, end)
	if err != nil {
		return err
	}
	var upcoming struct {
		Days []api.Day `json:"days"`
	}
	if err := a.get("/upcoming", url.Values{"from": {first}, "to": {last}}, &upcoming); err != nil {
		return err
	}
	type dayView struct {
		Date   string      `json:"date"`
		Events []api.Event `json:"events"`
		Items  []api.Item  `json:"items"`
	}
	var view []dayView
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		day := dayView{Date: d.Format(dateLayout), Events: []api.Event{}, Items: []api.Item{}}
		next := d.AddDate(0, 0, 1)
		for _, e := range events {
			if a.touches(e, d, next) {
				day.Events = append(day.Events, e)
			}
		}
		for _, u := range upcoming.Days {
			if u.Date == day.Date {
				day.Items = u.Items
			}
		}
		view = append(view, day)
	}
	if a.json {
		return a.printJSON(map[string]any{"days": view})
	}
	projects := a.projectNames()
	for _, day := range view {
		a.printf("%s\n", dateLabel(day.Date))
		if len(day.Events) == 0 && len(day.Items) == 0 {
			a.printf("  （无安排）\n")
		}
		for _, e := range day.Events {
			a.printf("  %s\n", a.eventLine(e, projects))
		}
		for _, it := range day.Items {
			a.printf("  %s\n", a.itemLine(it, projects))
		}
	}
	return nil
}

// touches reports whether an event overlaps the local day [from, to).
func (a *app) touches(e api.Event, from, to time.Time) bool {
	if e.AllDay {
		if e.StartDate == nil || e.EndDate == nil {
			return false
		}
		day := from.Format(dateLayout)
		return *e.StartDate <= day && day <= *e.EndDate
	}
	if e.Start == nil || e.End == nil {
		return false
	}
	s, en := localTime(*e.Start, a.loc), localTime(*e.End, a.loc)
	return s.Before(to) && (en.After(from) || s.Equal(from))
}

func (a *app) free(args []string) error {
	fs := a.flags("free", false)
	date := fs.String("date", "today", "day to look at")
	duration := fs.String("duration", "30m", "minimum length of a slot")
	if _, err := parseN(fs, args, 0, "--date D --duration 60m"); err != nil {
		return err
	}
	loc, err := a.zone()
	if err != nil {
		return err
	}
	day, err := parseDate(*date, a.env.Now(), loc)
	if err != nil {
		return err
	}
	minutes, err := parseMinutes(*duration)
	if err != nil {
		return err
	}
	var resp struct {
		Slots []api.Slot `json:"slots"`
	}
	q := url.Values{"date": {day}, "duration": {strconv.Itoa(minutes)}}
	if printed, _, err := a.send(http.MethodGet, "/free", q, nil, &resp); err != nil || printed {
		return err
	}
	a.printf("%s 至少 %s 的空档：\n", dateLabel(day), minutesLabel(minutes))
	if len(resp.Slots) == 0 {
		a.printf("  （没有）\n")
	}
	for _, s := range resp.Slots {
		from, to := localTime(s.Start, loc), localTime(s.End, loc)
		a.printf("  %s-%s  (%s)\n", from.Format("15:04"), to.Format("15:04"), minutesLabel(int(to.Sub(from).Minutes())))
	}
	return nil
}

func (a *app) projectList(args []string) error {
	fs := a.flags("project list", false)
	if _, err := parseN(fs, args, 0, ""); err != nil {
		return err
	}
	var b api.Bootstrap
	if printed, _, err := a.send(http.MethodGet, "/bootstrap", nil, nil, &b); err != nil || printed {
		return err
	}
	areas := map[string]string{}
	for _, area := range b.Areas {
		areas[area.ID] = area.Name
	}
	for _, p := range b.Projects {
		line := fmt.Sprintf("%s  %s  (%s · 未完成 %d · 已完成 %d", short(p.ID), p.Name, p.Color, p.OpenCount, p.DoneCount)
		if p.AreaID != nil && areas[*p.AreaID] != "" {
			line += " · " + areas[*p.AreaID]
		}
		if p.Archived {
			line += " · 已归档"
		}
		a.printf("%s)\n", line)
	}
	if len(b.Projects) == 0 {
		a.printf("（还没有项目）\n")
	}
	return nil
}

// areaID finds an area by name or ID, creating it when it does not exist.
func (a *app) areaID(ref string) (string, error) {
	b, err := a.bootstrap()
	if err != nil {
		return "", err
	}
	for _, area := range b.Areas {
		if area.ID == ref || strings.EqualFold(area.Name, ref) {
			return area.ID, nil
		}
	}
	if a.dryRun {
		return "", fmt.Errorf("area %q does not exist; a dry run does not create it", ref)
	}
	var resp struct {
		Area api.Area `json:"area"`
	}
	data, _, err := a.request(http.MethodPost, "/areas", nil, map[string]any{"name": ref})
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	return resp.Area.ID, nil
}

func (a *app) printJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	a.printf("%s\n", data)
	return nil
}

func (a *app) projectAdd(args []string) error {
	fs := a.flags("project add", true)
	color := fs.String("color", "", "blue, indigo, orange, teal, green, pink, purple or brown")
	area := fs.String("area", "", "area name or ID; created when missing")
	notes := fs.String("notes", "", "notes")
	pos, err := parseN(fs, args, 1, "NAME [--color C] [--area A]")
	if err != nil {
		return err
	}
	body := map[string]any{"name": pos[0]}
	if *color != "" {
		body["color"] = *color
	}
	if *notes != "" {
		body["notes"] = *notes
	}
	if *area != "" {
		id, err := a.areaID(*area)
		if err != nil {
			return err
		}
		body["area_id"] = id
	}
	var resp struct {
		Project api.Project `json:"project"`
	}
	if printed, _, err := a.send(http.MethodPost, "/projects", nil, body, &resp); err != nil || printed {
		return err
	}
	a.printf("已新建项目「%s」 %s (%s)%s\n", resp.Project.Name, short(resp.Project.ID), resp.Project.Color, a.dryNote())
	return nil
}

func (a *app) activity(args []string) error {
	fs := a.flags("activity", false)
	limit := fs.Int("limit", 20, "number of entries (1-200)")
	if _, err := parseN(fs, args, 0, "[--limit N]"); err != nil {
		return err
	}
	var resp struct {
		Activities []api.Activity `json:"activities"`
	}
	q := url.Values{"limit": {strconv.Itoa(*limit)}}
	if printed, _, err := a.send(http.MethodGet, "/activity", q, nil, &resp); err != nil || printed {
		return err
	}
	if _, err := a.zone(); err != nil {
		return err
	}
	for _, act := range resp.Activities {
		a.printf("%s\n", a.activityLine(act))
	}
	if len(resp.Activities) == 0 {
		a.printf("（还没有动态）\n")
	}
	return nil
}

// undo reverts one activity entry: the given one, or the most recent
// undoable entry this token made.
func (a *app) undo(args []string) error {
	fs := a.flags("undo", true)
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return usagef("usage: keduly undo [ACTIVITY_ID]")
	}
	recent, err := a.activities(200)
	if err != nil {
		return err
	}
	id := ""
	if len(pos) == 1 {
		ids := make([]string, 0, len(recent))
		for _, act := range recent {
			ids = append(ids, act.ID)
		}
		if id = pos[0]; len(id) != idLength {
			if id, err = pick("activity", pos[0], ids); err != nil {
				return err
			}
		}
	} else {
		me, err := a.account()
		if err != nil {
			return err
		}
		for _, act := range recent {
			if act.Undoable && !act.Undone && act.Actor == me.Actor {
				id = act.ID
				break
			}
		}
		if id == "" {
			return fmt.Errorf("nothing to undo: no recent undoable change was made by %q", me.Actor.Name)
		}
	}
	var resp struct {
		Activity api.Activity `json:"activity"`
	}
	if printed, _, err := a.send(http.MethodPost, "/activity/"+id+"/undo", nil, nil, &resp); err != nil || printed {
		return err
	}
	a.printf("已撤销：%s\n", resp.Activity.Summary)
	return nil
}
