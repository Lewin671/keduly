package cli_test

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/cli"
)

// agent signs a second token of the same account in, with its own config.
func (h *harness) agent(name string, confirmDelete bool) *harness {
	h.t.Helper()
	other := &harness{t: h.t, s: h.s, me: h.me, config: h.t.TempDir()}
	other.env = map[string]string{"XDG_CONFIG_HOME": other.config}
	token := h.me.NewToken(name, "agent", "write", confirmDelete).Token
	if out, code := other.run("login", "--server", h.s.URL, "--token", token); code != 0 {
		h.t.Fatalf("login failed: %s", out)
	}
	return other
}

// fails runs a command that must exit with code and mention every part.
func (h *harness) fails(code int, args []string, parts ...string) {
	h.t.Helper()
	out, got := h.run(args...)
	if got != code {
		h.t.Fatalf("keduly %s: exit %d, want %d: %s", strings.Join(args, " "), got, code, out)
	}
	wantIn(h.t, out, parts...)
}

func wantOut(t *testing.T, out string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if strings.Contains(out, p) {
			t.Fatalf("output must not have %q:\n%s", p, out)
		}
	}
}

var commands = map[string][]string{
	"item":    {"add", "list", "show", "edit", "done", "reopen", "rm", "schedule", "unschedule"},
	"event":   {"add", "list", "edit", "move", "rm"},
	"project": {"list", "show", "add", "edit", "archive", "unarchive", "rm"},
	"area":    {"list", "add", "rename", "rm"},
	"heading": {"list", "add", "rename", "rm"},
	"suggest": {"schedule", "add-item", "add-event", "move", "delete-item", "delete-event", "withdraw", "list"},
}

func TestHelpCoversEveryCommand(t *testing.T) {
	h := newHarness(t, true)
	help := h.ok("help")
	for _, single := range []string{"serve", "login", "whoami", "version", "agenda", "free", "activity", "undo", "redo"} {
		wantIn(t, help, "keduly "+single)
	}
	for _, single := range []string{"whoami", "agenda", "free", "activity", "undo", "redo"} {
		wantIn(t, h.ok(single, "--help"), "usage: keduly "+single, "--json")
	}
	for group, subs := range commands {
		lines := regexp.MustCompile(`(?m)^  keduly `+group+` .*(\n {6,}.*)*`).FindAllString(help, -1)
		listed := strings.Join(lines, "\n")
		groupHelp := h.ok(group, "--help")
		for _, sub := range subs {
			if !regexp.MustCompile(`[ |]` + sub + `\b`).MatchString(listed) {
				t.Errorf("`keduly help` does not list %s %s:\n%s", group, sub, listed)
			}
			wantIn(t, groupHelp, sub)
			wantIn(t, h.ok(group, sub, "--help"), "usage: keduly "+group+" "+sub, "--json")
		}
		h.fails(2, []string{group}, "missing subcommand", "keduly "+group)
		h.fails(2, []string{group, "frobnicate"}, "unknown command")
	}
	wantIn(t, h.ok("item", "edit", "-h"), "--heading", "--no-evening", "--dry-run", "--reason", "--due")
	wantIn(t, h.ok("item", "list", "--help"), "--query", "--status", "--quadrant", "--cursor", "--all", "(default 50)")
	wantIn(t, h.ok("event", "edit", "--help"), "--all-day", "--timed", "--location")

	// A usage error is one line and exits with 2.
	out, code := h.run("item", "list", "--nope")
	if code != 2 || strings.Count(out, "\n") != 1 || !strings.Contains(out, "flag provided but not defined") {
		t.Fatalf("an unknown flag: %d %q", code, out)
	}
	h.fails(2, []string{"item", "add"}, "usage: keduly item add TITLE")
	h.fails(2, []string{"item", "list", "--limit", "many"}, "invalid value")
	h.fails(2, []string{"redo"}, "usage: keduly redo ACTIVITY_ID")
	h.fails(2, []string{"project", "edit", "x"}, "nothing to change")
}

