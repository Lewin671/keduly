// Package dav exposes projects as CalDAV calendars through go-webdav.
package dav

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"

	"github.com/Lewin671/keduly/internal/core"
	"github.com/Lewin671/keduly/internal/store"
)

const (
	root            = "/dav/"
	maxResourceSize = 1 << 20
)

func principalPath(userID string) string { return root + "principals/" + userID + "/" }
func homePath(userID string) string      { return root + "calendars/" + userID + "/" }

func calendarPath(userID, key string) string { return homePath(userID) + key + "/" }

func objectPath(userID, key, name string) string { return calendarPath(userID, key) + name }

// target is a parsed request path below /dav/.
type target struct {
	kind   string // "root", "principal", "home", "calendar", "object" or "" when unknown
	userID string
	key    string // calendar key: "inbox" or a project ID
	name   string // object resource name
}

func parsePath(p string) target {
	if !strings.HasPrefix(p+"/", root) {
		return target{}
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(p, "/dav"), "/"), "/")
	if len(parts) == 1 && parts[0] == "" {
		return target{kind: "root"}
	}
	switch {
	case len(parts) == 2 && parts[0] == "principals":
		return target{kind: "principal", userID: parts[1]}
	case len(parts) == 2 && parts[0] == "calendars":
		return target{kind: "home", userID: parts[1]}
	case len(parts) == 3 && parts[0] == "calendars":
		return target{kind: "calendar", userID: parts[1], key: parts[2]}
	case len(parts) == 4 && parts[0] == "calendars":
		return target{kind: "object", userID: parts[1], key: parts[2], name: parts[3]}
	}
	return target{}
}

func (t target) collection() bool { return t.kind != "object" && t.kind != "" }

// backend serves one request for one authenticated user.
type backend struct {
	svc *core.Service
	id  *core.Identity
	req *http.Request
}

var errNotFound = webdav.NewHTTPError(http.StatusNotFound, errors.New("not found"))

func httpError(err error) error {
	var apiErr *core.Error
	if errors.As(err, &apiErr) {
		return webdav.NewHTTPError(apiErr.Status, errors.New(apiErr.Message))
	}
	return err
}

func (b *backend) userID() string { return b.id.User.ID }

func (b *backend) CurrentUserPrincipal(ctx context.Context) (string, error) {
	return principalPath(b.userID()), nil
}

func (b *backend) CalendarHomeSetPath(ctx context.Context) (string, error) {
	return homePath(b.userID()), nil
}

func (b *backend) CreateCalendar(ctx context.Context, calendar *caldav.Calendar) error {
	return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendars are projects; create them in the web app"))
}

func (b *backend) calendar(c core.Calendar) caldav.Calendar {
	return caldav.Calendar{Path: calendarPath(b.userID(), c.Key), Name: c.Name,
		MaxResourceSize: maxResourceSize, SupportedComponentSet: []string{ical.CompEvent}}
}

func (b *backend) ListCalendars(ctx context.Context) ([]caldav.Calendar, error) {
	calendars, err := b.svc.Read(ctx, b.id).Calendars()
	if err != nil {
		return nil, err
	}
	out := make([]caldav.Calendar, 0, len(calendars))
	for _, c := range calendars {
		out = append(out, b.calendar(c))
	}
	return out, nil
}

// own parses a path and rejects anything outside the caller's own tree.
func (b *backend) own(path, kind string) (target, error) {
	t := parsePath(path)
	if t.kind != kind || t.userID != b.userID() {
		return t, errNotFound
	}
	return t, nil
}

func (b *backend) GetCalendar(ctx context.Context, path string) (*caldav.Calendar, error) {
	t, err := b.own(path, "calendar")
	if err != nil {
		return nil, err
	}
	c, err := b.svc.Read(ctx, b.id).Calendar(t.key)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, errNotFound
	}
	out := b.calendar(*c)
	return &out, nil
}

func (b *backend) object(t target, ev *store.Event) (*caldav.CalendarObject, error) {
	cal, err := core.DecodeICal(ev.ICal)
	if err != nil {
		return nil, err
	}
	return &caldav.CalendarObject{
		Path: objectPath(b.userID(), t.key, ev.DavName), ModTime: store.ParseTime(ev.UpdatedAt),
		ContentLength: int64(len(ev.ICal)), ETag: ev.ETag, Data: cal,
	}, nil
}

func (b *backend) GetCalendarObject(ctx context.Context, path string, req *caldav.CalendarCompRequest) (*caldav.CalendarObject, error) {
	t, err := b.own(path, "object")
	if err != nil {
		return nil, err
	}
	ev, err := b.svc.Read(ctx, b.id).DavEvent(t.key, t.name)
	if err != nil {
		return nil, err
	}
	if ev == nil {
		return nil, errNotFound
	}
	return b.object(t, ev)
}

