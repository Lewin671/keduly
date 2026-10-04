package core

import (
	"fmt"
	"time"

	"github.com/Lewin671/keduly/internal/store"
)

// The activity log is shown to the user as is, so its summaries are written in
// the language of the interface.

var weekdays = [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

func quote(title string) string {
	if title == "" {
		title = "无标题"
	}
	return "「" + title + "」"
}

// dayLabel names a date relative to today: 今天, 明天, a weekday within the
// coming and past week, or the month and day.
func (op *Op) dayLabel(date string) string {
	d := dayStart(date, time.UTC)
	today := dayStart(op.today(), time.UTC)
	diff := int(d.Sub(today).Hours() / 24)
	switch {
	case diff == 0:
		return "今天"
	case diff == 1:
		return "明天"
	case diff == -1:
		return "昨天"
	case diff > -7 && diff < 7:
		return weekdays[d.Weekday()]
	case d.Year() == today.Year():
		return fmt.Sprintf("%d月%d日", int(d.Month()), d.Day())
	}
	return fmt.Sprintf("%d年%d月%d日", d.Year(), int(d.Month()), d.Day())
}

func (op *Op) whenLabel(t time.Time) string {
	local := t.In(op.Loc)
	return op.dayLabel(local.Format(dateLayout)) + " " + local.Format("15:04")
}

// eventWhen describes where an event sits on the calendar.
func (op *Op) eventWhen(ev *store.Event) string {
	if ev.AllDay {
		if *ev.StartDate == *ev.EndDate {
			return op.dayLabel(*ev.StartDate)
		}
		return op.dayLabel(*ev.StartDate) + "至" + op.dayLabel(*ev.EndDate)
	}
	return op.whenLabel(store.ParseTime(*ev.StartAt))
}

// movedSummary describes an event or time block moving from one slot to another.
func (op *Op) movedSummary(title string, before, after *store.Event) string {
	if !before.AllDay && !after.AllDay {
		a, b := store.ParseTime(*before.StartAt).In(op.Loc), store.ParseTime(*after.StartAt).In(op.Loc)
		if a.Format(dateLayout) == b.Format(dateLayout) && !a.Equal(b) {
			return fmt.Sprintf("%s从 %s 改到 %s", quote(title), a.Format("15:04"), b.Format("15:04"))
		}
		if a.Equal(b) {
			return fmt.Sprintf("%s的时长改为 %s", quote(title), minutesLabel(eventMinutes(after)))
		}
	}
	return fmt.Sprintf("%s从 %s 改到 %s", quote(title), op.eventWhen(before), op.eventWhen(after))
}

func eventMinutes(ev *store.Event) int {
	return int(store.ParseTime(*ev.EndAt).Sub(store.ParseTime(*ev.StartAt)).Minutes())
}

func minutesLabel(m int) string {
	switch {
	case m < 60:
		return fmt.Sprintf("%d 分钟", m)
	case m%60 == 0:
		return fmt.Sprintf("%d 小时", m/60)
	}
	return fmt.Sprintf("%d 小时 %d 分钟", m/60, m%60)
}

func sameSlot(a, b *store.Event) bool {
	return a.AllDay == b.AllDay && eqStr(a.StartAt, b.StartAt) && eqStr(a.EndAt, b.EndAt) &&
		eqStr(a.StartDate, b.StartDate) && eqStr(a.EndDate, b.EndDate)
}

func eqStr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// itemUpdateSummary picks the most telling description of an item edit.
func (op *Op) itemUpdateSummary(before, after *store.Item) string {
	t := quote(after.Title)
	switch {
	case before.Title != after.Title:
		return fmt.Sprintf("%s改名为%s", quote(before.Title), t)
	case !eqStr(before.ProjectID, after.ProjectID):
		if after.ProjectID == nil {
			return t + "移到收件箱"
		}
		if p, _ := store.Projects.Get(op.ctx, op.q, op.User.ID, *after.ProjectID); p != nil {
			return t + "移到项目" + quote(p.Name)
		}
	case !eqStr(before.DueDate, after.DueDate) || !eqStr(before.DueTime, after.DueTime):
		if after.DueDate == nil {
			return "取消了" + t + "的截止时间"
		}
		label := op.dayLabel(*after.DueDate)
		if after.DueTime != nil {
			label += " " + *after.DueTime
		}
		return t + "的截止时间改为 " + label
	case !eqStr(before.PlannedDate, after.PlannedDate):
		if after.PlannedDate == nil {
			return "取消了" + t + "的计划日期"
		}
		return t + "计划到 " + op.dayLabel(*after.PlannedDate)
	case before.Important != after.Important:
		if after.Important {
			return "将" + t + "标为重要"
		}
		return "取消了" + t + "的重要标记"
	}
	return "修改了" + t
}