func TestProjectsAreasAndHeadings(t *testing.T) {
	h := newHarness(t, true)
	wantIn(t, h.ok("area", "list"), "（还没有领域）")
	wantIn(t, h.ok("area", "add", "工作", "--reason", "asked"), "已新建领域「工作」")
	wantIn(t, h.ok("area", "add", "不写入", "--dry-run"), "试运行")
	var areas struct {
		Areas []api.Area `json:"areas"`
	}
	h.json(&areas, "area", "list")
	if len(areas.Areas) != 1 || areas.Areas[0].Name != "工作" {
		t.Fatalf("areas %+v", areas.Areas)
	}
	wantIn(t, h.ok("project", "add", "Keduly", "--area", "工作", "--color", "teal"), "已新建项目")
	wantIn(t, h.ok("area", "list"), "工作", "1 个项目")
	wantIn(t, h.ok("area", "rename", areas.Areas[0].ID[:5], "Work"), "领域「工作」已改名为「Work」")
	h.fails(1, []string{"area", "rename", "工作", "x"}, "no area is named")

	out := h.ok("project", "edit", "keduly", "--name", "Keduly 开发", "--color", "pink", "--notes", "第一行\n第二行", "--reason", "asked")
	wantIn(t, out, "已修改项目", "Keduly 开发", "pink", "Work")
	wantIn(t, h.ok("project", "edit", "Keduly 开发", "--color", "blue", "--dry-run"), "试运行", "blue")
	wantIn(t, h.ok("project", "list"), "pink")
	h.fails(1, []string{"project", "edit", "Keduly 开发", "--color", "plaid"}, "color must be one of", "invalid_request")
	h.fails(1, []string{"project", "show", "nope"}, "no project is named")

	// Headings by ID prefix and by PROJECT/NAME.
	var heading struct {
		Heading api.Heading `json:"heading"`
	}
	h.json(&heading, "heading", "add", "Keduly 开发", "同步")
	wantIn(t, h.ok("heading", "add", "Keduly 开发", "界面"), "已在项目「Keduly 开发」中新建分组「界面」")
	wantIn(t, h.ok("heading", "list", "Keduly 开发"), heading.Heading.ID[:8]+"  同步", "界面")
	wantIn(t, h.ok("heading", "rename", "keduly 开发/界面", "UI"), "分组「界面」已改名为「UI」")
	wantIn(t, h.ok("heading", "rename", heading.Heading.ID[:6], "Sync"), "已改名为「Sync」")
	h.fails(1, []string{"heading", "rm", "UI"}, "the project is not known")
	h.fails(1, []string{"heading", "rm", "Keduly 开发/nope"}, "no heading matches")
	var headings struct {
		Headings []api.Heading `json:"headings"`
	}
	h.json(&headings, "heading", "list", "Keduly 开发")
	if len(headings.Headings) != 2 || headings.Headings[0].Name != "Sync" {
		t.Fatalf("headings %+v", headings.Headings)
	}

	h.ok("item", "add", "实现同步", "--project", "Keduly 开发", "--heading", "Sync")
	h.ok("item", "add", "写文档", "--project", "Keduly 开发", "--when", "today")
	h.ok("item", "done", h.itemID("写文档"))
	h.ok("event", "add", "设计评审", "--start", "tomorrow 10:00", "--duration", "1h", "--project", "Keduly 开发")
	out = h.ok("project", "show", "Keduly 开发")
	wantIn(t, out, "Keduly 开发", "pink", "未完成 1", "已完成 1", "未安排 1", "Work", "第一行\n第二行",
		"分组：", "Sync", "UI", "近期日程：", "2026-10-14 周三", "10:00-11:00", "设计评审")
	var detail api.ProjectDetail
	h.json(&detail, "project", "show", "Keduly 开发")
	if detail.UnplannedCount != 1 || len(detail.Headings) != 2 || len(detail.UpcomingEvents) != 1 || detail.Project.OpenCount != 1 {
		t.Fatalf("detail %+v", detail)
	}

	wantIn(t, h.ok("heading", "rm", "Keduly 开发/UI"), "已删除分组「UI」")
	wantOut(t, h.ok("heading", "list", "Keduly 开发"), "UI")

	// Archived projects leave the list but stay reachable.
	wantIn(t, h.ok("project", "archive", "Keduly 开发"), "已归档项目", "已归档)")
	out = h.ok("project", "list")
	wantIn(t, out, "另有 1 个已归档项目", "--archived")
	wantOut(t, out, "Keduly 开发", "还没有项目")
	wantIn(t, h.ok("project", "list", "--archived"), "Keduly 开发", "已归档")
	wantIn(t, h.ok("project", "unarchive", "Keduly 开发"), "已恢复项目")
	wantOut(t, h.ok("project", "list"), "已归档")

	wantIn(t, h.ok("project", "edit", "Keduly 开发", "--area", "none", "--notes", ""), "已修改项目")
	wantOut(t, h.ok("project", "show", "Keduly 开发"), "Work", "第一行")
	wantIn(t, h.ok("project", "edit", "Keduly 开发", "--area", "work"), "Work")
	wantIn(t, h.ok("area", "rm", "Work"), "已删除领域「Work」")
	wantIn(t, h.ok("project", "list"), "Keduly 开发")
	wantIn(t, h.ok("activity", "--limit", "50"), "新建领域「工作」", "原因：asked", "归档了项目")
}

