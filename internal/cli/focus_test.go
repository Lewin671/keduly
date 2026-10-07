package cli_test

import (
	"testing"
	"time"

	"github.com/Lewin671/keduly/internal/api"
)

func TestFocus(t *testing.T) {
	h := newHarness(t, true)
	wantIn(t, h.ok("focus", "status"), "计时器空闲", "今天 0 个番茄", "本轮 0/4")
	wantIn(t, h.ok("focus", "log"), "没有专注记录")
	wantIn(t, h.ok("whoami"), "番茄 25m · 休息 5m · 每 4 个番茄后长休息 15m")

	var added struct {
		Item api.Item `json:"item"`
	}
	h.json(&added, "item", "add", "实现同步", "--estimate", "90m")
	id := added.Item.ID

	// A dry run starts nothing.
	wantIn(t, h.ok("focus", "start", id[:6], "--dry-run"), "试运行", "专注中：「实现同步」")
	wantIn(t, h.ok("focus", "status"), "计时器空闲")

	wantIn(t, h.ok("focus", "start", id[:6]), "已开始一个番茄", "专注中：「实现同步」", "还剩 25m0s", "到 10:25")
	h.fails(1, []string{"focus", "rest"}, "a tomato is running", "conflict")
	wantIn(t, h.ok("focus", "log"), "10:00-10:25", "进行中", "「实现同步」")

	// The tomato runs out on its own.
	h.s.SetNow(h.s.Now().Add(25 * time.Minute))
	wantIn(t, h.ok("focus", "status"), "番茄到点了：「实现同步」", "今天 1 个番茄 · 25m · 本轮 1/4", "keduly focus rest")
	wantIn(t, h.ok("item", "show", id), "番茄 1 · 已用 25m")
	wantIn(t, h.ok("focus", "rest"), "已开始休息", "休息中 · 还剩 5m0s，到 10:30")
	wantIn(t, h.ok("focus", "stop"), "已停止计时", "计时器空闲")

	// Free focus, given up after ten minutes: minutes but no tomato.
	wantIn(t, h.ok("focus", "start"), "专注中：自由专注")
	h.s.SetNow(h.s.Now().Add(10 * time.Minute))
	wantIn(t, h.ok("focus", "stop"), "今天 1 个番茄 · 35m")
	out := h.ok("focus", "log")
	wantIn(t, out, "25m  番茄  「实现同步」", "10m  未完成  自由专注")

	// The free session is named and filed under a project afterwards, then moved to the item.
	var log struct {
		Sessions []api.FocusSession `json:"sessions"`
	}
	h.json(&log, "focus", "log")
	free := log.Sessions[1].ID
	wantIn(t, out, free[:8])
	h.ok("project", "add", "Keduly")
	wantIn(t, h.ok("focus", "edit", free[:8], "--title", "读 RFC", "--project", "Keduly", "--dry-run"), "试运行", "「读 RFC」  (Keduly)")
	wantIn(t, h.ok("focus", "log"), "未完成  自由专注")
	wantIn(t, h.ok("focus", "edit", free[:8], "--title", "读 RFC", "--project", "Keduly"), "已修改专注记录", "「读 RFC」  (Keduly)")
	wantIn(t, h.ok("focus", "log"), "未完成  「读 RFC」  (Keduly)")
	wantIn(t, h.ok("focus", "stats"), "Keduly")
	wantIn(t, h.ok("focus", "edit", free, "--item", id[:6]), "「实现同步」")
	wantIn(t, h.ok("item", "show", id), "番茄 1 · 已用 35m")
	h.fails(1, []string{"focus", "edit", free, "--title", "x"}, "follow the session's item")
	wantIn(t, h.ok("focus", "edit", free, "--item", "none"), "自由专注")
	h.fails(2, []string{"focus", "edit", free}, "nothing to change")
	h.fails(1, []string{"focus", "edit", "zzzz", "--title", "x"}, "no focus session")

	wantIn(t, h.ok("focus", "start", "--title", "读 RFC", "--project", "Keduly", "--dry-run"), "专注中：「读 RFC」")
	h.fails(1, []string{"focus", "start", id[:6], "--title", "x"}, "follow the session's item")

	wantIn(t, h.ok("focus", "stats"), "最近 7 天：1 个番茄 · 35m · 连续 1 天", "2026-10-13 周二   1 个番茄  35m", "未归项目")
	var stats api.FocusStats
	h.json(&stats, "focus", "stats")
	if len(stats.Days) != 7 || stats.Tomatoes != 1 {
		t.Fatalf("stats %+v", stats)
	}
	var status struct {
		Focus api.Focus `json:"focus"`
	}
	h.json(&status, "focus", "status")
	if status.Focus.State != "idle" || status.Focus.TomatoesToday != 1 {
		t.Fatalf("focus %+v", status.Focus)
	}

	h.fails(2, []string{"focus", "start", "a", "b"}, "usage: keduly focus start [ITEM_ID]")
	h.fails(2, []string{"focus", "log", "--from", "someday"}, "cannot read")
	wantIn(t, h.ok("focus", "start", "--help"), "--dry-run", "--reason")
}
