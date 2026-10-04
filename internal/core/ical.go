package core

import (
	"bytes"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/emersion/go-ical"
	"github.com/teambition/rrule-go"

	"github.com/Lewin671/keduly/internal/store"
)

const (
	icalDate     = "20060102"
	icalProdID   = "-//Keduly//Keduly//EN"
	maxInstances = 2000
	maxRuleSteps = 60000
)

// DecodeICal parses stored iCalendar text.
func DecodeICal(text string) (*ical.Calendar, error) {
	return ical.NewDecoder(strings.NewReader(text)).Decode()
}

// EncodeICal serialises a calendar, filling in the properties the encoder
// insists on so that sloppy client data still round-trips.
func EncodeICal(cal *ical.Calendar, now time.Time) (string, error) {
	if cal.Props.Get(ical.PropVersion) == nil {
		cal.Props.SetText(ical.PropVersion, "2.0")
	}
	if cal.Props.Get(ical.PropProductID) == nil {
		cal.Props.SetText(ical.PropProductID, icalProdID)
	}
	for _, child := range cal.Children {
		if child.Name == ical.CompEvent && child.Props.Get(ical.PropDateTimeStamp) == nil {
			child.Props.SetDateTime(ical.PropDateTimeStamp, now.UTC())
		}
	}
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// masterEvent is the VEVENT that defines the event (not an overridden instance).
func masterEvent(cal *ical.Calendar) *ical.Component {
	var first *ical.Component
	for _, child := range cal.Children {
		if child.Name != ical.CompEvent {
			continue
		}
		if child.Props.Get(ical.PropRecurrenceID) == nil {
			return child
		}
		if first == nil {
			first = child
		}
	}
	return first
}

func isDateProp(p *ical.Prop) bool {
	return p.ValueType() == ical.ValueDate || (p.ValueType() == ical.ValueDefault && len(p.Value) == len(icalDate))
}

// propTime reads a date or date-time. Floating times, dates and unknown TZIDs
// are interpreted in loc.
func propTime(p *ical.Prop, loc *time.Location) (time.Time, error) {
	t, err := p.DateTime(loc)
	if err == nil {
		return t, nil
	}
	if p.Params.Get(ical.PropTimezoneID) != "" {
		floating := *p
		floating.Params = ical.Params{}
		return floating.DateTime(loc)
	}
	return t, err
}

// span returns the start and end of a VEVENT. For all-day events both are
// local midnights and the end is exclusive.
func span(comp *ical.Component, loc *time.Location) (start, end time.Time, allDay bool, err error) {
	sp := comp.Props.Get(ical.PropDateTimeStart)
	if sp == nil {
		return start, end, false, Invalid("the event has no DTSTART")
	}
	if start, err = propTime(sp, loc); err != nil {
		return start, end, false, Invalid("the event has an invalid DTSTART")
	}
	allDay = isDateProp(sp)
	switch {
	case comp.Props.Get(ical.PropDateTimeEnd) != nil:
		if end, err = propTime(comp.Props.Get(ical.PropDateTimeEnd), loc); err != nil {
			return start, end, allDay, Invalid("the event has an invalid DTEND")
		}
	case comp.Props.Get(ical.PropDuration) != nil:
		d, derr := comp.Props.Get(ical.PropDuration).Duration()
		if derr != nil {
			return start, end, allDay, Invalid("the event has an invalid DURATION")
		}
		end = start.Add(d)
	case allDay:
		end = start.AddDate(0, 0, 1)
	default:
		end = start
	}
	if end.Before(start) {
		end = start
	}
	if allDay && !end.After(start) {
		end = start.AddDate(0, 0, 1)
	}
	return start, end, allDay, nil
}

func clip(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

func propText(comp *ical.Component, name string) string {
	s, _ := comp.Props.Text(name)
	return s
}

// applyICal copies what the server understands from iCalendar data into the row.
func applyICal(ev *store.Event, cal *ical.Calendar, loc *time.Location) error {
	master := masterEvent(cal)
	if master == nil {
		return Invalid("the calendar object has no VEVENT")
	}
	start, end, allDay, err := span(master, loc)
	if err != nil {
		return err
	}
	ev.UID = propText(master, ical.PropUID)
	ev.Title = clip(strings.TrimSpace(propText(master, ical.PropSummary)), maxTitle)
	ev.Notes = clip(propText(master, ical.PropDescription), maxNotes)
	ev.Location = clip(propText(master, ical.PropLocation), maxTitle)
	ev.AllDay = allDay
	ev.StartAt, ev.EndAt, ev.StartDate, ev.EndDate = nil, nil, nil, nil
	if allDay {
		sd, ed := start.Format(dateLayout), end.AddDate(0, 0, -1).Format(dateLayout)
		ev.StartDate, ev.EndDate = &sd, &ed
	} else {
		sa, ea := store.FormatTime(start), store.FormatTime(end)
		ev.StartAt, ev.EndAt = &sa, &ea
	}
	ev.RRule = nil
	if p := master.Props.Get(ical.PropRecurrenceRule); p != nil && strings.TrimSpace(p.Value) != "" {
		rule := strings.TrimSpace(p.Value)
		ev.RRule = &rule
	}
	return nil
}

// eventSpan returns the row's start and exclusive end as instants.
func eventSpan(ev *store.Event, loc *time.Location) (time.Time, time.Time) {
	if ev.AllDay {
		return dayStart(*ev.StartDate, loc), dayStart(*ev.EndDate, loc).AddDate(0, 0, 1)
	}
	return store.ParseTime(*ev.StartAt), store.ParseTime(*ev.EndAt)
}

func dayStart(date string, loc *time.Location) time.Time {
	t, _ := time.ParseInLocation(dateLayout, date, loc)
	return t
}

func setDate(comp *ical.Component, name, date string) {
	p := ical.NewProp(name)
	p.SetValueType(ical.ValueDate)
	p.Value = strings.ReplaceAll(date, "-", "")
	comp.Props.Set(p)
}

// setInstant writes a date-time, keeping the property's existing time zone.
func setInstant(comp *ical.Component, name string, t time.Time) {
	zone := time.UTC
	if old := comp.Props.Get(name); old != nil {
		if tzid := old.Params.Get(ical.PropTimezoneID); tzid != "" {
			if l, err := time.LoadLocation(tzid); err == nil {
				zone = l
			}
		}
	}
	p := ical.NewProp(name)
	p.SetDateTime(t.In(zone))
	comp.Props.Set(p)
}

func setOptionalText(comp *ical.Component, name, value string) {
	if value == "" {
		comp.Props.Del(name)
		return
	}
	comp.Props.SetText(name, value)
}

// refreshICal makes the row's iCalendar text match its fields. Text that came
// from a CalDAV client is patched in place so unknown properties survive.
func refreshICal(ev *store.Event, loc *time.Location, now time.Time) error {
	var cal *ical.Calendar
	if ev.ICal != "" {
		cal, _ = DecodeICal(ev.ICal)
	}
	var master *ical.Component
	if cal != nil {
		master = masterEvent(cal)
	}
	if master == nil {
		cal = ical.NewCalendar()
		master = ical.NewComponent(ical.CompEvent)
		master.Props.SetText(ical.PropUID, ev.UID)
		cal.Children = append(cal.Children, master)
	}
	master.Props.SetText(ical.PropSummary, ev.Title)
	setOptionalText(master, ical.PropDescription, ev.Notes)
	setOptionalText(master, ical.PropLocation, ev.Location)

	oldStart, oldEnd, oldAllDay, err := span(master, loc)
	newStart, newEnd := eventSpan(ev, loc)
	if err != nil || oldAllDay != ev.AllDay || !oldStart.Equal(newStart) || !oldEnd.Equal(newEnd) {
		master.Props.Del(ical.PropDuration)
		if ev.AllDay {
			setDate(master, ical.PropDateTimeStart, *ev.StartDate)
			setDate(master, ical.PropDateTimeEnd, newEnd.Format(dateLayout))
		} else {
			if oldAllDay {
				master.Props.Del(ical.PropDateTimeStart)
				master.Props.Del(ical.PropDateTimeEnd)
			}
			setInstant(master, ical.PropDateTimeStart, newStart)
			setInstant(master, ical.PropDateTimeEnd, newEnd)
		}
	}
	seq := 0
	if p := master.Props.Get(ical.PropSequence); p != nil {
		if n, err := p.Int(); err == nil {
			seq = n + 1
		}
	}
	sp := ical.NewProp(ical.PropSequence)
	sp.Value = strconv.Itoa(seq)
	master.Props.Set(sp)
	master.Props.SetDateTime(ical.PropDateTimeStamp, now.UTC())
	master.Props.SetDateTime(ical.PropLastModified, now.UTC())
	text, err := EncodeICal(cal, now)
	if err != nil {
		return err
	}
	ev.ICal = text
	return nil
}

// occurrence is one instance of an event on the timeline.
type occurrence struct {
	Start, End time.Time // End is exclusive
	Original   time.Time
	Title      string
}

// splitTimes reads every value of a multi-valued date property such as EXDATE.
func splitTimes(props []ical.Prop, loc *time.Location) []time.Time {
	var out []time.Time
	for _, p := range props {
		for _, v := range strings.Split(p.Value, ",") {
			one := p
			one.Value = strings.TrimSpace(v)
			if t, err := propTime(&one, loc); err == nil {
				out = append(out, t)
			}
		}
	}
	return out
}

// expand lists the occurrences of a recurring event that overlap [from, to).
func expand(ev *store.Event, from, to time.Time, loc *time.Location) []occurrence {
	cal, err := DecodeICal(ev.ICal)
	if err != nil {
		return nil
	}
	master := masterEvent(cal)
	if master == nil || ev.RRule == nil {
		return nil
	}
	start, end, _, err := span(master, loc)
	if err != nil {
		return nil
	}
	length := end.Sub(start)
	overlaps := func(s, e time.Time) bool {
		return s.Before(to) && (e.After(from) || (e.Equal(s) && !s.Before(from)))
	}

	// Instances the client edited on their own replace the generated ones.
	overridden := map[int64]bool{}
	var out []occurrence
	for _, child := range cal.Children {
		rid := child.Props.Get(ical.PropRecurrenceID)
		if child.Name != ical.CompEvent || rid == nil {
			continue
		}
		original, err := propTime(rid, loc)
		if err != nil {
			continue
		}
		overridden[original.Unix()] = true
		s, e, _, err := span(child, loc)
		if err == nil && overlaps(s, e) {
			out = append(out, occurrence{Start: s, End: e, Original: original, Title: strings.TrimSpace(propText(child, ical.PropSummary))})
		}
	}

	opt, err := rrule.StrToROptionInLocation(*ev.RRule, start.Location())
	// Sub-daily rules could mean millions of steps; such an event shows once.
	if err != nil || opt.Freq > rrule.DAILY {
		if overlaps(start, end) {
			out = append(out, occurrence{Start: start, End: end, Original: start})
		}
		return out
	}
	opt.Dtstart = start
	rule, err := rrule.NewRRule(*opt)
	if err != nil {
		return out
	}
	set := rrule.Set{}
	set.RRule(rule)
	set.DTStart(start)
	for _, t := range splitTimes(master.Props[ical.PropExceptionDates], loc) {
		set.ExDate(t)
	}
	for _, t := range splitTimes(master.Props[ical.PropRecurrenceDates], loc) {
		set.RDate(t)
	}
	next := set.Iterator()
	for steps := 0; steps < maxRuleSteps && len(out) < maxInstances; steps++ {
		s, ok := next()
		if !ok || !s.Before(to) {
			break
		}
		if overridden[s.Unix()] {
			continue
		}
		e := s.Add(length)
		if ev.AllDay {
			// Keep whole days across daylight-saving changes.
			e = s.AddDate(0, 0, int(length.Hours()+12)/24)
		}
		if overlaps(s, e) {
			out = append(out, occurrence{Start: s, End: e, Original: s})
		}
	}
	return out
}