// itemID finds an open or done item by its exact title.
func (h *harness) itemID(title string) string {
	h.t.Helper()
	var page struct {
		Items []api.Item `json:"items"`
	}
	h.json(&page, "item", "list", "--status", "any", "--query", title)
	for _, it := range page.Items {
		if it.Title == title {
			return it.ID
		}
	}
	h.t.Fatalf("no item is titled %q", title)
	return ""
}

func TestProjectRemoveIsForTheUser(t *testing.T) {
	h := newHarness(t, true)
	h.ok("project", "add", "Work")
	h.ok("item", "add", "写周报", "--project", "Work")
	h.fails(1, []string{"project", "rm", "Work"}, "the user must delete it in the web app", "(forbidden)")
	h.fails(1, []string{"project", "rm", "Work", "--dry-run"}, "(forbidden)")
	wantIn(t, h.ok("project", "list"), "Work", "未完成 1")

	trusted := h.agent("Trusted", false)
	wantIn(t, trusted.ok("project", "rm", "Work", "--dry-run"), "试运行")
	wantIn(t, trusted.ok("project", "list"), "Work")
	wantIn(t, trusted.ok("project", "rm", "Work", "--reason", "the user asked"), "已删除项目「Work」")
	wantIn(t, trusted.ok("project", "list"), "（还没有项目）")
	wantIn(t, trusted.ok("item", "list", "--view", "all"), "（没有事项）")
	h.fails(1, []string{"project", "rm", "Work"}, "no project is named")
}

