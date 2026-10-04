package dav

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/emersion/go-webdav/caldav"

	"github.com/Lewin671/keduly/internal/core"
)

// Handler serves /dav/ and /.well-known/caldav.
type Handler struct {
	Service *core.Service
}

func New(svc *core.Service) *Handler { return &Handler{Service: svc} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/.well-known/caldav" {
		// 307 keeps the method: clients arrive here with PROPFIND.
		http.Redirect(w, r, root, http.StatusTemporaryRedirect)
		return
	}
	// Clients probe with OPTIONS before they have credentials.
	if r.Method == http.MethodOptions {
		w.Header().Set("DAV", "1, 3, calendar-access")
		w.Header().Set("Allow", "OPTIONS, GET, HEAD, PUT, DELETE, PROPFIND, REPORT")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	email, password, ok := r.BasicAuth()
	if !ok {
		unauthorized(w)
		return
	}
	id, err := h.Service.Basic(r.Context(), email, password)
	if err != nil {
		unauthorized(w)
		return
	}
	t := parsePath(r.URL.Path)
	if t.kind == "" || (t.kind != "root" && t.userID != id.User.ID) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case "MKCOL", "MKCALENDAR", "PROPPATCH", "COPY", "MOVE":
		http.Error(w, "calendars are projects; manage them in the web app", http.StatusForbidden)
		return
	case http.MethodPut, http.MethodGet, http.MethodHead:
		if t.kind != "object" {
			http.Error(w, "not a calendar object", http.StatusMethodNotAllowed)
			return
		}
	}
	// go-webdav compares collection paths exactly, with the trailing slash.
	if t.collection() && !strings.HasSuffix(r.URL.Path, "/") {
		r = r.Clone(r.Context())
		r.URL.Path += "/"
	}
	// go-webdav tells resource kinds apart by depth below the prefix alone;
	// principals sit one level deeper than it expects, hence their own prefix.
	prefix := "/dav"
	if t.kind == "principal" {
		prefix = "/dav/principals"
	}
	inner := &caldav.Handler{Backend: &backend{svc: h.Service, id: id, req: r}, Prefix: prefix}
	if r.Method != "PROPFIND" {
		inner.ServeHTTP(w, r)
		return
	}
	rec := &recorder{header: http.Header{}}
	inner.ServeHTTP(rec, r)
	body := rec.body.Bytes()
	if rec.status == http.StatusMultiStatus {
		if patched, err := h.addProps(r, id, t, body); err == nil {
			body = patched
		}
	}
	for k, v := range rec.header {
		w.Header()[k] = v
	}
	w.Header().Del("Content-Length")
	w.WriteHeader(rec.status)
	w.Write(body)
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Keduly", charset="UTF-8"`)
	http.Error(w, "sign in with your email and an app password", http.StatusUnauthorized)
}

// recorder buffers a response so that it can be amended before it is sent.
type recorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}
