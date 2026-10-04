// Package core implements the domain logic behind the JSON API and CalDAV.
package core

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	_ "time/tzdata" // the release image has no zoneinfo

	"github.com/Lewin671/keduly/internal/api"
	"github.com/Lewin671/keduly/internal/store"
)

// Limits caps how many rows one user may own.
type Limits struct {
	Items    int
	Events   int
	Projects int
	Tokens   int
}

var DefaultLimits = Limits{Items: 20000, Events: 20000, Projects: 200, Tokens: 50}

const (
	maxTitle = 500
	maxNotes = 20000
	maxName  = 200
)

type Service struct {
	DB           *store.DB
	Registration string // "open" or "closed"
	Version      string
	Limits       Limits
	Now          func() time.Time

	hashSlots chan struct{}
}

func New(db *store.DB) *Service {
	return &Service{
		DB:           db,
		Registration: "open",
		Version:      "dev",
		Limits:       DefaultLimits,
		Now:          time.Now,
		// Argon2 is memory-hard; bound how many run at once on a small server.
		hashSlots: make(chan struct{}, 4),
	}
}

// Error is an API error with its HTTP status and contract code.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func Invalid(format string, a ...any) *Error {
	return &Error{http.StatusBadRequest, "invalid_request", fmt.Sprintf(format, a...)}
}

func NotFound(what string) *Error {
	return &Error{http.StatusNotFound, "not_found", what + " not found"}
}

func Conflict(format string, a ...any) *Error {
	return &Error{http.StatusConflict, "conflict", fmt.Sprintf(format, a...)}
}

func Forbidden(message string) *Error {
	return &Error{http.StatusForbidden, "forbidden", message}
}

var ErrUnauthenticated = &Error{http.StatusUnauthorized, "unauthenticated", "authentication required"}

// committed wraps an error that must be returned although the transaction commits.
type committed struct{ err error }

func (c *committed) Error() string { return c.err.Error() }

// Identity is an authenticated caller.
type Identity struct {
	User        *store.User
	Actor       api.Actor
	Token       *store.Token // nil for a web session
	SessionHash string
}

func (id *Identity) IsSession() bool { return id.Token == nil }

// Op is one unit of work for one user: a read against the database or a write
// inside a transaction. Writes collect row-level changes for the activity log.
type Op struct {
	svc     *Service
	ctx     context.Context
	q       store.Q
	User    *store.User
	Loc     *time.Location
	Now     time.Time
	ID      *Identity
	Reason  string
	changes []change
	deco    *decorations
}

func loadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

func (s *Service) newOp(ctx context.Context, q store.Q, id *Identity) *Op {
	return &Op{svc: s, ctx: ctx, q: q, User: id.User, Loc: loadLocation(id.User.Timezone),
		Now: s.Now().UTC().Truncate(time.Second), ID: id}
}

// Read returns an Op that queries the database directly.
func (s *Service) Read(ctx context.Context, id *Identity) *Op {
	return s.newOp(ctx, s.DB, id)
}

type WriteOptions struct {
	DryRun bool
	Reason string
}

// Write runs fn in a transaction, bumping the user's revision on commit.
// A dry run executes everything and rolls back.
func (s *Service) Write(ctx context.Context, id *Identity, opts WriteOptions, fn func(*Op) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	op := s.newOp(ctx, tx, id)
	op.Reason = strings.TrimSpace(opts.Reason)
	if len(op.Reason) > 2000 {
		return Invalid("reason must be at most 2000 characters")
	}
	err = fn(op)
	var keep *committed
	if err != nil && !errors.As(err, &keep) {
		return mapConstraint(err)
	}
	if opts.DryRun {
		return unwrapCommitted(err)
	}
	if _, berr := store.BumpRevision(ctx, tx, id.User.ID); berr != nil {
		return berr
	}
	if cerr := tx.Commit(); cerr != nil {
		return cerr
	}
	return unwrapCommitted(err)
}

func unwrapCommitted(err error) error {
	var keep *committed
	if errors.As(err, &keep) {
		return keep.err
	}
	return err
}

// mapConstraint turns a foreign key or uniqueness failure into a conflict.
func mapConstraint(err error) error {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return err
	}
	if msg := err.Error(); strings.Contains(msg, "constraint failed") {
		return Conflict("the change conflicts with the current data")
	}
	return err
}

var idEncoding = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// NewID returns 16 lowercase base32 characters (80 random bits).
func NewID() string {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return idEncoding.EncodeToString(b)
}

func (op *Op) now() string { return store.FormatTime(op.Now) }

// today is the current date in the user's time zone.
func (op *Op) today() string { return op.Now.In(op.Loc).Format(dateLayout) }

func (op *Op) checkLimit(n, limit int, what string) error {
	if n >= limit {
		return Invalid("limit reached: an account can hold at most %d %s", limit, what)
	}
	return nil
}

func noRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }
