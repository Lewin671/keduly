package dav

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Lewin671/keduly/internal/core"
)

// go-webdav answers the standard properties but not the ones calendar apps
// lean on for change detection and display (getctag, calendar-color, ...).
// Its multistatus is therefore parsed into a generic tree, the properties it
// reported as missing are filled in, and the tree is written out again.

const (
	nsDAV    = "DAV:"
	nsCalDAV = "urn:ietf:params:xml:ns:caldav"
	nsCS     = "http://calendarserver.org/ns/"
	nsApple  = "http://apple.com/ns/ical/"
)

type node struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Text     string     `xml:",chardata"`
	Children []*node    `xml:",any"`
}

func el(space, local string, children ...*node) *node {
	return &node{XMLName: xml.Name{Space: space, Local: local}, Children: children}
}

func text(space, local, value string) *node {
	return &node{XMLName: xml.Name{Space: space, Local: local}, Text: value}
}

func (n *node) child(local string) *node {
	for _, c := range n.Children {
		if c.XMLName.Space == nsDAV && c.XMLName.Local == local {
			return c
		}
	}
	return nil
}

// clean drops namespace declarations, which the encoder writes itself, and
// the whitespace between elements.
func (n *node) clean() {
	attrs := n.Attrs[:0]
	for _, a := range n.Attrs {
		if a.Name.Local != "xmlns" && a.Name.Space != "xmlns" {
			attrs = append(attrs, a)
		}
	}
	n.Attrs = attrs
	if len(n.Children) > 0 {
		n.Text = ""
	}
	for _, c := range n.Children {
		c.clean()
	}
}

var colorHex = map[string]string{
	"blue": "#2F80EDFF", "indigo": "#5856D6FF", "orange": "#F2994AFF", "teal": "#30B0C7FF",
	"green": "#27AE60FF", "pink": "#EB5C9DFF", "purple": "#9B51E0FF", "brown": "#A2845EFF",
}

func href(path string) *node { return text(nsDAV, "href", path) }

func reportSet() *node {
	set := el(nsDAV, "supported-report-set")
	for _, name := range []string{"calendar-query", "calendar-multiget"} {
		set.Children = append(set.Children, el(nsDAV, "supported-report", el(nsDAV, "report", el(nsCalDAV, name))))
	}
	return set
}

// extraProps returns the additional properties of the resource at path.
func extraProps(id *core.Identity, calendars map[string]core.Calendar, path string) map[xml.Name]*node {
	t := parsePath(path)
	uid := id.User.ID
	props := map[xml.Name]*node{}
	add := func(n *node) { props[n.XMLName] = n }
	switch t.kind {
	case "principal":
		add(text(nsDAV, "displayname", id.User.Name))
		add(el(nsDAV, "principal-URL", href(principalPath(uid))))
		add(el(nsCalDAV, "calendar-user-address-set", href("mailto:"+id.User.Email), href(principalPath(uid))))
		add(reportSet())
	case "home":
		add(el(nsDAV, "owner", href(principalPath(uid))))
		add(reportSet())
	case "calendar":
		c, ok := calendars[t.key]
		if !ok {
			break
		}
		add(text(nsCS, "getctag", strconv.FormatInt(c.CTag, 10)))
		add(el(nsDAV, "owner", href(principalPath(uid))))
		add(reportSet())
		if hex := colorHex[c.Color]; hex != "" {
			add(text(nsApple, "calendar-color", hex))
		}
	}
	return props
}

// addProps fills in the properties go-webdav reported as not found.
func (h *Handler) addProps(r *http.Request, id *core.Identity, t target, body []byte) ([]byte, error) {
	var ms node
	if err := xml.Unmarshal(body, &ms); err != nil {
		return nil, err
	}
	ms.clean()
	list, err := h.Service.Read(r.Context(), id).Calendars()
	if err != nil {
		return nil, err
	}
	calendars := map[string]core.Calendar{}
	for _, c := range list {
		calendars[c.Key] = c
	}
	for _, resp := range ms.Children {
		hrefNode := resp.child("href")
		if resp.XMLName.Local != "response" || hrefNode == nil {
			continue
		}
		// go-webdav labels the root with the principal's address.
		if t.kind == "root" {
			hrefNode.Text = root
			continue
		}
		path := strings.TrimSpace(hrefNode.Text)
		if u, err := url.Parse(path); err == nil {
			path = u.Path
		}
		fill(resp, extraProps(id, calendars, path))
	}
	var out bytes.Buffer
	out.WriteString(xml.Header)
	if err := xml.NewEncoder(&out).Encode(&ms); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// fill moves properties we can answer from the 404 propstat to the 200 one.
func fill(resp *node, extra map[xml.Name]*node) {
	if len(extra) == 0 {
		return
	}
	var found []*node
	kept := resp.Children[:0]
	for _, ps := range resp.Children {
		status, prop := ps.child("status"), ps.child("prop")
		if ps.XMLName.Local != "propstat" || status == nil || prop == nil || !strings.Contains(status.Text, " 404 ") {
			kept = append(kept, ps)
			continue
		}
		missing := prop.Children[:0]
		for _, p := range prop.Children {
			if value, ok := extra[p.XMLName]; ok {
				found = append(found, value)
			} else {
				missing = append(missing, p)
			}
		}
		prop.Children = missing
		if len(missing) > 0 {
			kept = append(kept, ps)
		}
	}
	resp.Children = kept
	if len(found) == 0 {
		return
	}
	for _, ps := range resp.Children {
		status, prop := ps.child("status"), ps.child("prop")
		if ps.XMLName.Local == "propstat" && status != nil && prop != nil && strings.Contains(status.Text, " 200 ") {
			prop.Children = append(prop.Children, found...)
			return
		}
	}
	resp.Children = append(resp.Children, el(nsDAV, "propstat",
		el(nsDAV, "prop", found...), text(nsDAV, "status", "HTTP/1.1 200 OK")))
}
