package dav

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

// Properties a client may set because they only concern the client itself. They are accepted and
// not kept: Apple's Calendar sets the order of the calendars in its own sidebar right after an
// account is added, and treats a refusal as an account error.
var clientOnlyProps = map[xml.Name]bool{
	{Space: "http://apple.com/ns/ical/", Local: "calendar-order"}: true,
}

// proppatch answers a property update. Names and colours belong to the project and are changed in
// the web app, so those are refused; per RFC 4918 the update is all or nothing, and the properties
// that would have been fine report 424 when another one is refused.
func (h *Handler) proppatch(w http.ResponseWriter, r *http.Request) {
	names, err := patchedProps(io.LimitReader(r.Body, 64<<10))
	if err != nil || len(names) == 0 {
		http.Error(w, "malformed property update", http.StatusBadRequest)
		return
	}
	allAllowed := true
	for _, n := range names {
		if !clientOnlyProps[n] {
			allAllowed = false
		}
	}
	var ok, refused bytes.Buffer
	for _, n := range names {
		el := fmt.Sprintf(`<x:%s xmlns:x="%s"/>`, n.Local, xmlEscape(n.Space))
		if clientOnlyProps[n] {
			ok.WriteString(el)
		} else {
			refused.WriteString(el)
		}
	}
	var body bytes.Buffer
	body.WriteString(xml.Header + `<d:multistatus xmlns:d="DAV:"><d:response><d:href>` + xmlEscape(r.URL.Path) + `</d:href>`)
	if ok.Len() > 0 {
		status := "HTTP/1.1 200 OK"
		if !allAllowed {
			status = "HTTP/1.1 424 Failed Dependency"
		}
		body.WriteString(`<d:propstat><d:prop>` + ok.String() + `</d:prop><d:status>` + status + `</d:status></d:propstat>`)
	}
	if refused.Len() > 0 {
		body.WriteString(`<d:propstat><d:prop>` + refused.String() + `</d:prop><d:status>HTTP/1.1 403 Forbidden</d:status></d:propstat>`)
	}
	body.WriteString(`</d:response></d:multistatus>`)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	w.Write(body.Bytes())
}

// patchedProps lists the properties a PROPPATCH body sets or removes.
func patchedProps(body io.Reader) ([]xml.Name, error) {
	dec := xml.NewDecoder(body)
	var names []xml.Name
	var path []xml.Name
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return names, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			// propertyupdate > set|remove > prop > the property itself.
			if len(path) == 3 && path[2].Space == "DAV:" && path[2].Local == "prop" {
				names = append(names, t.Name)
			}
			path = append(path, t.Name)
		case xml.EndElement:
			if len(path) > 0 {
				path = path[:len(path)-1]
			}
		}
	}
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