func TestItemHeadingsAndFilters(t *testing.T) {
	h := newHarness(t, false)
	for _, project := range []string{"Work", "Home"} {
		h.ok("project", "add", project)
		h.ok("heading", "add", project, "本周")
	}
	var created struct {
		Item api.Item `json:"item"`
	}
	h.json(&created, "item", "add", "写周报", "--project", "work", "--heading", "本周", "--evening",
		"--due", "2026-10-20 09:30", "--notes", "含 CalDAV 进度")
	it := created.Item
	if it.HeadingID == nil || !it.Evening || *it.DueDate != "2026-10-20" || it.DueTime == nil || *it.DueTime != "09:30" {
		t.Fatalf("created %+v", it)
	}
	wantIn(t, h.ok("item", "show", it.ID), "写周报", "分组 本周", "今晚", "截止 2026-10-20 09:30", "含 CalDAV 进度")

	// A bare name needs a project; PROJECT/NAME carries its own.
	h.fails(1, []string{"item", "add", "x", "--heading", "本周"}, "the project is not known")
	h.fails(2, []string{"item", "add", "x", "--project", "none", "--heading", "本周"}, "--heading needs a project")
	h.fails(1, []string{"item", "add", "x", "--project", "Work", "--heading", "nope"}, "no heading matches")
	h.json(&created, "item", "add", "买菜", "--heading", "home/本周")
	home := created.Item
	if home.ProjectID == nil || home.HeadingID == nil {
		t.Fatalf("an item added under PROJECT/NAME: %+v", home)
	}
	wantIn(t, h.ok("item", "show", home.ID), "Home", "分组 本周")

	// Editing resolves a name in the project the item is in.
	wantOut(t, h.ok("item", "edit", it.ID, "--heading", "none", "--no-evening", "--due", "fri 18:00"), "今晚")
	out := h.ok("item", "show", it.ID)
	wantIn(t, out, "截止 2026-10-16 18:00")
	wantOut(t, out, "分组", "今晚")
	h.ok("item", "edit", it.ID[:6], "--heading", "本周")
	wantIn(t, h.ok("item", "show", it.ID), "分组 本周", "Work")
	h.fails(1, []string{"item", "edit", it.ID, "--heading", "home/本周", "--project", "Work"}, "different project")
	h.ok("item", "edit", it.ID, "--project", "Home", "--heading", "本周")
	wantIn(t, h.ok("item", "show", it.ID), "Home", "分组 本周")
	h.ok("item", "edit", it.ID, "--project", "Work", "--heading", "本周")

	h.json(&created, "item", "add", "收件箱里的", "--important", "--due", "today")
	inbox := created.Item
	h.fails(1, []string{"item", "edit", inbox.ID, "--heading", "本周"}, "the project is not known")

	// Two headings of one project may share a name; then only the ID tells them apart.
	var second struct {
		Heading api.Heading `json:"heading"`
	}
	h.json(&second, "heading", "add", "Work", "本周")
	h.fails(1, []string{"item", "edit", it.ID, "--heading", "本周"}, "2 headings are named", "heading list Work")
	h.fails(1, []string{"heading", "rm", "Work/本周"}, "2 headings are named")
	h.ok("item", "edit", it.ID, "--heading", second.Heading.ID[:8])
	h.ok("heading", "rm", second.Heading.ID)
	wantOut(t, h.ok("item", "show", it.ID), "分组")
	h.ok("item", "edit", it.ID, "--heading", "本周")

	// Filters.
	h.ok("item", "add", "已完成的", "--project", "Work")
	h.ok("item", "done", h.itemID("已完成的"))
	out = h.ok("item", "list", "--query", "caldav")
	wantIn(t, out, "写周报")
	wantOut(t, out, "买菜", "收件箱里的")
	out = h.ok("item", "list", "--project", "Work", "--heading", "本周")
	wantIn(t, out, "写周报")
	wantOut(t, out, "已完成的")
	wantIn(t, h.ok("item", "list", "--heading", "Home/本周"), "买菜")
	out = h.ok("item", "list", "--project", "Work", "--heading", "none", "--status", "any")
	wantIn(t, out, "[x]", "已完成的")
	wantOut(t, out, "写周报")
	wantOut(t, h.ok("item", "list", "--project", "Work", "--status", "done"), "写周报")
	out = h.ok("item", "list", "--quadrant", "do")
	wantIn(t, out, "收件箱里的")
	wantOut(t, out, "写周报")
	wantIn(t, h.ok("item", "list", "--quadrant", "later"), "买菜")
	wantIn(t, h.ok("item", "list", "--view", "done", "--query", "完成"), "已完成的")
	wantIn(t, h.ok("item", "list", "--query", "nothing like this"), "（没有事项）")

	h.fails(1, []string{"item", "list", "--quadrant", "soon"}, "invalid_request")
	h.fails(1, []string{"item", "list", "--status", "maybe"}, "invalid_request")
	h.fails(1, []string{"item", "list", "--cursor", "!!"}, "cursor is not valid")
	h.fails(2, []string{"item", "list", "--view", "today", "--query", "x"}, "takes no filters")
	h.fails(2, []string{"item", "list", "--view", "matrix", "--all"}, "takes no filters")
	h.fails(2, []string{"item", "list", "--view", "done", "--status", "open"}, "--view done")
	h.fails(2, []string{"item", "list", "--view", "inbox", "--project", "Work"}, "--view inbox")
	h.fails(1, []string{"item", "list", "--heading", "本周"}, "the project is not known")
}

