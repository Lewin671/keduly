package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/cli"
	"github.com/Lewin671/keduly/internal/testutil"
)

type harness struct {
	t      *testing.T
	s      *testutil.Server
	me     *testutil.Client
	config string
	env    map[string]string
}

func newHarness(t *testing.T, confirmDelete bool) *harness {
	t.Helper()
	s := testutil.New(t, testutil.Options{})
	me := s.Register("me@example.com")
	h := &harness{t: t, s: s, me: me, config: t.TempDir()}
	token := me.NewToken("Claude Code · test", "agent", "write", confirmDelete).Token
	h.env = map[string]string{"XDG_CONFIG_HOME": h.config}
	if out, code := h.run("login", "--server", s.URL, "--token", token); code != 0 {
		t.Fatalf("login failed: %s", out)
	}
	return h
}

// run executes the CLI and returns stdout (plus stderr on failure) and the exit code.
func (h *harness) run(args ...string) (string, int) {
	h.t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Run(args, cli.Env{
		Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr, Version: "test",
		Getenv: func(k string) string { return h.env[k] },
		Now:    func() time.Time { return testutil.Clock },
	})
	if code != 0 {
		return stdout.String() + stderr.String(), code
	}
	return stdout.String(), code
}

func (h *harness) ok(args ...string) string {
	h.t.Helper()
	out, code := h.run(args...)
	if code != 0 {
		h.t.Fatalf("keduly %s: exit %d: %s", strings.Join(args, " "), code, out)
	}
	return out
}

func (h *harness) json(out any, args ...string) {
	h.t.Helper()
	raw := h.ok(append(args, "--json")...)
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		h.t.Fatalf("keduly %s --json printed %q: %v", strings.Join(args, " "), raw, err)
	}
}

func wantIn(t *testing.T, out string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(out, p) {
			t.Fatalf("output lacks %q:\n%s", p, out)
		}
	}
}

func TestLoginWritesPrivateConfig(t *testing.T) {
	h := newHarness(t, true)
	path := filepath.Join(h.config, "keduly", "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode %v, want 0600", info.Mode().Perm())
	}
	data, _ := os.ReadFile(path)
	var cfg map[string]string
	if json.Unmarshal(data, &cfg) != nil || cfg["server"] != h.s.URL || !strings.HasPrefix(cfg["token"], "kdl_") {
		t.Fatalf("config %s", data)
	}
	wantIn(t, h.ok("whoami"), "me@example.com", "Asia/Shanghai", "Claude Code · test")
	wantIn(t, h.ok("version"), "keduly test")
	wantIn(t, h.ok("help"), "keduly item add")

	if out, code := h.run("login", "--server", h.s.URL, "--token", "kdl_wrong"); code == 0 || !strings.Contains(out, "unauthenticated") {
		t.Fatalf("login with a bad token: %d %s", code, out)
	}
	// The environment overrides the file.
	h.env["KEDULY_TOKEN"] = "kdl_wrong"
	if out, code := h.run("whoami"); code == 0 {
		t.Fatalf("KEDULY_TOKEN was ignored: %s", out)
	}
	delete(h.env, "KEDULY_TOKEN")
	empty := &harness{t: t, s: h.s, env: map[string]string{"XDG_CONFIG_HOME": t.TempDir()}}
	if out, code := empty.run("whoami"); code == 0 || !strings.Contains(out, "not signed in") {
		t.Fatalf("without a config: %d %s", code, out)
	}
	if out, code := h.run("frobnicate"); code != 2 {
		t.Fatalf("unknown command: %d %s", code, out)
	}
}

