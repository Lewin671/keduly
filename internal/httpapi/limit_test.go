package httpapi

import (
	"net/http"
	"testing"
	"time"
)

func TestLimiterRefills(t *testing.T) {
	now := time.Unix(0, 0)
	l := newLimiter(2)
	l.now = func() time.Time { return now }
	if !l.allow("a") || !l.allow("a") || l.allow("a") {
		t.Fatal("the bucket should allow exactly two requests")
	}
	if !l.allow("b") {
		t.Fatal("another client has its own bucket")
	}
	now = now.Add(30 * time.Second)
	if !l.allow("a") || l.allow("a") {
		t.Fatal("half a minute refills one request")
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct{ remote, forwarded, want string }{
		{"198.51.100.7:4000", "", "198.51.100.7"},
		// A public peer cannot choose its own address.
		{"198.51.100.7:4000", "203.0.113.9", "198.51.100.7"},
		{"127.0.0.1:4000", "203.0.113.9", "203.0.113.9"},
		{"10.0.0.2:4000", "203.0.113.9", "203.0.113.9"},
		// Only the entry the proxy appended counts; earlier ones are client-supplied.
		{"127.0.0.1:4000", "1.2.3.4, 203.0.113.9", "203.0.113.9"},
		{"127.0.0.1:4000", "not an address", "127.0.0.1"},
		{"[::1]:4000", "", "::1"},
	}
	for _, c := range cases {
		r := &http.Request{RemoteAddr: c.remote, Header: http.Header{}}
		if c.forwarded != "" {
			r.Header.Set("X-Forwarded-For", c.forwarded)
		}
		if got := clientIP(r); got != c.want {
			t.Errorf("clientIP(%s, %q) = %s, want %s", c.remote, c.forwarded, got, c.want)
		}
	}
}