func TestItemListPagination(t *testing.T) {
	h := newHarness(t, true)
	const total = 205
	for i := range total {
		h.me.Call("POST", "/items", map[string]any{"title": fmt.Sprintf("事项 %03d", i)}, http.StatusCreated, nil)
	}
	type page struct {
		Items      []api.Item `json:"items"`
		NextCursor *string    `json:"next_cursor"`
		Total      int        `json:"total"`
	}
	cursorIn := func(out string) string {
		t.Helper()
		m := regexp.MustCompile(`--cursor (\S+?)(（|\n|$)`).FindStringSubmatch(out)
		if m == nil {
			t.Fatalf("no --cursor to continue with:\n%s", out)
		}
		return m[1]
	}

	// One page at a time: the last line says how to continue.
	out := h.ok("item", "list", "--view", "all", "--limit", "100")
	wantIn(t, out, "事项 000", "事项 099", "共 205 件，本页 100 件", "--all")
	wantOut(t, out, "事项 100")
	out = h.ok("item", "list", "--limit", "100", "--cursor", cursorIn(out))
	wantIn(t, out, "事项 100", "事项 199", "本页 100 件")
	out = h.ok("item", "list", "--limit", "100", "--cursor", cursorIn(out))
	wantIn(t, out, "事项 200", "事项 204")
	wantOut(t, out, "--cursor", "事项 199")

	var first page
	h.json(&first, "item", "list", "--view", "inbox", "--limit", "3")
	if len(first.Items) != 3 || first.NextCursor == nil || first.Total != total {
		t.Fatalf("first page: %d items, cursor %v, total %d", len(first.Items), first.NextCursor, first.Total)
	}

	// --all follows the cursors and prints one object.
	out = h.ok("item", "list", "--all")
	if lines := strings.Count(out, "\n"); lines != total {
		t.Fatalf("--all printed %d lines, want %d", lines, total)
	}
	wantOut(t, out, "--cursor")
	var all page
	h.json(&all, "item", "list", "--all", "--limit", "7")
	if len(all.Items) != total || all.NextCursor != nil || all.Total != total || all.Items[204].Title != "事项 204" {
		t.Fatalf("--all: %d items, cursor %v, total %d", len(all.Items), all.NextCursor, all.Total)
	}
	h.json(&all, "item", "list", "--all", "--query", "事项 20")
	if len(all.Items) != 5 || all.Total != 5 {
		t.Fatalf("--all with a filter: %d items, total %d", len(all.Items), all.Total)
	}

	// It stops at the cap and says where to pick up.
	defer cli.SetMaxAllRows(150)()
	out = h.ok("item", "list", "--all")
	wantIn(t, out, "事项 149", "共 205 件，已到 150 件上限")
	wantOut(t, out, "事项 150")
	out = h.ok("item", "list", "--all", "--cursor", cursorIn(out))
	wantIn(t, out, "事项 150", "事项 204")
	wantOut(t, out, "--cursor")
	h.json(&all, "item", "list", "--all")
	if len(all.Items) != 150 || all.NextCursor == nil {
		t.Fatalf("capped --all: %d items, cursor %v", len(all.Items), all.NextCursor)
	}
}

