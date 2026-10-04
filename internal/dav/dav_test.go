package dav_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/testutil"
)

type M = map[string]any

type fixture struct {
	t       *testing.T
	s       *testutil.Server
	me      *testutil.Client
	userID  string
	secret  string
	dav     *caldav.Client
	project api.Project
}

func setup(t *testing.T) *fixture {
	t.Helper()
	s := testutil.New(t, testutil.Options{})
	me := s.Register("me@example.com")
	f := &fixture{t: t, s: s, me: me}
	var who struct {
		User api.User `json:"user"`
	}
	me.Call("GET", "/me", nil, http.StatusOK, &who)
	f.userID = who.User.ID
	var project struct {
		Project api.Project `json:"project"`
	}
	me.Call("POST", "/projects", M{"name": "Keduly 开发", "color": "teal"}, http.StatusCreated, &project)
	f.project = project.Project
	f.secret = me.NewToken("iPhone 日历", "caldav", "write", true).Token
	f.dav = f.client("me@example.com", f.secret)
	return f
}

func (f *fixture) client(email, password string) *caldav.Client {
	f.t.Helper()
	c, err := caldav.NewClient(webdav.HTTPClientWithBasicAuth(http.DefaultClient, email, password), f.s.URL+"/dav/")
	if err != nil {
		f.t.Fatal(err)
	}
	return c
}

// raw sends a WebDAV request with the app password and returns status, headers and body.
func (f *fixture) raw(method, path, body string, headers map[string]string) (int, http.Header, string) {
	f.t.Helper()
	req, err := http.NewRequest(method, f.s.URL+path, strings.NewReader(body))
	if err != nil {
		f.t.Fatal(err)
	}
	req.SetBasicAuth("me@example.com", f.secret)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(data)
}

func (f *fixture) events(from, to string) []api.Event {
	f.t.Helper()
	var resp struct {
		Events []api.Event `json:"events"`
	}
	f.me.Call("GET", "/events?from="+from+"&to="+to, nil, http.StatusOK, &resp)
	return resp.Events
}

func (f *fixture) ctag(calendar string) string {
	f.t.Helper()
	body := `<?xml version="1.0"?><d:propfind xmlns:d="DAV:" xmlns:cs="http://calendarserver.org/ns/"><d:prop><cs:getctag/></d:prop></d:propfind>`
	status, _, out := f.raw("PROPFIND", calendar, body, map[string]string{"Depth": "0", "Content-Type": "application/xml"})
	if status != http.StatusMultiStatus {
		f.t.Fatalf("PROPFIND %s: %d %s", calendar, status, out)
	}
	_, rest, ok := strings.Cut(out, "getctag")
	if !ok {
		f.t.Fatalf("no getctag in %s", out)
	}
	value, _, _ := strings.Cut(rest[strings.Index(rest, ">")+1:], "<")
	if value == "" || strings.Contains(out, "404") {
		f.t.Fatalf("empty getctag in %s", out)
	}
	return value
}

