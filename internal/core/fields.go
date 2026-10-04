package core

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Lewin671/keduly/internal/store"
)

const dateLayout = "2006-01-02"

// Fields is a decoded JSON object whose members are read one by one, so that
// PATCH can tell an absent member from a null one and unknown members are rejected.
type Fields map[string]json.RawMessage

// DecodeFields parses a JSON object. An empty body is an empty object.
func DecodeFields(r io.Reader) (Fields, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, Invalid("could not read the request body: it may be larger than 1 MiB")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return Fields{}, nil
	}
	var f Fields
	if err := json.Unmarshal(data, &f); err != nil || f == nil {
		return nil, Invalid("the request body must be a JSON object")
	}
	return f, nil
}

// Take removes and returns a string member, used for envelope fields such as "reason".
func (f Fields) Take(name string) (string, error) {
	raw, ok := f[name]
	if !ok {
		return "", nil
	}
	delete(f, name)
	if isNull(raw) {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", Invalid("%s must be a string", name)
	}
	return s, nil
}

func isNull(raw json.RawMessage) bool { return string(bytes.TrimSpace(raw)) == "null" }

// reader consumes Fields and remembers the first error.
type reader struct {
	f    Fields
	used map[string]bool
	err  error
}

func newReader(f Fields) *reader { return &reader{f: f, used: map[string]bool{}} }

func (r *reader) fail(format string, a ...any) {
	if r.err == nil {
		r.err = Invalid(format, a...)
	}
}

func (r *reader) raw(name string) (json.RawMessage, bool) {
	r.used[name] = true
	raw, ok := r.f[name]
	return raw, ok
}

func (r *reader) has(name string) bool {
	_, ok := r.f[name]
	return ok
}

// done reports the first error, or an unknown member.
func (r *reader) done() error {
	if r.err != nil {
		return r.err
	}
	for name := range r.f {
		if !r.used[name] {
			return Invalid("unknown field %q", name)
		}
	}
	return nil
}

func (r *reader) str(name string, dst *string, max int) {
	raw, ok := r.raw(name)
	if !ok {
		return
	}
	var s string
	if isNull(raw) || json.Unmarshal(raw, &s) != nil {
		r.fail("%s must be a string", name)
		return
	}
	if utf8.RuneCountInString(s) > max {
		r.fail("%s must be at most %d characters", name, max)
		return
	}
	*dst = s
}

// name reads a required-when-present, trimmed, non-empty string.
func (r *reader) name(name string, dst *string, max int) {
	if !r.has(name) {
		return
	}
	var s string
	r.str(name, &s, max)
	s = strings.TrimSpace(s)
	if r.err == nil && s == "" {
		r.fail("%s must not be empty", name)
		return
	}
	*dst = s
}

func (r *reader) boolean(name string, dst *bool) {
	raw, ok := r.raw(name)
	if !ok {
		return
	}
	var b bool
	if isNull(raw) || json.Unmarshal(raw, &b) != nil {
		r.fail("%s must be true or false", name)
		return
	}
	*dst = b
}

func (r *reader) integer(name string, dst *int) {
	raw, ok := r.raw(name)
	if !ok {
		return
	}
	var n int
	if isNull(raw) || json.Unmarshal(raw, &n) != nil {
		r.fail("%s must be an integer", name)
		return
	}
	*dst = n
}

func (r *reader) nullInt(name string, dst **int, min, max int) {
	raw, ok := r.raw(name)
	if !ok {
		return
	}
	if isNull(raw) {
		*dst = nil
		return
	}
	var n int
	if json.Unmarshal(raw, &n) != nil || n < min || n > max {
		r.fail("%s must be an integer between %d and %d, or null", name, min, max)
		return
	}
	*dst = &n
}

// nullStr reads a string-or-null member, validating non-null values with check.
func (r *reader) nullStr(name string, dst **string, check func(string) bool, what string) {
	raw, ok := r.raw(name)
	if !ok {
		return
	}
	if isNull(raw) {
		*dst = nil
		return
	}
	var s string
	if json.Unmarshal(raw, &s) != nil || (check != nil && !check(s)) {
		r.fail("%s must be %s or null", name, what)
		return
	}
	*dst = &s
}

func (r *reader) date(name string, dst **string) {
	r.nullStr(name, dst, validDate, "a date (YYYY-MM-DD)")
}

// instant reads an RFC 3339 timestamp (or null) into the stored UTC form.
func (r *reader) instant(name string, dst **string) {
	raw, ok := r.raw(name)
	if !ok {
		return
	}
	if isNull(raw) {
		*dst = nil
		return
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		r.fail("%s must be an RFC 3339 timestamp", name)
		return
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		r.fail("%s must be an RFC 3339 timestamp", name)
		return
	}
	v := store.FormatTime(t)
	*dst = &v
}

func validDate(s string) bool {
	t, err := time.Parse(dateLayout, s)
	return err == nil && t.Format(dateLayout) == s
}

var clockRE = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

func validClock(s string) bool { return clockRE.MatchString(s) }

func anyID(s string) bool { return s != "" && len(s) <= 64 }