func TestItemsAndProjects(t *testing.T) {
	h := newHarness(t, true)
	wantIn(t, h.ok("project", "add", "Keduly 开发", "--color", "teal", "--area", "工作"), "Keduly 开发", "teal")
	wantIn(t, h.ok("project", "list"), "Keduly 开发", "工作", "未完成 0")

	out := h.ok("item", "add", "实现 CalDAV 同步", "--project", "keduly 开发", "--when", "today", "--due", "fri 18:00",
		"--estimate", "1h30m", "--important", "--notes", "RFC 4791", "--reason", "asked by the user")
	wantIn(t, out, "已新建事项", "实现 CalDAV 同步", "Keduly 开发", "计划 2026-10-13", "截止 2026-10-16 18:00", "预计 1h30m", "重要")

	var created struct {
		Item api.Item `json:"item"`
	}
	h.json(&created, "item", "add", "写周报", "--when", "tomorrow", "--estimate", "45")
	id := created.Item.ID
	if len(id) != 16 || *created.Item.PlannedDate != "2026-10-14" || *created.Item.EstimateMinutes != 45 || created.Item.CreatedBy.Name != "Claude Code · test" {
		t.Fatalf("created %+v", created.Item)
	}

	wantIn(t, h.ok("item", "list"), "实现 CalDAV 同步", "空闲 9h")
	wantIn(t, h.ok("item", "list", "--view", "all"), "实现 CalDAV 同步", "写周报")
	wantIn(t, h.ok("item", "list", "--view", "inbox"), "写周报")
	wantIn(t, h.ok("item", "list", "--view", "upcoming"), "2026-10-14 周三", "写周报")
	wantIn(t, h.ok("item", "list", "--view", "matrix"), "重要不紧急（1 件")
	wantIn(t, h.ok("item", "list", "--project", "Keduly 开发"), "实现 CalDAV 同步")
	if out := h.ok("item", "list", "--view", "inbox"); strings.Contains(out, "实现 CalDAV 同步") {
		t.Fatalf("the inbox lists a project item:\n%s", out)
	}

	// A unique prefix is enough; flags may come before or after it.
	prefix := id[:6]
	wantIn(t, h.ok("item", "show", prefix), "写周报", id)
	wantIn(t, h.ok("item", "edit", "--title", "写本周周报", prefix, "--due", "2026-10-15", "--when", "none"), "写本周周报", "截止 2026-10-15")
	if out, code := h.run("item", "show", "zz"); code == 0 || !strings.Contains(out, "too short") {
		t.Fatalf("a two-character prefix: %d %s", code, out)
	}
	if out, code := h.run("item", "show", "zzzzzz"); code == 0 || !strings.Contains(out, "no item") {
		t.Fatalf("an unknown prefix: %d %s", code, out)
	}

	// A dry run changes nothing.
	wantIn(t, h.ok("item", "done", prefix, "--dry-run"), "试运行")
	wantIn(t, h.ok("item", "show", prefix), "[ ]")
	wantIn(t, h.ok("item", "done", prefix), "已完成", "[x]")
	wantIn(t, h.ok("item", "list", "--view", "done"), "写本周周报")
	wantIn(t, h.ok("item", "reopen", prefix), "已重新打开", "[ ]")

	wantIn(t, h.ok("item", "schedule", prefix, "--start", "14:00", "--duration", "1h"), "已安排时间", "已排 10-13 周二 14:00-15:00")
	wantIn(t, h.ok("agenda"), "2026-10-13 周二", "14:00-15:00", "写本周周报", "时间块")
	wantIn(t, h.ok("free", "--date", "today", "--duration", "60m"), "09:00-14:00", "15:00-18:00")
	wantIn(t, h.ok("item", "unschedule", prefix), "已取消时间安排")

	// This token must have its deletions confirmed.
	wantIn(t, h.ok("item", "rm", prefix, "--reason", "duplicate"), "等待用户确认")
	wantIn(t, h.ok("item", "show", prefix), "写本周周报")
	wantIn(t, h.ok("suggest", "list"), "删除事项「写本周周报」", "duplicate")

	out = h.ok("activity", "--limit", "50")
	wantIn(t, out, "新建事项「实现 CalDAV 同步」", "原因：asked by the user", "完成了「写本周周报」", "Claude Code · test")
}

