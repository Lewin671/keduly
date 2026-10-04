// Package httpapi serves the JSON API described in docs/api.md and mounts the
// CalDAV endpoint and the web app next to it.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Lewin671/keduly/internal/core"
)

const (
	cookieName   = "keduly_session"
	maxBodyBytes = 1 << 20
)

// Config wires the server together. Zero limits take the defaults.
type Config struct {
	Service *core.Service
	BaseURL string       // only decides whether cookies are Secure
	DAV     http.Handler // receives /dav/ and /.well-known/caldav
	Static  http.Handler // receives everything else
	Logger  *slog.Logger

	AuthPerMinute int
	APIPerMinute  int
	DAVPerMinute  int
}

type Server struct {
	cfg   Config
	svc   *core.Service
	mux   *http.ServeMux
	auth  *limiter
	api   *limiter
	dav   *limiter
	log   *slog.Logger
	https bool
}

func orDefault(n, def int) int {
	if n <= 0 {
		return def
	}
	return n
}

// New returns the root handler of the server.
func New(cfg Config) http.Handler {
	s := &Server{
		cfg: cfg, svc: cfg.Service, mux: http.NewServeMux(), log: cfg.Logger,
		auth:  newLimiter(orDefault(cfg.AuthPerMinute, 10)),
		api:   newLimiter(orDefault(cfg.APIPerMinute, 300)),
		dav:   newLimiter(orDefault(cfg.DAVPerMinute, 600)),
		https: strings.HasPrefix(strings.ToLower(cfg.BaseURL), "https://"),
	}
	if s.log == nil {
		s.log = slog.New(slog.DiscardHandler)
	}
	s.routes()
	return s.logged(http.HandlerFunc(s.serve))
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case path == "/healthz":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok"))
	case strings.HasPrefix(path, "/api/"):
		s.serveAPI(w, r)
	case s.cfg.DAV != nil && (path == "/dav" || strings.HasPrefix(path, "/dav/") || path == "/.well-known/caldav"):
		if !s.dav.allow(clientIP(r)) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		s.cfg.DAV.ServeHTTP(w, r)
	case s.cfg.Static != nil:
		s.cfg.Static.ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) serveAPI(w http.ResponseWriter, r *http.Request) {
	bucket := s.api
	if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
		bucket = s.auth
	}
	if !bucket.allow(clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		writeError(w, &core.Error{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "too many requests; slow down"})
		return
	}
	// The mux answers unknown paths and methods in plain text; the contract wants JSON.
	if _, pattern := s.mux.Handler(r); pattern == "" {
		writeError(w, core.NotFound("endpoint"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	s.mux.ServeHTTP(w, r)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// logged writes one structured line per request. It logs the path only: no
// query string, headers, cookies or bodies, so no secret can leak into the log.
func (s *Server) logged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status == 0 {
			sw.status = http.StatusOK
		}
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.status,
			"duration_ms", float64(time.Since(start).Microseconds())/1000)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, e *core.Error) {
	writeJSON(w, e.Status, map[string]any{"error": map[string]string{"code": e.Code, "message": e.Message}})
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *core.Error
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &apiErr):
		writeError(w, apiErr)
	case errors.As(err, &tooLarge):
		writeError(w, core.Invalid("the request body is larger than 1 MiB"))
	default:
		s.log.Error("internal error", "method", r.Method, "path", r.URL.Path, "error", err.Error())
		writeError(w, &core.Error{Status: http.StatusInternalServerError, Code: "internal", Message: "internal error"})
	}
}

func (s *Server) secure(r *http.Request) bool {
	return s.https || r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (s *Server) setCookie(w http.ResponseWriter, r *http.Request, secret string) {
	c := &http.Cookie{Name: cookieName, Value: secret, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: s.secure(r), MaxAge: int(core.SessionTTL.Seconds())}
	if secret == "" {
		c.MaxAge = -1
	}
	http.SetCookie(w, c)
}

func bearer(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:]), true
	}
	return "", false
}

// access says who may call a route.
type access int

