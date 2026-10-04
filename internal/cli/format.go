package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/Lewin671/keduly/internal/api"
)

var weekdayLabels = [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func localTime(stamp string, loc *time.Location) time.Time {
	t, _ := time.Parse(time.RFC3339, stamp)
	return t.In(loc)
}

// span formats a start and end as "10-13 周二 14:00-15:30".
func span(start, end string, loc *time.Location) string {
	s, e := localTime(start, loc), localTime(end, loc)
	out := fmt.Sprintf("%s %s %s-", s.Format("01-02"), weekdayLabels[s.Weekday()], s.Format("15:04"))
	if s.Format(dateLayout) != e.Format(dateLayout) {
		out += e.Format("01-02 ")
	}
	return out + e.Format("15:04")
}

func dateLabel(date string) string {
	t, err := time.Parse(dateLayout, date)
	if err != nil {
		return date
	}
	return date + " " + weekdayLabels[t.Weekday()]
}

func minutesLabel(m int) string {
	switch {
	case m < 60:
		return fmt.Sprintf("%dm", m)
	case m%60 == 0:
		return fmt.Sprintf("%dh", m/60)
	}
	return fmt.Sprintf("%dh%dm", m/60, m%60)
}

func (a *app) itemLine(it api.Item, projects map[string]string) string {
	box := "[ ]"
	if it.Status == "done" {
		box = "[x]"
	}
	var tags []string
	if it.Important {
		tags = append(tags, "重要")
	}
	if it.ProjectID != nil {
		if name := projects[*it.ProjectID]; name != "" {
			tags = append(tags, name)
		}
	}
	if it.PlannedDate != nil {
		tags = append(tags, "计划 "+*it.PlannedDate)
	}
	if it.DueDate != nil {
		due := "截止 " + *it.DueDate
		if it.DueTime != nil {
			due += " " + *it.DueTime
		}
		tags = append(tags, due)
	}
	if it.EstimateMinutes != nil {
		tags = append(tags, "预计 "+minutesLabel(*it.EstimateMinutes))
	}
	if it.Focus.Minutes > 0 {
		tags = append(tags, fmt.Sprintf("番茄 %d · 已用 %s", it.Focus.Tomatoes, minutesLabel(it.Focus.Minutes)))
	}
	if it.Block != nil {
		tags = append(tags, "已排 "+span(it.Block.Start, it.Block.End, a.loc))
	}
	if it.Suggestion != nil {
		tags = append(tags, "待定 "+span(it.Suggestion.Start, it.Suggestion.End, a.loc))
	}
	line := fmt.Sprintf("%s %s  %s", box, short(it.ID), it.Title)
	if len(tags) > 0 {
		line += "  (" + strings.Join(tags, " · ") + ")"
	}
	return line
}

func (a *app) printItems(items []api.Item, projects map[string]string) {
	for _, it := range items {
		a.printf("%s\n", a.itemLine(it, projects))
	}
}

func (a *app) eventLine(e api.Event, projects map[string]string) string {
	when := "全天      "
	if e.AllDay {
		if e.StartDate != nil && e.EndDate != nil && *e.StartDate != *e.EndDate {
			when = "全天 至 " + (*e.EndDate)[5:]
		}
	} else if e.Start != nil && e.End != nil {
		s, en := localTime(*e.Start, a.loc), localTime(*e.End, a.loc)
		when = s.Format("15:04") + "-" + en.Format("15:04")
	}
	var tags []string
	switch e.Status {
	case "tentative":
		tags = append(tags, "待定")
	case "leaving":
		tags = append(tags, "待调整")
	}
	if e.ItemID != nil {
		tags = append(tags, "时间块")
	}
	if e.Recurring {
		tags = append(tags, "重复")
	}
	if e.ProjectID != nil {
		if name := projects[*e.ProjectID]; name != "" {
			tags = append(tags, name)
		}
	}
	if e.Location != "" {
		tags = append(tags, "@"+e.Location)
	}
	line := fmt.Sprintf("%s  %s  %s", when, short(e.ID), e.Title)
	if len(tags) > 0 {
		line += "  (" + strings.Join(tags, " · ") + ")"
	}
	return line
}

// eventDay is the local date an event starts on.
func (a *app) eventDay(e api.Event) string {
	if e.AllDay && e.StartDate != nil {
		return *e.StartDate
	}
	if e.Start != nil {
		return localTime(*e.Start, a.loc).Format(dateLayout)
	}
	return ""
}

func (a *app) suggestionLine(s api.Suggestion) string {
	what := map[string]string{
		"schedule_item": "安排时间", "create_item": "新建事项", "create_event": "新建日程",
		"move_event": "移动日程", "delete_event": "删除日程", "delete_item": "删除事项",
	}[s.Kind]
	line := fmt.Sprintf("%s  %s「%s」", short(s.ID), what, s.Title)
	if s.Start != nil && s.End != nil {
		line += "  " + span(*s.Start, *s.End, a.loc)
	}
	if s.Status != "pending" {
		line += "  [" + s.Status + "]"
	}
	return fmt.Sprintf("%s\n          %s · %s", line, s.Actor.Name, s.Reason)
}

func (a *app) activityLine(act api.Activity) string {
	line := fmt.Sprintf("%s  %s  %s · %s", short(act.ID), localTime(act.CreatedAt, a.loc).Format("01-02 15:04"),
		act.Actor.Name, act.Summary)
	if act.Undone {
		line += "  [已撤销]"
	}
	if act.Reason != nil && *act.Reason != "" {
		line += "\n          原因：" + *act.Reason
	}
	return line
}