func TestEventsSuggestionsAndUndo(t *testing.T) {
	h := newHarness(t, false)
	var created struct {
		Event api.Event `json:"event"`
	}
	h.json(&created, "event", "add", "设计评审", "--start", "2026-10-13 10:00", "--end", "11:30", "--location", "3F")
	if *created.Event.Start != "2026-10-13T02:00:00Z" || *created.Event.End != "2026-10-13T03:30:00Z" || created.Event.Location != "3F" {
		t.Fatalf("created %+v", created.Event)
	}
	id := created.Event.ID
	wantIn(t, h.ok("event", "add", "出差", "--all-day", "--start", "thu", "--end", "fri"), "已新建日程", "2026-10-15 周四", "全天")
	wantIn(t, h.ok("event", "list"), "10:00-11:30", "设计评审", "@3F", "出差")
	wantIn(t, h.ok("event", "list", "--from", "2026-10-15", "--to", "2026-10-15"), "出差")

	wantIn(t, h.ok("event", "move", id[:5], "--start", "tomorrow 15:00"), "已修改日程", "2026-10-14 周三", "15:00-16:30")
	wantIn(t, h.ok("event", "edit", id[:5], "--duration", "30m", "--title", "设计评审（短）"), "15:00-15:30", "设计评审（短）")

	// Undo without an ID takes this token's most recent change.
	wantIn(t, h.ok("undo"), "已撤销")
	wantIn(t, h.ok("event", "list"), "15:00-16:30")
	wantIn(t, h.ok("activity"), "[已撤销]")

	// Suggestions.
	if out, code := h.run("suggest", "move", id[:5], "--start", "fri 10:00"); code != 2 || !strings.Contains(out, "--reason") {
		t.Fatalf("a suggestion without a reason: %d %s", code, out)
	}
	wantIn(t, h.ok("suggest", "move", id[:5], "--start", "fri 10:00", "--reason", "和客户演示冲突"), "已提出建议", "移动日程", "10-16 周五 10:00-11:30")
	var item struct {
		Item api.Item `json:"item"`
	}
	h.json(&item, "item", "add", "写接口文档", "--estimate", "1h")
	wantIn(t, h.ok("suggest", "schedule", item.Item.ID, "--start", "thu 13:00", "--duration", "1h", "--reason", "周四下午有空"), "安排时间「写接口文档」")
	wantIn(t, h.ok("suggest", "add-item", "准备评审材料", "--estimate", "30m", "--start", "09:30", "--duration", "30m", "--reason", "评审前准备"), "新建事项「准备评审材料」")
	wantIn(t, h.ok("suggest", "add-event", "复盘", "--start", "17:00", "--duration", "45m", "--reason", "本周收尾"), "新建日程「复盘」")
	wantIn(t, h.ok("suggest", "add-event", "不写入", "--start", "17:00", "--duration", "45m", "--reason", "x", "--dry-run"), "试运行")

	var list struct {
		Suggestions []api.Suggestion `json:"suggestions"`
	}
	h.json(&list, "suggest", "list")
	if len(list.Suggestions) != 4 {
		t.Fatalf("%d suggestions", len(list.Suggestions))
	}
	out := h.ok("agenda", "--date", "2026-10-13", "--days", "4")
	wantIn(t, out, "2026-10-13 周二", "09:30-10:00", "准备评审材料", "待定", "17:00-17:45", "复盘", "2026-10-15 周四", "写接口文档", "2026-10-16 周五", "待调整")

	// The user accepts everything; the agenda settles.
	h.me.Call("POST", "/suggestions/accept-all", nil, http.StatusOK, nil)
	out = h.ok("agenda", "--date", "2026-10-13", "--days", "4")
	if strings.Contains(out, "待定") || strings.Contains(out, "待调整") {
		t.Fatalf("the agenda still shows pending entries:\n%s", out)
	}
	wantIn(t, out, "10:00-11:30  "+id[:8])
	wantIn(t, h.ok("suggest", "list"), "（没有建议）")

	// With confirm_delete off, rm deletes.
	wantIn(t, h.ok("event", "rm", id[:5], "--reason", "cancelled"), "已删除日程")
	if out := h.ok("event", "list", "--from", "2026-10-16", "--to", "2026-10-16"); strings.Contains(out, "设计评审") {
		t.Fatalf("the event is still listed:\n%s", out)
	}
	wantIn(t, h.ok("undo", "--dry-run"), "已撤销：删除日程")
}