func newEvent(uid, summary string, start, end time.Time) *ical.Calendar {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//Example//Calendar Client//EN")
	ev := ical.NewEvent()
	ev.Props.SetText(ical.PropUID, uid)
	ev.Props.SetDateTime(ical.PropDateTimeStamp, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	ev.Props.SetText(ical.PropSummary, summary)
	ev.Props.SetDateTime(ical.PropDateTimeStart, start)
	ev.Props.SetDateTime(ical.PropDateTimeEnd, end)
	cal.Children = append(cal.Children, ev.Component)
	return cal
}

func summary(t *testing.T, obj *caldav.CalendarObject) string {
	t.Helper()
	for _, ev := range obj.Data.Events() {
		s, _ := ev.Props.Text(ical.PropSummary)
		return s
	}
	t.Fatal("object has no VEVENT")
	return ""
}

func TestDiscovery(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	status, header, _ := f.raw("PROPFIND", "/.well-known/caldav", "", nil)
	if status != http.StatusTemporaryRedirect || header.Get("Location") != "/dav/" {
		t.Fatalf(".well-known: %d %q", status, header.Get("Location"))
	}
	status, header, _ = f.raw("OPTIONS", "/dav/", "", nil)
	if status != http.StatusNoContent || !strings.Contains(header.Get("DAV"), "calendar-access") {
		t.Fatalf("OPTIONS: %d %q", status, header.Get("DAV"))
	}

	principal, err := f.dav.FindCurrentUserPrincipal(ctx)
	if err != nil || principal != "/dav/principals/"+f.userID+"/" {
		t.Fatalf("principal %q, %v", principal, err)
	}
	home, err := f.dav.FindCalendarHomeSet(ctx, principal)
	if err != nil || home != "/dav/calendars/"+f.userID+"/" {
		t.Fatalf("home set %q, %v", home, err)
	}
	calendars, err := f.dav.FindCalendars(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(calendars) != 2 || calendars[0].Path != home+"inbox/" || calendars[1].Path != home+f.project.ID+"/" ||
		calendars[1].Name != "Keduly 开发" || len(calendars[1].SupportedComponentSet) != 1 || calendars[1].SupportedComponentSet[0] != "VEVENT" {
		t.Fatalf("calendars %+v", calendars)
	}

	// What an iPhone asks for when it sets the account up.
	body := `<?xml version="1.0"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:cs="http://calendarserver.org/ns/" xmlns:a="http://apple.com/ns/ical/">
		<d:prop><d:resourcetype/><d:displayname/><cs:getctag/><a:calendar-color/><c:supported-calendar-component-set/><d:current-user-privilege-set/><d:sync-token/></d:prop></d:propfind>`
	status, _, out := f.raw("PROPFIND", home, body, map[string]string{"Depth": "1", "Content-Type": "application/xml"})
	if status != http.StatusMultiStatus {
		t.Fatalf("PROPFIND home: %d %s", status, out)
	}
	for _, want := range []string{"Keduly 开发", "#30B0C7FF", "getctag", "VEVENT", home + "inbox/", home + f.project.ID + "/"} {
		if !strings.Contains(out, want) {
			t.Fatalf("PROPFIND home lacks %q:\n%s", want, out)
		}
	}
	body = `<?xml version="1.0"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><d:displayname/><c:calendar-home-set/><c:calendar-user-address-set/><d:current-user-principal/></d:prop></d:propfind>`
	status, _, out = f.raw("PROPFIND", principal, body, map[string]string{"Depth": "0", "Content-Type": "application/xml"})
	for _, want := range []string{"mailto:me@example.com", home, "Tester"} {
		if status != http.StatusMultiStatus || !strings.Contains(out, want) {
			t.Fatalf("PROPFIND principal lacks %q: %d\n%s", want, status, out)
		}
	}
	if strings.Contains(out, "404") {
		t.Fatalf("PROPFIND principal reports a missing property:\n%s", out)
	}
	status, _, out = f.raw("PROPFIND", "/dav/", "", map[string]string{"Depth": "0"})
	if status != http.StatusMultiStatus || !strings.Contains(out, ">/dav/<") || !strings.Contains(out, principal) {
		t.Fatalf("PROPFIND root: %d\n%s", status, out)
	}
}

func TestAuthentication(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	req, _ := http.NewRequest("PROPFIND", f.s.URL+"/dav/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || !strings.HasPrefix(resp.Header.Get("WWW-Authenticate"), "Basic") {
		t.Fatalf("anonymous: %d %q", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}
	if _, err := f.client("me@example.com", "kdl_wrong").FindCurrentUserPrincipal(ctx); err == nil {
		t.Fatal("a wrong app password was accepted")
	}
	if _, err := f.client("me@example.com", "correct horse").FindCurrentUserPrincipal(ctx); err == nil {
		t.Fatal("the account password must not work for CalDAV")
	}
	if _, err := f.client("other@example.com", f.secret).FindCurrentUserPrincipal(ctx); err == nil {
		t.Fatal("an app password was accepted with someone else's email")
	}
	agent := f.me.NewToken("Agent", "agent", "write", true).Token
	if _, err := f.client("me@example.com", agent).FindCurrentUserPrincipal(ctx); err == nil {
		t.Fatal("an agent token was accepted as an app password")
	}

	// Another user's tree does not exist for this user.
	other := f.s.Register("other@example.com")
	var who struct {
		User api.User `json:"user"`
	}
	other.Call("GET", "/me", nil, http.StatusOK, &who)
	for _, path := range []string{"/dav/calendars/" + who.User.ID + "/", "/dav/calendars/" + who.User.ID + "/inbox/", "/dav/principals/" + who.User.ID + "/"} {
		if status, _, _ := f.raw("PROPFIND", path, "", map[string]string{"Depth": "0"}); status != http.StatusNotFound {
			t.Fatalf("PROPFIND %s: %d, want 404", path, status)
		}
	}
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//x//x//EN\r\nBEGIN:VEVENT\r\nUID:x\r\nDTSTAMP:20261001T000000Z\r\nDTSTART:20261013T020000Z\r\nSUMMARY:x\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	if status, _, _ := f.raw("PUT", "/dav/calendars/"+who.User.ID+"/inbox/x.ics", ics, map[string]string{"Content-Type": "text/calendar"}); status != http.StatusNotFound {
		t.Fatalf("PUT into another user's calendar: %d", status)
	}
	if status, _, _ := f.raw("MKCOL", "/dav/calendars/"+f.userID+"/new/", "", nil); status != http.StatusForbidden {
		t.Fatalf("MKCOL: %d", status)
	}
	if status, _, _ := f.raw("DELETE", "/dav/calendars/"+f.userID+"/"+f.project.ID+"/", "", nil); status != http.StatusForbidden {
		t.Fatalf("DELETE calendar: %d", status)
	}

	// A read-only app password cannot write.
	f.secret = f.me.NewToken("Viewer", "caldav", "read", true).Token
	if status, _, _ := f.raw("PUT", "/dav/calendars/"+f.userID+"/inbox/x.ics", ics, map[string]string{"Content-Type": "text/calendar"}); status != http.StatusForbidden {
		t.Fatalf("PUT with a read-only app password: %d", status)
	}
	if status, _, _ := f.raw("PROPFIND", "/dav/calendars/"+f.userID+"/", "", map[string]string{"Depth": "1"}); status != http.StatusMultiStatus {
		t.Fatalf("PROPFIND with a read-only app password: %d", status)
	}
}

func TestEventRoundTrip(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	calendar := "/dav/calendars/" + f.userID + "/" + f.project.ID + "/"
	path := calendar + "6F1C2B0A-meeting.ics"
	ctag := f.ctag(calendar)
	inboxCTag := f.ctag("/dav/calendars/" + f.userID + "/inbox/")

	shanghai, _ := time.LoadLocation("Asia/Shanghai")
	cal := newEvent("6F1C2B0A-meeting", "朋友聚餐", time.Date(2026, 10, 13, 12, 0, 0, 0, shanghai), time.Date(2026, 10, 13, 13, 30, 0, 0, shanghai))
	event := cal.Events()[0]
	event.Props.SetText(ical.PropLocation, "外滩")
	custom := ical.NewProp("X-APPLE-TRAVEL-ADVISORY-BEHAVIOR")
	custom.Value = "AUTOMATIC"
	event.Props.Set(custom)
	alarm := ical.NewComponent(ical.CompAlarm)
	alarm.Props.SetText(ical.PropAction, "DISPLAY")
	alarm.Props.SetText(ical.PropDescription, "Reminder")
	trigger := ical.NewProp(ical.PropTrigger)
	trigger.Value = "-PT15M"
	alarm.Props.Set(trigger)
	event.Children = append(event.Children, alarm)

	put, err := f.dav.PutCalendarObject(ctx, path, cal)
	if err != nil {
		t.Fatal(err)
	}
	if put.ETag == "" {
		t.Fatal("PUT returned no ETag")
	}
	if got := f.ctag(calendar); got == ctag {
		t.Fatal("the CTag did not change after a PUT")
	}
	if got := f.ctag("/dav/calendars/" + f.userID + "/inbox/"); got != inboxCTag {
		t.Fatal("the CTag of an untouched calendar changed")
	}

	// The JSON API sees the same event.
	events := f.events("2026-10-12T16:00:00Z", "2026-10-13T16:00:00Z")
	if len(events) != 1 {
		t.Fatalf("events %+v", events)
	}
	e := events[0]
	if e.Title != "朋友聚餐" || e.Location != "外滩" || *e.Start != "2026-10-13T04:00:00Z" || *e.End != "2026-10-13T05:30:00Z" ||
		*e.ProjectID != f.project.ID || e.CreatedBy.Kind != "caldav" || e.CreatedBy.Name != "iPhone 日历" || len(e.ID) != 16 {
		t.Fatalf("event %+v", e)
	}
	var log struct {
		Activities []api.Activity `json:"activities"`
	}
	f.me.Call("GET", "/activity", nil, http.StatusOK, &log)
	if a := log.Activities[0]; a.Action != "event.create" || a.Summary != "新建日程「朋友聚餐」" || a.Actor.Kind != "caldav" || a.Actor.Name != "iPhone 日历" {
		t.Fatalf("activity %+v", a)
	}

	// The client reads back what it wrote, unknown properties included.
	got, err := f.dav.GetCalendarObject(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	back := got.Data.Events()[0]
	if got.ETag != put.ETag || summary(t, got) != "朋友聚餐" || back.Props.Get("X-APPLE-TRAVEL-ADVISORY-BEHAVIOR") == nil ||
		len(back.Children) != 1 || back.Props.Get(ical.PropDateTimeStart).Params.Get(ical.PropTimezoneID) != "Asia/Shanghai" {
		t.Fatalf("round trip lost data: etag %q vs %q, props %+v", got.ETag, put.ETag, back.Props)
	}

	// calendar-query with a time range, and multiget.
	query := func(start, end time.Time) []caldav.CalendarObject {
		t.Helper()
		objs, err := f.dav.QueryCalendar(ctx, calendar, &caldav.CalendarQuery{
			CompRequest: caldav.CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true},
			CompFilter:  caldav.CompFilter{Name: "VCALENDAR", Comps: []caldav.CompFilter{{Name: "VEVENT", Start: start, End: end}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return objs
	}
	if objs := query(time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)); len(objs) != 1 || objs[0].Path != path || objs[0].ETag != put.ETag {
		t.Fatalf("query in range %+v", objs)
	}
	if objs := query(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)); len(objs) != 0 {
		t.Fatalf("query out of range %+v", objs)
	}
	multi, err := f.dav.MultiGetCalendar(ctx, calendar, &caldav.CalendarMultiGet{Paths: []string{path},
		CompRequest: caldav.CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true}})
	if err != nil || len(multi) != 1 || summary(t, &multi[0]) != "朋友聚餐" {
		t.Fatalf("multiget %+v, %v", multi, err)
	}

	// An edit through the JSON API reaches the client with a new ETag, and
	// leaves what the server does not understand alone.
	f.me.Call("PATCH", "/events/"+e.ID, M{"title": "朋友聚餐（改期）", "start": "2026-10-14T04:00:00Z"}, http.StatusOK, nil)
	edited, err := f.dav.GetCalendarObject(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	back = edited.Data.Events()[0]
	start, _ := back.DateTimeStart(time.UTC)
	end, _ := back.DateTimeEnd(time.UTC)
	if edited.ETag == put.ETag || summary(t, edited) != "朋友聚餐（改期）" || !start.Equal(time.Date(2026, 10, 14, 4, 0, 0, 0, time.UTC)) ||
		!end.Equal(time.Date(2026, 10, 14, 5, 30, 0, 0, time.UTC)) || back.Props.Get("X-APPLE-TRAVEL-ADVISORY-BEHAVIOR") == nil || len(back.Children) != 1 ||
		back.Props.Get(ical.PropDateTimeStart).Params.Get(ical.PropTimezoneID) != "Asia/Shanghai" {
		t.Fatalf("after a JSON edit: etag %q -> %q, %+v", put.ETag, edited.ETag, back.Props)
	}

	// The client changes it; a stale If-Match is refused.
	back.Props.SetText(ical.PropSummary, "朋友聚餐（最终）")
	var buf strings.Builder
	if err := ical.NewEncoder(&buf).Encode(edited.Data); err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{"Content-Type": "text/calendar; charset=utf-8", "If-Match": `"` + put.ETag + `"`}
	if status, _, _ := f.raw("PUT", path, buf.String(), headers); status != http.StatusPreconditionFailed {
		t.Fatalf("PUT with a stale If-Match: %d", status)
	}
	headers["If-Match"] = `"` + edited.ETag + `"`
	status, header, _ := f.raw("PUT", path, buf.String(), headers)
	if status/100 != 2 || header.Get("ETag") == "" || header.Get("ETag") == `"`+edited.ETag+`"` {
		t.Fatalf("PUT with the current If-Match: %d %q", status, header.Get("ETag"))
	}
	if status, _, _ := f.raw("PUT", path, buf.String(), map[string]string{"Content-Type": "text/calendar", "If-None-Match": "*"}); status != http.StatusPreconditionFailed {
		t.Fatalf("PUT with If-None-Match over an existing object: %d", status)
	}
	var one struct {
		Event api.Event `json:"event"`
	}
	f.me.Call("GET", "/events/"+e.ID, nil, http.StatusOK, &one)
	if one.Event.Title != "朋友聚餐（最终）" {
		t.Fatalf("the client's edit did not reach the JSON API: %+v", one.Event)
	}
	f.me.Call("GET", "/activity", nil, http.StatusOK, &log)
	if a := log.Activities[0]; a.Action != "event.update" || a.Actor.Kind != "caldav" || a.Summary != "日程「朋友聚餐（改期）」改名为「朋友聚餐（最终）」" {
		t.Fatalf("activity %+v", a)
	}

	// Undoing the client's edit is visible to the client as another change.
	beforeUndo, _ := f.dav.GetCalendarObject(ctx, path)
	f.me.Call("POST", "/activity/"+log.Activities[0].ID+"/undo", nil, http.StatusOK, nil)
	afterUndo, err := f.dav.GetCalendarObject(ctx, path)
	if err != nil || afterUndo.ETag == beforeUndo.ETag || summary(t, afterUndo) != "朋友聚餐（改期）" {
		t.Fatalf("after undo: %v %+v", err, afterUndo)
	}

	// Depth 1 lists the object with its ETag.
	status, _, out := f.raw("PROPFIND", calendar, `<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:getetag/></d:prop></d:propfind>`,
		map[string]string{"Depth": "1", "Content-Type": "application/xml"})
	if status != http.StatusMultiStatus || !strings.Contains(out, path) || !strings.Contains(out, afterUndo.ETag) {
		t.Fatalf("PROPFIND calendar: %d\n%s", status, out)
	}

	// Delete.
	before := f.ctag(calendar)
	if err := f.dav.RemoveAll(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, err := f.dav.GetCalendarObject(ctx, path); err == nil {
		t.Fatal("the object is still there after DELETE")
	}
	if f.ctag(calendar) == before {
		t.Fatal("the CTag did not change after a DELETE")
	}
	f.me.Call("GET", "/events/"+e.ID, nil, http.StatusNotFound, nil)
	if status, _, _ := f.raw("DELETE", path, "", nil); status != http.StatusNotFound {
		t.Fatalf("deleting twice: %d", status)
	}
}

func TestJSONEventsAppearInCalDAV(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	home := "/dav/calendars/" + f.userID + "/"
	var created struct {
		Event api.Event `json:"event"`
	}
	f.me.Call("POST", "/events", M{"title": "设计评审", "notes": "带上原型", "start": "2026-10-13T02:00:00Z", "end": "2026-10-13T03:30:00Z", "project_id": f.project.ID},
		http.StatusCreated, &created)
	f.me.Call("POST", "/events", M{"title": "出差", "all_day": true, "start_date": "2026-10-15", "end_date": "2026-10-16"}, http.StatusCreated, nil)
	var item struct {
		Item api.Item `json:"item"`
	}
	f.me.Call("POST", "/items", M{"title": "写周报"}, http.StatusCreated, &item)
	f.me.Call("POST", "/items/"+item.Item.ID+"/schedule", M{"start": "2026-10-13T06:00:00Z", "end": "2026-10-13T07:00:00Z"}, http.StatusOK, &item)

	obj, err := f.dav.GetCalendarObject(ctx, home+f.project.ID+"/"+created.Event.ID+".ics")
	if err != nil {
		t.Fatal(err)
	}
	ev := obj.Data.Events()[0]
	start, _ := ev.DateTimeStart(time.UTC)
	notes, _ := ev.Props.Text(ical.PropDescription)
	if summary(t, obj) != "设计评审" || notes != "带上原型" || !start.Equal(time.Date(2026, 10, 13, 2, 0, 0, 0, time.UTC)) {
		t.Fatalf("generated iCalendar %+v", ev.Props)
	}

	inbox, err := f.dav.QueryCalendar(ctx, home+"inbox/", &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true},
		CompFilter:  caldav.CompFilter{Name: "VCALENDAR", Comps: []caldav.CompFilter{{Name: "VEVENT"}}},
	})
	if err != nil || len(inbox) != 2 {
		t.Fatalf("inbox %+v, %v", inbox, err)
	}
	for _, o := range inbox {
		e := o.Data.Events()[0]
		switch summary(t, &o) {
		case "出差":
			// All-day: DATE values, with an exclusive end.
			if s, en := e.Props.Get(ical.PropDateTimeStart), e.Props.Get(ical.PropDateTimeEnd); s.Value != "20261015" || en.Value != "20261017" || s.ValueType() != ical.ValueDate {
				t.Fatalf("all-day event %+v", e.Props)
			}
		case "写周报":
			// A time block is an ordinary event to calendar apps.
			if o.Path != home+"inbox/"+item.Item.Block.EventID+".ics" {
				t.Fatalf("time block path %s", o.Path)
			}
		default:
			t.Fatalf("unexpected object %s", o.Path)
		}
	}

	// A client moving the time block moves the item's plan with it.
	block, err := f.dav.GetCalendarObject(ctx, home+"inbox/"+item.Item.Block.EventID+".ics")
	if err != nil {
		t.Fatal(err)
	}
	be := block.Data.Events()[0]
	be.Props.SetDateTime(ical.PropDateTimeStart, time.Date(2026, 10, 16, 6, 0, 0, 0, time.UTC))
	be.Props.SetDateTime(ical.PropDateTimeEnd, time.Date(2026, 10, 16, 7, 0, 0, 0, time.UTC))
	if _, err := f.dav.PutCalendarObject(ctx, block.Path, block.Data); err != nil {
		t.Fatal(err)
	}
	f.me.Call("GET", "/items/"+item.Item.ID, nil, http.StatusOK, &item)
	if item.Item.Block.Start != "2026-10-16T06:00:00Z" || *item.Item.PlannedDate != "2026-10-16" {
		t.Fatalf("item after the client moved its block: %+v", item.Item)
	}

	// Moving an event to another project moves it to another calendar.
	f.me.Call("PATCH", "/events/"+created.Event.ID, M{"project_id": nil}, http.StatusOK, nil)
	if _, err := f.dav.GetCalendarObject(ctx, home+f.project.ID+"/"+created.Event.ID+".ics"); err == nil {
		t.Fatal("the event is still in its old calendar")
	}
	if _, err := f.dav.GetCalendarObject(ctx, home+"inbox/"+created.Event.ID+".ics"); err != nil {
		t.Fatal(err)
	}
}

func TestRecurringEvents(t *testing.T) {
	f := setup(t)
	path := "/dav/calendars/" + f.userID + "/inbox/weekly.ics"
	// Every Monday 09:00-09:30 Shanghai from 5 October, except the 19th;
	// the 26th was moved to 15:00 and renamed by the client.
	ics := strings.Join([]string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Example//Calendar Client//EN",
		"BEGIN:VEVENT", "UID:weekly", "DTSTAMP:20261001T000000Z",
		"DTSTART;TZID=Asia/Shanghai:20261005T090000", "DTEND;TZID=Asia/Shanghai:20261005T093000",
		"RRULE:FREQ=WEEKLY;BYDAY=MO", "EXDATE;TZID=Asia/Shanghai:20261019T090000", "SUMMARY:周会", "END:VEVENT",
		"BEGIN:VEVENT", "UID:weekly", "DTSTAMP:20261001T000000Z",
		"RECURRENCE-ID;TZID=Asia/Shanghai:20261026T090000",
		"DTSTART;TZID=Asia/Shanghai:20261026T150000", "DTEND;TZID=Asia/Shanghai:20261026T153000",
		"SUMMARY:周会（改到下午）", "END:VEVENT",
		"END:VCALENDAR", "",
	}, "\r\n")
	status, _, body := f.raw("PUT", path, ics, map[string]string{"Content-Type": "text/calendar; charset=utf-8"})
	if status/100 != 2 {
		t.Fatalf("PUT: %d %s", status, body)
	}

	events := f.events("2026-10-01T00:00:00Z", "2026-11-01T00:00:00Z")
	type want struct{ start, instance, title string }
	wants := []want{
		{"2026-10-05T01:00:00Z", "2026-10-05T01:00:00Z", "周会"},
		{"2026-10-12T01:00:00Z", "2026-10-12T01:00:00Z", "周会"},
		{"2026-10-26T07:00:00Z", "2026-10-26T01:00:00Z", "周会（改到下午）"},
	}
	if len(events) != len(wants) {
		data, _ := json.Marshal(events)
		t.Fatalf("%d instances, want %d: %s", len(events), len(wants), data)
	}
	for i, w := range wants {
		e := events[i]
		if *e.Start != w.start || e.Instance == nil || *e.Instance != w.instance || e.Title != w.title || !e.Recurring || !e.Readonly ||
			e.RRule == nil || *e.RRule != "FREQ=WEEKLY;BYDAY=MO" || e.ID != events[0].ID {
			t.Fatalf("instance %d = %+v, want %+v", i, e, w)
		}
	}
	if end := *events[0].End; end != "2026-10-05T01:30:00Z" {
		t.Fatalf("instance end %s", end)
	}

	// Views and calculations see the instances too.
	var today api.Today
	f.s.SetNow(time.Date(2026, 10, 12, 2, 0, 0, 0, time.UTC))
	f.me.Call("GET", "/today", nil, http.StatusOK, &today)
	if len(today.Events) != 1 || today.Events[0].Title != "周会" || today.FreeMinutes != 540-30 {
		t.Fatalf("today %+v", today)
	}
	var heat struct {
		Days map[string]int `json:"days"`
	}
	f.me.Call("GET", "/calendar/heat?year=2026", nil, http.StatusOK, &heat)
	if heat.Days["2026-10-05"] != 1 || heat.Days["2026-10-19"] != 0 || heat.Days["2026-10-26"] != 1 || heat.Days["2026-12-28"] != 1 {
		t.Fatalf("heat %+v", heat.Days)
	}
	var free struct {
		Slots []api.Slot `json:"slots"`
	}
	f.me.Call("GET", "/free?date=2026-10-26&duration=60", nil, http.StatusOK, &free)
	if len(free.Slots) != 2 || free.Slots[0].End != "2026-10-26T07:00:00Z" || free.Slots[1].Start != "2026-10-26T07:30:00Z" {
		t.Fatalf("free around the moved instance %+v", free.Slots)
	}

	// The web app cannot change a series; a calendar app can.
	id := events[0].ID
	for _, call := range [][2]string{{"PATCH", `{"title":"x"}`}, {"DELETE", ""}} {
		if status, code := f.me.ErrorCode(call[0], "/events/"+id, call[1]); status != 400 || code != "invalid_request" {
			t.Fatalf("%s on a recurring event: %d %s", call[0], status, code)
		}
	}
	var one struct {
		Event api.Event `json:"event"`
	}
	f.me.Call("GET", "/events/"+id, nil, http.StatusOK, &one)
	if !one.Event.Recurring || !one.Event.Readonly || one.Event.Instance != nil {
		t.Fatalf("series %+v", one.Event)
	}

	// A calendar-query for one week finds the series through its instances.
	objs, err := f.dav.QueryCalendar(context.Background(), "/dav/calendars/"+f.userID+"/inbox/", &caldav.CalendarQuery{
		CompRequest: caldav.CalendarCompRequest{Name: "VCALENDAR", AllProps: true, AllComps: true},
		CompFilter: caldav.CompFilter{Name: "VCALENDAR", Comps: []caldav.CompFilter{{Name: "VEVENT",
			Start: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)}}},
	})
	if err != nil || len(objs) != 1 || len(objs[0].Data.Events()) != 2 {
		t.Fatalf("query %+v, %v", objs, err)
	}

	// All-day yearly event: instances are dates.
	birthday := strings.Join([]string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Example//Calendar Client//EN",
		"BEGIN:VEVENT", "UID:birthday", "DTSTAMP:20261001T000000Z",
		"DTSTART;VALUE=DATE:20201020", "DTEND;VALUE=DATE:20201021", "RRULE:FREQ=YEARLY", "SUMMARY:生日", "END:VEVENT",
		"END:VCALENDAR", "",
	}, "\r\n")
	if status, _, body := f.raw("PUT", "/dav/calendars/"+f.userID+"/inbox/birthday.ics", birthday, map[string]string{"Content-Type": "text/calendar"}); status/100 != 2 {
		t.Fatalf("PUT: %d %s", status, body)
	}
	events = f.events("2026-10-19T16:00:00Z", "2026-10-20T16:00:00Z")
	if len(events) != 1 || !events[0].AllDay || *events[0].StartDate != "2026-10-20" || *events[0].EndDate != "2026-10-20" || events[0].Start != nil {
		t.Fatalf("all-day instance %+v", events)
	}
}

