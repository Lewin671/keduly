// Package testutil starts a complete server on a temporary database for tests.
package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Lewin671/keduly/internal/core"
	"github.com/Lewin671/keduly/internal/dav"
	"github.com/Lewin671/keduly/internal/httpapi"
	"github.com/Lewin671/keduly/internal/store"
)

// Clock is 10:00 on Tuesday 13 October 2026 in Asia/Shanghai.
var Clock = time.Date(2026, 10, 13, 2, 0, 0, 0, time.UTC)

type Server struct {
	T       testing.TB
	URL     string
	Service *core.Service
	now     time.Time
}

// Options adjusts the server under test.
type Options struct {
	AuthPerMinute int
	Registration  string
}

func New(t testing.TB, opts Options) *Server {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{T: t, now: Clock}
	svc := core.New(db)
	svc.Now = func() time.Time { return s.now }
	if opts.Registration != "" {
		svc.Registration = opts.Registration
	}
	if opts.AuthPerMinute == 0 {
		opts.AuthPerMinute = 100000
	}
	handler := httpapi.New(httpapi.Config{Service: svc, DAV: dav.New(svc),
		AuthPerMinute: opts.AuthPerMinute, APIPerMinute: 100000, DAVPerMinute: 100000})
	srv := httptest.NewServer(handler)
	t.Cleanup(func() {
		srv.Close()
		db.Close()
	})
	s.URL, s.Service = srv.URL, svc
	return s
}

// Now is the server's clock.
func (s *Server) Now() time.Time { return s.now }

// SetNow moves the server's clock.
func (s *Server) SetNow(t time.Time) { s.now = t }

// Client is one caller: a browser session (cookie) or a bearer token.
type Client struct {
	S      *Server
	Cookie *http.Cookie
	Token  string
	NoCSRF bool
}

// Do sends a request and returns the status and raw body.
func (c *Client) Do(method, path string, body any) (int, []byte, http.Header) {
	c.S.T.Helper()
	var payload io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		payload = strings.NewReader(b)
	default:
		data, err := json.Marshal(b)
		if err != nil {
			c.S.T.Fatal(err)
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.S.URL+"/api/v1"+path, payload)
	if err != nil {
		c.S.T.Fatal(err)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.Cookie != nil {
		req.AddCookie(c.Cookie)
	}
	if !c.NoCSRF && c.Token == "" {
		req.Header.Set("X-Keduly-Request", "1")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.S.T.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data, resp.Header
}

// Call sends a request, checks the status and decodes the body into out.
func (c *Client) Call(method, path string, body any, want int, out any) {
	c.S.T.Helper()
	status, data, _ := c.Do(method, path, body)
	if status != want {
		c.S.T.Fatalf("%s %s: status %d, want %d; body %s", method, path, status, want, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			c.S.T.Fatalf("%s %s: cannot decode %s: %v", method, path, data, err)
		}
	}
}

// ErrorCode sends a request and returns the status and error code.
func (c *Client) ErrorCode(method, path string, body any) (int, string) {
	c.S.T.Helper()
	status, data, _ := c.Do(method, path, body)
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(data, &e)
	return status, e.Error.Code
}

// Register creates an account in Asia/Shanghai and returns its session.
func (s *Server) Register(email string) *Client {
	s.T.Helper()
	c := &Client{S: s}
	status, data, header := c.Do("POST", "/auth/register", map[string]any{
		"email": email, "password": "correct horse", "name": "Tester", "timezone": "Asia/Shanghai"})
	if status != http.StatusCreated {
		s.T.Fatalf("register: status %d: %s", status, data)
	}
	c.Cookie = sessionCookie(header)
	if c.Cookie == nil {
		s.T.Fatal("register set no session cookie")
	}
	return c
}

func sessionCookie(h http.Header) *http.Cookie {
	resp := http.Response{Header: h}
	for _, c := range resp.Cookies() {
		if c.Name == "keduly_session" {
			return c
		}
	}
	return nil
}

// SessionCookie extracts the session cookie from response headers.
func SessionCookie(h http.Header) *http.Cookie { return sessionCookie(h) }

// NewToken creates a token through the API and returns a client that uses it.
func (c *Client) NewToken(name, kind, scope string, confirmDelete bool) *Client {
	c.S.T.Helper()
	var resp struct {
		Token struct {
			Token string `json:"token"`
		} `json:"token"`
	}
	c.Call("POST", "/tokens", map[string]any{"name": name, "kind": kind, "scope": scope, "confirm_delete": confirmDelete},
		http.StatusCreated, &resp)
	if !strings.HasPrefix(resp.Token.Token, "kdl_") {
		c.S.T.Fatalf("token %q does not start with kdl_", resp.Token.Token)
	}
	return &Client{S: c.S, Token: resp.Token.Token}
}