const (
	public      access = iota // no authentication
	member                    // a session, or a token whose scope allows the method
	sessionOnly               // a session cookie; tokens are refused
)

// call is one authenticated API request.
type call struct {
	w      http.ResponseWriter
	r      *http.Request
	id     *core.Identity
	fields core.Fields
	dryRun bool
	reason string
}

type handler func(c *call) error

// handle registers a route under /api/v1.
func (s *Server) handle(pattern string, level access, fn handler) {
	method, path, _ := strings.Cut(pattern, " ")
	s.mux.HandleFunc(method+" /api/v1"+path, func(w http.ResponseWriter, r *http.Request) {
		c := &call{w: w, r: r}
		if err := s.prepare(c, level); err != nil {
			s.fail(w, r, err)
			return
		}
		if err := fn(c); err != nil {
			s.fail(w, r, err)
		}
	})
}

func (s *Server) prepare(c *call, level access) error {
	r := c.r
	token, hasToken := bearer(r)
	// A cross-site form cannot set a custom header, so requiring one on every
	// cookie-authenticated write defeats CSRF.
	if r.Method != http.MethodGet && !hasToken && r.Header.Get("X-Keduly-Request") != "1" {
		return &core.Error{Status: http.StatusForbidden, Code: "csrf", Message: "the X-Keduly-Request header is required"}
	}
	if level != public {
		if err := s.identify(c, token, hasToken); err != nil {
			return err
		}
		if t := c.id.Token; t != nil {
			if level == sessionOnly {
				return core.Forbidden("this endpoint requires a signed-in session, not a token")
			}
			if t.Scope != "write" && r.Method != http.MethodGet {
				return core.Forbidden("the token's scope is read-only")
			}
		}
	}
	if r.Method == http.MethodGet {
		return nil
	}
	var err error
	if c.fields, err = core.DecodeFields(r.Body); err != nil {
		return err
	}
	if level == public {
		return nil
	}
	switch r.URL.Query().Get("dry_run") {
	case "1", "true":
		c.dryRun = true
	}
	if level == member {
		if c.reason, err = c.fields.Take("reason"); err != nil {
			return err
		}
		if c.reason == "" {
			c.reason = r.Header.Get("X-Keduly-Reason")
		}
	}
	return nil
}

func (s *Server) identify(c *call, token string, hasToken bool) error {
	ctx := c.r.Context()
	if hasToken {
		id, err := s.svc.Bearer(ctx, token)
		c.id = id
		return err
	}
	cookie, err := c.r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		return core.ErrUnauthenticated
	}
	id, renewed, err := s.svc.Session(ctx, cookie.Value)
	if err != nil {
		return err
	}
	if renewed {
		s.setCookie(c.w, c.r, cookie.Value)
	}
	c.id = id
	return nil
}

func (c *call) query(name string) string { return c.r.URL.Query().Get(name) }

func (c *call) pathID() string { return c.r.PathValue("id") }

func (c *call) ok(body any) error {
	writeJSON(c.w, http.StatusOK, body)
	return nil
}

type obj = map[string]any

// write runs fn in a write transaction and sends what it returns. A nil body
// means 204, except on a dry run, which always reports itself.
func (s *Server) write(c *call, status int, fn func(op *core.Op) (obj, error)) error {
	var body obj
	err := s.svc.Write(c.r.Context(), c.id, core.WriteOptions{DryRun: c.dryRun, Reason: c.reason}, func(op *core.Op) error {
		var err error
		body, err = fn(op)
		return err
	})
	if err != nil {
		return err
	}
	if c.dryRun {
		if body == nil {
			body, status = obj{}, http.StatusOK
		}
		body["dry_run"] = true
	}
	if body == nil {
		c.w.WriteHeader(http.StatusNoContent)
		return nil
	}
	if s, ok := body["_status"].(int); ok {
		status = s
		delete(body, "_status")
	}
	writeJSON(c.w, status, body)
	return nil
}

func (s *Server) read(c *call) *core.Op { return s.svc.Read(c.r.Context(), c.id) }
