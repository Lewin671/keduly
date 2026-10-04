package dav

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/emersion/go-webdav/caldav"

	"github.com/Lewin671/keduly/internal/core"
)

// Handler serves /dav/ and /.well-known/caldav.
type Handler struct {
	Service *core.Service
	trace   bool
}

func New(svc *core.Service) *Handler {
	return &Handler{Service: svc, trace: os.Getenv("KEDULY_DAV_TRACE") == "1"}
}

// ServeHTTP optionally traces the exchange. Calendar clients differ in what they send, and their
// own error messages say nothing; KEDULY_DAV_TRACE=1 logs each request and response body
// (truncated) so a failing client can be diagnosed. It logs calendar content: leave it off normally.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.trace {
		h.serve(w, r)
		return
	}
	in, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	r.Body = io.NopCloser(bytes.NewReader(in))
	rec := &recorder{header: http.Header{}}
	h.serve(rec, r)
	slog.Info("dav trace", "method", r.Method, "path", r.URL.Path, "depth", r.Header.Get("Depth"), "agent", r.UserAgent(),
		"request", clip(in), "status", rec.status, "response", clip(rec.body.Bytes()))
	for k, v := range rec.header {
		w.Header()[k] = v
	}
	if rec.status == 0 {
		rec.status = http.StatusOK
	}
	w.WriteHeader(rec.status)
	w.Write(rec.body.Bytes())
}

func clip(b []byte) string {
	const max = 6000
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/.well-known/caldav" {
		// 307 keeps the method: clients arrive here with PROPFIND.
		http.Redirect(w, r, root, http.StatusTemporaryRedirect)
		return
	}
	// Clients probe with OPTIONS before they have credentials.
	if r.Method == http.MethodOptions {
		w.Header().Set("DAV", "1, 3, calendar-access")
		w.Header().Set("Allow", "OPTIONS, GET, HEAD, PUT, DELETE, PROPFIND, PROPPATCH, REPORT")
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
	case "PROPPATCH":
		h.proppatch(w, r, id, t)
		return
	case "MKCOL", "MKCALENDAR", "COPY", "MOVE":
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
