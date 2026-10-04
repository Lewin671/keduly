package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// limiter is a token bucket per key: perMinute requests refill every minute,
// with a burst of the same size.
type limiter struct {
	mu        sync.Mutex
	perMinute float64
	buckets   map[string]*bucket
	now       func() time.Time
}

type bucket struct {
	tokens float64
	seen   time.Time
}

func newLimiter(perMinute int) *limiter {
	return &limiter{perMinute: float64(perMinute), buckets: map[string]*bucket{}, now: time.Now}
}

func (l *limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b := l.buckets[key]
	if b == nil {
		if len(l.buckets) > 50000 {
			l.prune(now)
		}
		b = &bucket{tokens: l.perMinute, seen: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.perMinute, b.tokens+now.Sub(b.seen).Minutes()*l.perMinute)
	b.seen = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *limiter) prune(now time.Time) {
	for key, b := range l.buckets {
		if now.Sub(b.seen) > time.Minute {
			delete(l.buckets, key)
		}
	}
}

// clientIP is the peer address, or the address a trusted reverse proxy saw.
// X-Forwarded-For is honoured only when the peer is loopback or private, and
// only its last entry, which is the one the proxy itself appended.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil || !(peer.IsLoopback() || peer.IsPrivate()) {
		return host
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	last := strings.TrimSpace(parts[len(parts)-1])
	if ip := net.ParseIP(last); ip != nil {
		return ip.String()
	}
	return host
}
