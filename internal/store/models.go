package store

import (
	"context"
	"database/sql"
	"errors"
)

type User struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	Timezone     string
	WorkStart    string
	WorkEnd      string
	Revision     int64
	CreatedAt    string
}

const userCols = "id, email, name, password_hash, timezone, work_start, work_end, revision, created_at"

func scanUser(row *sql.Row) (*User, error) {
	u := new(User)
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Timezone, &u.WorkStart, &u.WorkEnd, &u.Revision, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func UserByID(ctx context.Context, q Q, id string) (*User, error) {
	return scanUser(q.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE id = ?", id))
}

func UserByEmail(ctx context.Context, q Q, email string) (*User, error) {
	return scanUser(q.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE email = ?", email))
}

func InsertUser(ctx context.Context, q Q, u *User) error {
	_, err := q.ExecContext(ctx, "INSERT INTO users ("+userCols+") VALUES (?,?,?,?,?,?,?,?,?)",
		u.ID, u.Email, u.Name, u.PasswordHash, u.Timezone, u.WorkStart, u.WorkEnd, u.Revision, u.CreatedAt)
	return err
}

func UpdateUser(ctx context.Context, q Q, u *User) error {
	_, err := q.ExecContext(ctx, "UPDATE users SET name = ?, password_hash = ?, timezone = ?, work_start = ?, work_end = ? WHERE id = ?",
		u.Name, u.PasswordHash, u.Timezone, u.WorkStart, u.WorkEnd, u.ID)
	return err
}

// BumpRevision increments and returns the user's revision.
func BumpRevision(ctx context.Context, q Q, userID string) (int64, error) {
	var rev int64
	err := q.QueryRowContext(ctx, "UPDATE users SET revision = revision + 1 WHERE id = ? RETURNING revision", userID).Scan(&rev)
	return rev, err
}

func Revision(ctx context.Context, q Q, userID string) (int64, error) {
	var rev int64
	err := q.QueryRowContext(ctx, "SELECT revision FROM users WHERE id = ?", userID).Scan(&rev)
	return rev, err
}

type Session struct {
	Hash      string
	UserID    string
	ExpiresAt string
	CreatedAt string
}

func InsertSession(ctx context.Context, q Q, s *Session) error {
	_, err := q.ExecContext(ctx, "INSERT INTO sessions (hash, user_id, expires_at, created_at) VALUES (?,?,?,?)",
		s.Hash, s.UserID, s.ExpiresAt, s.CreatedAt)
	return err
}

func SessionByHash(ctx context.Context, q Q, hash string) (*Session, error) {
	s := new(Session)
	err := q.QueryRowContext(ctx, "SELECT hash, user_id, expires_at, created_at FROM sessions WHERE hash = ?", hash).
		Scan(&s.Hash, &s.UserID, &s.ExpiresAt, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

func ExtendSession(ctx context.Context, q Q, hash, expiresAt string) error {
	_, err := q.ExecContext(ctx, "UPDATE sessions SET expires_at = ? WHERE hash = ?", expiresAt, hash)
	return err
}

func DeleteSession(ctx context.Context, q Q, hash string) error {
	_, err := q.ExecContext(ctx, "DELETE FROM sessions WHERE hash = ?", hash)
	return err
}

func DeleteOtherSessions(ctx context.Context, q Q, userID, keepHash string) error {
	_, err := q.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ? AND hash <> ?", userID, keepHash)
	return err
}

func DeleteExpiredSessions(ctx context.Context, q Q, now string) error {
	_, err := q.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < ?", now)
	return err
}

type Token struct {
	ID            string
	UserID        string
	Name          string
	Kind          string
	Scope         string
	ConfirmDelete bool
	Hash          string
	LastUsedAt    *string
	CreatedAt     string
}

var Tokens = Table[Token]{
	Name:  "tokens",
	Cols:  []string{"id", "user_id", "name", "kind", "scope", "confirm_delete", "hash", "last_used_at", "created_at"},
	Order: "rowid",
	fields: func(t *Token) []any {
		return []any{&t.ID, &t.UserID, &t.Name, &t.Kind, &t.Scope, &t.ConfirmDelete, &t.Hash, &t.LastUsedAt, &t.CreatedAt}
	},
}

func TokenByHash(ctx context.Context, q Q, hash string) (*Token, error) {
	rows, err := Tokens.Query(ctx, q, Tokens.SelectSQL()+" WHERE hash = ?", hash)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func TouchToken(ctx context.Context, q Q, id, now string) error {
	_, err := q.ExecContext(ctx, "UPDATE tokens SET last_used_at = ? WHERE id = ?", now, id)
	return err
}

type Area struct {
	ID       string `json:"id"`
	UserID   string `json:"-"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

var Areas = Table[Area]{
	Name:   "areas",
	Cols:   []string{"id", "user_id", "name", "position"},
	Order:  "position, rowid",
	fields: func(a *Area) []any { return []any{&a.ID, &a.UserID, &a.Name, &a.Position} },
}

type Project struct {
	ID        string  `json:"id"`
	UserID    string  `json:"-"`
	AreaID    *string `json:"area_id"`
	Name      string  `json:"name"`
	Color     string  `json:"color"`
	Notes     string  `json:"notes"`
	Position  int     `json:"position"`
	Archived  bool    `json:"archived"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

var Projects = Table[Project]{
	Name:  "projects",
	Cols:  []string{"id", "user_id", "area_id", "name", "color", "notes", "position", "archived", "created_at", "updated_at"},
	Order: "position, rowid",
	fields: func(p *Project) []any {
		return []any{&p.ID, &p.UserID, &p.AreaID, &p.Name, &p.Color, &p.Notes, &p.Position, &p.Archived, &p.CreatedAt, &p.UpdatedAt}
	},
}

type Heading struct {
	ID        string `json:"id"`
	UserID    string `json:"-"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Position  int    `json:"position"`
}

var Headings = Table[Heading]{
	Name:   "headings",
	Cols:   []string{"id", "user_id", "project_id", "name", "position"},
	Order:  "position, rowid",
	fields: func(h *Heading) []any { return []any{&h.ID, &h.UserID, &h.ProjectID, &h.Name, &h.Position} },
}

type Item struct {
	ID              string  `json:"id"`
	UserID          string  `json:"-"`
	ProjectID       *string `json:"project_id"`
	HeadingID       *string `json:"heading_id"`
	Title           string  `json:"title"`
	Notes           string  `json:"notes"`
	EstimateMinutes *int    `json:"estimate_minutes"`
	PlannedDate     *string `json:"planned_date"`
	Evening         bool    `json:"evening"`
	DueDate         *string `json:"due_date"`
	DueTime         *string `json:"due_time"`
	Important       bool    `json:"important"`
	Status          string  `json:"status"`
	CompletedAt     *string `json:"completed_at"`
	Position        int     `json:"position"`
	CreatedByKind   string  `json:"created_by_kind"`
	CreatedByName   string  `json:"created_by_name"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

var Items = Table[Item]{
	Name: "items",
	Cols: []string{"id", "user_id", "project_id", "heading_id", "title", "notes", "estimate_minutes", "planned_date",
		"evening", "due_date", "due_time", "important", "status", "completed_at", "position",
		"created_by_kind", "created_by_name", "created_at", "updated_at"},
	Order: "position, created_at, rowid",
	fields: func(i *Item) []any {
		return []any{&i.ID, &i.UserID, &i.ProjectID, &i.HeadingID, &i.Title, &i.Notes, &i.EstimateMinutes, &i.PlannedDate,
			&i.Evening, &i.DueDate, &i.DueTime, &i.Important, &i.Status, &i.CompletedAt, &i.Position,
			&i.CreatedByKind, &i.CreatedByName, &i.CreatedAt, &i.UpdatedAt}
	},
}

type Event struct {
	ID            string  `json:"id"`
	UserID        string  `json:"-"`
	ProjectID     *string `json:"project_id"`
	ItemID        *string `json:"item_id"`
	Title         string  `json:"title"`
	Notes         string  `json:"notes"`
	Location      string  `json:"location"`
	AllDay        bool    `json:"all_day"`
	StartAt       *string `json:"start_at"`
	EndAt         *string `json:"end_at"`
	StartDate     *string `json:"start_date"`
	EndDate       *string `json:"end_date"`
	RRule         *string `json:"rrule"`
	UID           string  `json:"uid"`
	DavName       string  `json:"dav_name"`
	ICal          string  `json:"ical"`
	ETag          string  `json:"etag"`
	CreatedByKind string  `json:"created_by_kind"`
	CreatedByName string  `json:"created_by_name"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

var Events = Table[Event]{
	Name: "events",
	Cols: []string{"id", "user_id", "project_id", "item_id", "title", "notes", "location", "all_day",
		"start_at", "end_at", "start_date", "end_date", "rrule", "uid", "dav_name", "ical", "etag",
		"created_by_kind", "created_by_name", "created_at", "updated_at"},
	Order: "start_at, start_date, rowid",
	fields: func(e *Event) []any {
		return []any{&e.ID, &e.UserID, &e.ProjectID, &e.ItemID, &e.Title, &e.Notes, &e.Location, &e.AllDay,
			&e.StartAt, &e.EndAt, &e.StartDate, &e.EndDate, &e.RRule, &e.UID, &e.DavName, &e.ICal, &e.ETag,
			&e.CreatedByKind, &e.CreatedByName, &e.CreatedAt, &e.UpdatedAt}
	},
}

type Suggestion struct {
	ID        string
	UserID    string
	Status    string
	Kind      string
	ActorKind string
	ActorName string
	Reason    string
	Title     string
	ItemID    *string
	EventID   *string
	StartAt   *string
	EndAt     *string
	Payload   *string
	CreatedAt string
	DecidedAt *string
}

var Suggestions = Table[Suggestion]{
	Name: "suggestions",
	Cols: []string{"id", "user_id", "status", "kind", "actor_kind", "actor_name", "reason", "title",
		"item_id", "event_id", "start_at", "end_at", "payload", "created_at", "decided_at"},
	Order: "created_at, rowid",
	fields: func(s *Suggestion) []any {
		return []any{&s.ID, &s.UserID, &s.Status, &s.Kind, &s.ActorKind, &s.ActorName, &s.Reason, &s.Title,
			&s.ItemID, &s.EventID, &s.StartAt, &s.EndAt, &s.Payload, &s.CreatedAt, &s.DecidedAt}
	},
}

type Activity struct {
	ID        string
	UserID    string
	ActorKind string
	ActorName string
	Action    string
	Summary   string
	Reason    *string
	Undoable  bool
	Undone    bool
	Changes   string
	CreatedAt string
}

var Activities = Table[Activity]{
	Name: "activities",
	Cols: []string{"id", "user_id", "actor_kind", "actor_name", "action", "summary", "reason",
		"undoable", "undone", "changes", "created_at"},
	Order: "seq DESC",
	fields: func(a *Activity) []any {
		return []any{&a.ID, &a.UserID, &a.ActorKind, &a.ActorName, &a.Action, &a.Summary, &a.Reason,
			&a.Undoable, &a.Undone, &a.Changes, &a.CreatedAt}
	},
}

func SetActivityUndone(ctx context.Context, q Q, userID, id string, undone bool) error {
	_, err := q.ExecContext(ctx, "UPDATE activities SET undone = ? WHERE user_id = ? AND id = ?", undone, userID, id)
	return err
}

// BumpCTag marks a calendar ("inbox" or a project ID) as changed.
func BumpCTag(ctx context.Context, q Q, userID, calendar string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO ctags (user_id, calendar, ctag) VALUES (?, ?, 1)
		ON CONFLICT(user_id, calendar) DO UPDATE SET ctag = ctag + 1`, userID, calendar)
	return err
}

func CTag(ctx context.Context, q Q, userID, calendar string) (int64, error) {
	var n int64
	err := q.QueryRowContext(ctx, "SELECT ctag FROM ctags WHERE user_id = ? AND calendar = ?", userID, calendar).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return n, err
}