func (b *backend) ListCalendarObjects(ctx context.Context, path string, req *caldav.CalendarCompRequest) ([]caldav.CalendarObject, error) {
	return b.objects(ctx, path, time.Time{}, time.Time{})
}

// QueryCalendarObjects honours the VEVENT time-range of a calendar-query,
// which is what calendar clients filter on; other filters are not narrowed.
func (b *backend) QueryCalendarObjects(ctx context.Context, path string, query *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	var from, to time.Time
	for _, comp := range query.CompFilter.Comps {
		if comp.Name == ical.CompEvent {
			from, to = comp.Start, comp.End
		}
	}
	return b.objects(ctx, path, from, to)
}

func (b *backend) objects(ctx context.Context, path string, from, to time.Time) ([]caldav.CalendarObject, error) {
	t, err := b.own(path, "calendar")
	if err != nil {
		return nil, err
	}
	op := b.svc.Read(ctx, b.id)
	if c, err := op.Calendar(t.key); err != nil {
		return nil, err
	} else if c == nil {
		return nil, errNotFound
	}
	events, err := op.CalendarEvents(t.key)
	if err != nil {
		return nil, err
	}
	filtered := !from.IsZero() || !to.IsZero()
	out := make([]caldav.CalendarObject, 0, len(events))
	for _, ev := range events {
		if filtered && !op.EventOverlaps(ev, from, to) {
			continue
		}
		obj, err := b.object(t, ev)
		if err != nil {
			return nil, err
		}
		out = append(out, *obj)
	}
	return out, nil
}

func (b *backend) writable() error {
	if b.id.Token != nil && b.id.Token.Scope != "write" {
		return webdav.NewHTTPError(http.StatusForbidden, errors.New("this app password is read-only"))
	}
	return nil
}

var errPrecondition = webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("precondition failed"))

// checkMatch applies If-Match and If-None-Match to the current object.
func checkMatch(ifMatch, ifNoneMatch webdav.ConditionalMatch, current *store.Event) error {
	if ifNoneMatch.IsSet() && current != nil {
		if ok, _ := ifNoneMatch.MatchETag(current.ETag); ok {
			return errPrecondition
		}
	}
	if ifMatch.IsSet() {
		if current == nil {
			return errPrecondition
		}
		if ok, _ := ifMatch.MatchETag(current.ETag); !ok {
			return errPrecondition
		}
	}
	return nil
}

func (b *backend) PutCalendarObject(ctx context.Context, path string, cal *ical.Calendar, opts *caldav.PutCalendarObjectOptions) (*caldav.CalendarObject, error) {
	t, err := b.own(path, "object")
	if err != nil {
		return nil, err
	}
	if err := b.writable(); err != nil {
		return nil, err
	}
	kind, _, err := caldav.ValidateCalendarObject(cal)
	if err != nil {
		return nil, caldav.NewPreconditionError(caldav.PreconditionValidCalendarObjectResource)
	}
	if kind != ical.CompEvent {
		return nil, caldav.NewPreconditionError(caldav.PreconditionSupportedCalendarComponent)
	}
	var saved *store.Event
	err = b.svc.Write(ctx, b.id, core.WriteOptions{}, func(op *core.Op) error {
		current, err := op.DavEvent(t.key, t.name)
		if err != nil {
			return err
		}
		if err := checkMatch(opts.IfMatch, opts.IfNoneMatch, current); err != nil {
			return err
		}
		saved, err = op.PutDavEvent(t.key, t.name, cal)
		return err
	})
	if err != nil {
		return nil, httpError(err)
	}
	return b.object(t, saved)
}

func (b *backend) DeleteCalendarObject(ctx context.Context, path string) error {
	t, err := b.own(path, "object")
	if err != nil {
		if parsePath(path).collection() {
			return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendars are projects; delete them in the web app"))
		}
		return err
	}
	if err := b.writable(); err != nil {
		return err
	}
	ifMatch := webdav.ConditionalMatch(b.req.Header.Get("If-Match"))
	return httpError(b.svc.Write(ctx, b.id, core.WriteOptions{}, func(op *core.Op) error {
		current, err := op.DavEvent(t.key, t.name)
		if err != nil {
			return err
		}
		if current == nil {
			return errNotFound
		}
		if err := checkMatch(ifMatch, "", current); err != nil {
			return err
		}
		return op.DeleteDavEvent(current)
	}))
}
