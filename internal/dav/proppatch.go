package dav

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"strings"

	"github.com/Lewin671/keduly/internal/core"
	"github.com/Lewin671/keduly/internal/store"
)

// Calendar apps keep a few properties of their own on each calendar: Apple's Calendar writes the
// sidebar order and the time zone right after an account is added, and treats a refusal as an
// account error. These are stored and returned as written, without being interpreted.
//
// Everything else is refused, in particular the name: a calendar is a project, and projects are
// renamed in the web app.
func clientProp(n xml.Name) bool {
	switch n.Space {
	case nsApple:
		return true
	case nsCalDAV:
		return n.Local == "calendar-timezone" || n.Local == "calendar-description" || n.Local == "schedule-calendar-transp"
	}
	return false
}

type propChange struct {
	name   xml.Name
	value  string
	remove bool
	nested bool // the value has child elements, which are not stored
}

// proppatch applies a property update to a calendar. Per RFC 4918 the update is all or nothing:
// when one property is refused, the others report 424 and nothing is written.
func (h *Handler) proppatch(w http.ResponseWriter, r *http.Request, id *core.Identity, t target) {
	changes, err := parsePropPatch(io.LimitReader(r.Body, 256<<10))
	if err != nil || len(changes) == 0 {
		http.Error(w, "malformed property update", http.StatusBadRequest)
		return
	}
	if t.kind != "calendar" || !h.ownsCalendar(r, id, t.key) {
		http.Error(w, "properties can only be set on a calendar", http.StatusForbidden)
		return
	}
	accepted := func(c propChange) bool { return clientProp(c.name) && !c.nested }
	allAccepted := true
	for _, c := range changes {
		if !accepted(c) {
			allAccepted = false
		}
	}
	if allAccepted {
		for _, c := range changes {
			var err error
			if c.remove {
				err = store.DeleteDavProp(r.Context(), h.Service.DB, id.User.ID, t.key, c.name.Space, c.name.Local)
			} else {
				err = store.SetDavProp(r.Context(), h.Service.DB, id.User.ID, t.key, store.DavProp{Space: c.name.Space, Local: c.name.Local, Value: c.value})
			}
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}
	}

	resp := el(nsDAV, "response", href(r.URL.Path))
	group := func(status string, keep func(propChange) bool) {
		prop := el(nsDAV, "prop")
		for _, c := range changes {
			if keep(c) {
				prop.Children = append(prop.Children, &node{XMLName: c.name})
			}
		}
		if len(prop.Children) > 0 {
			resp.Children = append(resp.Children, el(nsDAV, "propstat", prop, text(nsDAV, "status", status)))
		}
	}
	if allAccepted {
		group("HTTP/1.1 200 OK", accepted)
	} else {
		group("HTTP/1.1 424 Failed Dependency", accepted)
		group("HTTP/1.1 403 Forbidden", func(c propChange) bool { return !accepted(c) })
	}
	var out bytes.Buffer
	out.WriteString(xml.Header)
	if err := xml.NewEncoder(&out).Encode(el(nsDAV, "multistatus", resp)); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	w.Write(out.Bytes())
}

func (h *Handler) ownsCalendar(r *http.Request, id *core.Identity, key string) bool {
	list, err := h.Service.Read(r.Context(), id).Calendars()
	if err != nil {
		return false
	}
	for _, c := range list {
		if c.Key == key {
			return true
		}
	}
	return false
}

// parsePropPatch lists the properties a PROPPATCH body sets or removes, in order.
func parsePropPatch(body io.Reader) ([]propChange, error) {
	var doc node
	if err := xml.NewDecoder(body).Decode(&doc); err != nil {
		return nil, err
	}
	if doc.XMLName.Space != nsDAV || doc.XMLName.Local != "propertyupdate" {
		return nil, io.ErrUnexpectedEOF
	}
	var changes []propChange
	for _, action := range doc.Children {
		if action.XMLName.Space != nsDAV || (action.XMLName.Local != "set" && action.XMLName.Local != "remove") {
			continue
		}
		prop := action.child("prop")
		if prop == nil {
			continue
		}
		for _, p := range prop.Children {
			changes = append(changes, propChange{name: p.XMLName, value: strings.TrimSpace(p.Text),
				remove: action.XMLName.Local == "remove", nested: len(p.Children) > 0})
		}
	}
	return changes, nil
}