func TestRejectsWhatIsNotAnEvent(t *testing.T) {
	f := setup(t)
	todo := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//x//x//EN\r\nBEGIN:VTODO\r\nUID:t\r\nDTSTAMP:20261001T000000Z\r\nSUMMARY:x\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	path := "/dav/calendars/" + f.userID + "/inbox/t.ics"
	if status, _, _ := f.raw("PUT", path, todo, map[string]string{"Content-Type": "text/calendar"}); status/100 == 2 {
		t.Fatalf("a VTODO was accepted: %d", status)
	}
	if status, _, _ := f.raw("PUT", path, "not a calendar", map[string]string{"Content-Type": "text/calendar"}); status != http.StatusBadRequest {
		t.Fatalf("garbage: %d", status)
	}
	if status, _, _ := f.raw("PUT", "/dav/calendars/"+f.userID+"/nosuchproject/t.ics", todo, map[string]string{"Content-Type": "text/calendar"}); status/100 == 2 {
		t.Fatalf("PUT into a calendar that does not exist: %d", status)
	}
	if got := f.events("2026-01-01T00:00:00Z", "2026-12-31T00:00:00Z"); len(got) != 0 {
		t.Fatalf("something was stored: %+v", got)
	}
}

// A weekly meeting at 09:00 New York time must stay at 09:00 on the wall clock when New York
// leaves daylight saving time (1 November 2026), which moves it by an hour in UTC. The account's
// own zone (Shanghai, which has no DST) must not influence the expansion.
func TestRecurringEventKeepsItsWallClockTimeAcrossDST(t *testing.T) {
	f := setup(t)
	ics := strings.Join([]string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Example//Calendar Client//EN",
		"BEGIN:VEVENT", "UID:ny-weekly", "DTSTAMP:20261001T000000Z",
		"DTSTART;TZID=America/New_York:20261026T090000", "DTEND;TZID=America/New_York:20261026T100000",
		"RRULE:FREQ=WEEKLY;BYDAY=MO", "SUMMARY:New York sync", "END:VEVENT",
		"END:VCALENDAR", "",
	}, "\r\n")
	status, _, body := f.raw("PUT", "/dav/calendars/"+f.userID+"/inbox/ny-weekly.ics", ics, map[string]string{"Content-Type": "text/calendar; charset=utf-8"})
	if status/100 != 2 {
		t.Fatalf("PUT: %d %s", status, body)
	}
	events := f.events("2026-10-25T00:00:00Z", "2026-11-10T00:00:00Z")
	// 26 Oct is still EDT (UTC-4); 2 and 9 Nov are EST (UTC-5).
	wants := []string{"2026-10-26T13:00:00Z", "2026-11-02T14:00:00Z", "2026-11-09T14:00:00Z"}
	if len(events) != len(wants) {
		data, _ := json.Marshal(events)
		t.Fatalf("%d instances, want %d: %s", len(events), len(wants), data)
	}
	for i, want := range wants {
		if got := *events[i].Start; got != want {
			t.Fatalf("instance %d starts %s, want %s", i, got, want)
		}
	}
}