func TestAllDayEvents(t *testing.T) {
	h := newHarness(t, false)
	var created struct {
		Event api.Event `json:"event"`
	}
	h.json(&created, "event", "add", "出差", "--all-day", "--start", "thu", "--end", "fri", "--notes", "带电脑", "--location", "上海")
	ev := created.Event
	if !ev.AllDay || *ev.StartDate != "2026-10-15" || *ev.EndDate != "2026-10-16" || ev.Start != nil || ev.Notes != "带电脑" || ev.Location != "上海" {
		t.Fatalf("created %+v", ev)
	}
	wantIn(t, h.ok("event", "add", "生日", "--all-day", "--start", "2026-10-20"), "2026-10-20 周二", "全天")
	h.fails(2, []string{"event", "add", "x", "--all-day", "--start", "thu", "--duration", "1h"}, "--all-day takes --start DATE")
	h.fails(2, []string{"event", "add", "x", "--all-day"}, "--all-day takes --start DATE")
	h.fails(1, []string{"event", "add", "x", "--all-day", "--start", "fri", "--end", "thu"}, "end_date must not be before start_date")

	// A new first day keeps the number of days; --end changes it.
	id := ev.ID[:6]
	wantIn(t, h.ok("event", "edit", id, "--start", "2026-10-19"), "2026-10-19 周一", "全天 至 10-20")
	wantIn(t, h.ok("event", "move", id, "--start", "2026-10-21", "--dry-run"), "试运行", "2026-10-21 周三", "全天 至 10-22")
	wantIn(t, h.ok("event", "edit", id, "--end", "2026-10-22", "--title", "长差"), "2026-10-19 周一", "全天 至 10-22", "长差")
	wantIn(t, h.ok("event", "edit", id, "--start", "2026-10-15", "--end", "2026-10-15"), "2026-10-15 周四", "全天  ")
	h.fails(2, []string{"event", "edit", id, "--duration", "1h"}, "not --duration", "--timed")
	h.fails(1, []string{"event", "edit", id, "--start", "2026-10-15 10:00"}, "as a date")

	// To a timed event and back.
	h.fails(2, []string{"event", "edit", id, "--timed"}, "--start is required")
	h.fails(2, []string{"event", "edit", id, "--timed", "--start", "thu 10:00"}, "exactly one of --end and --duration")
	h.fails(2, []string{"event", "edit", id, "--timed", "--all-day"}, "only one of --all-day and --timed")
	wantIn(t, h.ok("event", "edit", id, "--timed", "--start", "thu 10:00", "--duration", "90m"), "2026-10-15 周四", "10:00-11:30")
	h.json(&created, "event", "edit", id, "--location", "北京")
	if ev = created.Event; ev.AllDay || *ev.Start != "2026-10-15T02:00:00Z" || ev.StartDate != nil || ev.Location != "北京" || ev.Notes != "带电脑" {
		t.Fatalf("timed %+v", ev)
	}
	wantIn(t, h.ok("event", "edit", id, "--all-day"), "2026-10-15 周四", "全天  ")
	wantIn(t, h.ok("event", "edit", id, "--timed", "--start", "fri 09:00", "--end", "fri 10:00"), "09:00-10:00")
	wantIn(t, h.ok("event", "edit", id, "--all-day", "--start", "2026-10-26", "--end", "2026-10-28"), "2026-10-26 周一", "全天 至 10-28")
	h.json(&created, "event", "edit", id, "--notes", "")
	if ev = created.Event; !ev.AllDay || *ev.StartDate != "2026-10-26" || *ev.EndDate != "2026-10-28" || ev.Start != nil || ev.Notes != "" {
		t.Fatalf("all-day again %+v", ev)
	}
	wantIn(t, h.ok("agenda", "--date", "2026-10-27"), "长差")
}

func TestSuggestDeleteWithdrawAndRedo(t *testing.T) {
	h := newHarness(t, true)
	var item struct {
		Item api.Item `json:"item"`
	}
	h.json(&item, "item", "add", "写周报")
	var event struct {
		Event api.Event `json:"event"`
	}
	h.json(&event, "event", "add", "设计评审", "--start", "14:00", "--duration", "1h")

	h.fails(2, []string{"suggest", "delete-item", item.Item.ID}, "--reason is required")
	wantIn(t, h.ok("suggest", "delete-item", item.Item.ID[:6], "--reason", "重复了"), "已提出建议", "删除事项「写周报」", "重复了")
	wantIn(t, h.ok("suggest", "delete-event", event.Event.ID[:6], "--reason", "已取消", "--dry-run"), "试运行")
	var proposed struct {
		Suggestion api.Suggestion `json:"suggestion"`
	}
	h.json(&proposed, "suggest", "delete-event", event.Event.ID[:6], "--reason", "已取消")
	if proposed.Suggestion.Kind != "delete_event" || *proposed.Suggestion.EventID != event.Event.ID {
		t.Fatalf("proposed %+v", proposed.Suggestion)
	}
	wantIn(t, h.ok("suggest", "list"), "删除事项「写周报」", "删除日程「设计评审」")
	wantIn(t, h.ok("agenda"), "设计评审", "待调整")
	h.fails(1, []string{"suggest", "delete-item", "zzzzzz", "--reason", "x"}, "no item")

	// Another token cannot withdraw it; the one that made it can, by prefix.
	other := h.agent("Other", true)
	id := proposed.Suggestion.ID
	other.fails(1, []string{"suggest", "withdraw", id}, "suggestion not found", "(not_found)")
	wantIn(t, h.ok("suggest", "withdraw", id[:6], "--dry-run"), "试运行")
	wantIn(t, h.ok("suggest", "list"), "删除日程「设计评审」")
	wantIn(t, h.ok("suggest", "withdraw", id[:6]), "已撤回建议 "+id[:8]+"「设计评审」")
	wantOut(t, h.ok("suggest", "list"), "设计评审")
	wantOut(t, h.ok("agenda"), "待调整")
	h.fails(1, []string{"suggest", "withdraw", id[:6]}, "no pending suggestion")
	h.fails(1, []string{"suggest", "withdraw", id}, "(not_found)")

	// rm became a suggestion; --json on a withdrawal prints an empty object.
	wantIn(t, h.ok("event", "rm", event.Event.ID, "--reason", "已取消"), "等待用户确认")
	var list struct {
		Suggestions []api.Suggestion `json:"suggestions"`
	}
	h.json(&list, "suggest", "list")
	if len(list.Suggestions) != 2 {
		t.Fatalf("%d suggestions pending", len(list.Suggestions))
	}
	if out := h.ok("suggest", "withdraw", list.Suggestions[1].ID, "--json"); strings.TrimSpace(out) != "{}" {
		t.Fatalf("withdraw --json printed %q", out)
	}

	// Once the user has decided, there is nothing left to withdraw.
	decided := list.Suggestions[0].ID
	h.me.Call("POST", "/suggestions/"+decided+"/reject", nil, http.StatusOK, nil)
	h.fails(1, []string{"suggest", "withdraw", decided}, "already rejected", "(conflict)")
	wantIn(t, h.ok("suggest", "list", "--status", "rejected"), "[rejected]")
	wantIn(t, h.ok("suggest", "list"), "（没有建议）")

	// Undo, then redo by the ID that undo and the activity log print.
	h.ok("item", "done", item.Item.ID)
	var log struct {
		Activities []api.Activity `json:"activities"`
	}
	h.json(&log, "activity")
	done := log.Activities[0].ID
	wantIn(t, h.ok("undo"), "已撤销：完成了「写周报」", done[:8])
	wantIn(t, h.ok("item", "show", item.Item.ID), "[ ]")
	wantIn(t, h.ok("redo", done[:6], "--dry-run"), "已重做", "试运行")
	wantIn(t, h.ok("item", "show", item.Item.ID), "[ ]")
	wantIn(t, h.ok("redo", done[:6]), "已重做：完成了「写周报」")
	wantIn(t, h.ok("item", "show", item.Item.ID), "[x]")
	wantOut(t, h.ok("activity"), "[已撤销]")
	h.fails(1, []string{"redo", done}, "(conflict)")
	h.fails(1, []string{"redo", "zzzzzz"}, "no activity")
}
